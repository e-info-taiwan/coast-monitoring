package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"coast-monitoring/internal/service"
)

type CWAMarineRepository struct {
	db DBTX
}

func NewCWAMarineRepository(db DBTX) CWAMarineRepository {
	return CWAMarineRepository{db: db}
}

func (r CWAMarineRepository) UpsertStations(ctx context.Context, stations []service.CWAMarineStation) error {
	for _, s := range stations {
		_, err := r.db.Exec(ctx, `
			INSERT INTO cwa_marine_station (
				id, name_zh, name_en, station_type, area_name, county_name, town_name,
				latitude, longitude, address, is_active, updated_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, now())
			ON CONFLICT (id) DO UPDATE SET
				name_zh = EXCLUDED.name_zh,
				name_en = COALESCE(NULLIF(EXCLUDED.name_en, ''), cwa_marine_station.name_en),
				station_type = EXCLUDED.station_type,
				area_name = COALESCE(NULLIF(EXCLUDED.area_name, ''), cwa_marine_station.area_name),
				county_name = COALESCE(NULLIF(EXCLUDED.county_name, ''), cwa_marine_station.county_name),
				town_name = COALESCE(NULLIF(EXCLUDED.town_name, ''), cwa_marine_station.town_name),
				latitude = COALESCE(EXCLUDED.latitude, cwa_marine_station.latitude),
				longitude = COALESCE(EXCLUDED.longitude, cwa_marine_station.longitude),
				address = COALESCE(NULLIF(EXCLUDED.address, ''), cwa_marine_station.address),
				is_active = true,
				updated_at = now()
		`, s.ID, s.NameZH, s.NameEN, s.StationType, s.AreaName, s.CountyName, s.TownName, s.Latitude, s.Longitude, s.Address, s.IsActive)
		if err != nil {
			return translateError(err)
		}
	}
	return nil
}

func (r CWAMarineRepository) ListStations(ctx context.Context) ([]service.CWAMarineStation, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, name_zh, COALESCE(name_en, ''), station_type, COALESCE(area_name, ''),
		       COALESCE(county_name, ''), COALESCE(town_name, ''), latitude, longitude,
		       COALESCE(address, ''), is_active
		FROM cwa_marine_station
		WHERE is_active = true
		ORDER BY area_name, county_name, name_zh
	`)
	if err != nil {
		return nil, translateError(err)
	}
	defer rows.Close()

	var stations []service.CWAMarineStation
	for rows.Next() {
		var s service.CWAMarineStation
		var lat, lng sql.NullFloat64
		if err := rows.Scan(
			&s.ID, &s.NameZH, &s.NameEN, &s.StationType, &s.AreaName,
			&s.CountyName, &s.TownName, &lat, &lng, &s.Address, &s.IsActive,
		); err != nil {
			return nil, translateError(err)
		}
		if lat.Valid {
			s.Latitude = &lat.Float64
		}
		if lng.Valid {
			s.Longitude = &lng.Float64
		}
		stations = append(stations, s)
	}
	return stations, translateError(rows.Err())
}

func (r CWAMarineRepository) GetStation(ctx context.Context, id string) (*service.CWAMarineStation, error) {
	var s service.CWAMarineStation
	var lat, lng sql.NullFloat64
	err := r.db.QueryRow(ctx, `
		SELECT id, name_zh, COALESCE(name_en, ''), station_type, COALESCE(area_name, ''),
		       COALESCE(county_name, ''), COALESCE(town_name, ''), latitude, longitude,
		       COALESCE(address, ''), is_active
		FROM cwa_marine_station
		WHERE id = $1
	`, id).Scan(
		&s.ID, &s.NameZH, &s.NameEN, &s.StationType, &s.AreaName,
		&s.CountyName, &s.TownName, &lat, &lng, &s.Address, &s.IsActive,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrNotFound
		}
		return nil, translateError(err)
	}
	if lat.Valid {
		s.Latitude = &lat.Float64
	}
	if lng.Valid {
		s.Longitude = &lng.Float64
	}
	return &s, nil
}

func (r CWAMarineRepository) UpsertSeaTemperatures(ctx context.Context, records []service.CWAObservationRecord) (int, error) {
	saved := 0
	for _, rec := range records {
		if rec.StationID == "" || rec.ObservedAt.IsZero() {
			continue
		}
		dateStr := rec.ObservedAt.Format("2006-01-02")
		tag, err := r.db.Exec(ctx, `
			INSERT INTO cwa_sea_temperature (station_id, observed_at, observed_date, sea_temp_c, air_temp_c)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (station_id, observed_at) DO UPDATE SET
				sea_temp_c = COALESCE(EXCLUDED.sea_temp_c, cwa_sea_temperature.sea_temp_c),
				air_temp_c = COALESCE(EXCLUDED.air_temp_c, cwa_sea_temperature.air_temp_c)
		`, rec.StationID, rec.ObservedAt, dateStr, rec.SeaTempC, rec.AirTempC)
		if err != nil {
			return saved, translateError(err)
		}
		if tag.RowsAffected() > 0 {
			saved++
		}
	}
	return saved, nil
}

func (r CWAMarineRepository) GetSeaTemperature(ctx context.Context, stationID string, dateStr string, timeStr string) (*service.CWATempResult, error) {
	var stName string
	err := r.db.QueryRow(ctx, `SELECT name_zh FROM cwa_marine_station WHERE id = $1`, stationID).Scan(&stName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			stName = stationID
		} else {
			return nil, translateError(err)
		}
	}

	result := &service.CWATempResult{
		StationID:   stationID,
		StationName: stName,
		SurveyDate:  dateStr,
		EventTime:   timeStr,
	}

	// Try to find closest observation
	var obsAt time.Time
	var seaTemp, airTemp sql.NullFloat64

	if timeStr != "" && timeStr != "na" {
		targetTimeStr := dateStr + " " + timeStr + ":00+08"
		err = r.db.QueryRow(ctx, `
			SELECT observed_at, sea_temp_c, air_temp_c
			FROM cwa_sea_temperature
			WHERE station_id = $1 AND observed_date = $2::date AND sea_temp_c IS NOT NULL
			ORDER BY ABS(EXTRACT(EPOCH FROM (observed_at - $3::timestamptz))) ASC
			LIMIT 1
		`, stationID, dateStr, targetTimeStr).Scan(&obsAt, &seaTemp, &airTemp)
	} else {
		// No specific time given, take latest reading of that date
		err = r.db.QueryRow(ctx, `
			SELECT observed_at, sea_temp_c, air_temp_c
			FROM cwa_sea_temperature
			WHERE station_id = $1 AND observed_date = $2::date AND sea_temp_c IS NOT NULL
			ORDER BY observed_at DESC
			LIMIT 1
		`, stationID, dateStr).Scan(&obsAt, &seaTemp, &airTemp)
	}

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return result, nil
		}
		return nil, translateError(err)
	}

	result.ObservedAt = &obsAt
	if seaTemp.Valid {
		result.SeaTempC = &seaTemp.Float64
	}
	if airTemp.Valid {
		result.AirTempC = &airTemp.Float64
	}
	return result, nil
}

func (r CWAMarineRepository) AutoLinkSites(ctx context.Context) (int, error) {
	tag, err := r.db.Exec(ctx, `
		UPDATE site s
		SET cwa_station_id = (
			SELECT st.id
			FROM cwa_marine_station st
			WHERE st.latitude IS NOT NULL AND st.longitude IS NOT NULL AND st.is_active = true
			ORDER BY (
				pow(s.latitude - st.latitude, 2) +
				pow((s.longitude - st.longitude) * cos(radians(COALESCE(s.latitude, 23.5))), 2)
			) ASC
			LIMIT 1
		)
		WHERE s.cwa_station_id IS NULL AND s.latitude IS NOT NULL AND s.longitude IS NOT NULL
	`)
	if err != nil {
		return 0, translateError(err)
	}
	return int(tag.RowsAffected()), nil
}
