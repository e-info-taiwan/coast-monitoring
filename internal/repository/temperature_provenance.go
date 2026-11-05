package repository

import (
	"coast-monitoring/internal/service"
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"strings"
	"time"
)

type eventTemperature struct {
	Value      *float64
	Source     *string
	ObservedAt *time.Time
	FetchedAt  *time.Time
}

func resolveEventTemperature(ctx context.Context, db DBTX, station *string, date, eventTime string, value *float64, source string) (eventTemperature, error) {
	result := eventTemperature{}
	if source != "" && source != "automatic" && source != "manual" {
		return result, fmt.Errorf("%w: 水溫來源無效", service.ErrValidation)
	}
	now := time.Now()
	if value != nil && source != "automatic" {
		s := "manual"
		return eventTemperature{Value: value, Source: &s, FetchedAt: &now}, nil
	}
	if station == nil || *station == "" {
		return result, nil
	}
	eventTime = strings.ReplaceAll(eventTime, "-", ":")
	target := date + "T23:59:59+08:00"
	if eventTime != "" && eventTime != "na" {
		target = date + "T" + eventTime + ":00+08:00"
	}
	var v float64
	var observed time.Time
	err := db.QueryRow(ctx, `SELECT sea_temp_c,observed_at FROM cwa_sea_temperature WHERE station_id=$1 AND observed_date=$2::date AND sea_temp_c IS NOT NULL ORDER BY abs(extract(epoch FROM observed_at-$3::timestamptz)),observed_at DESC LIMIT 1`, *station, date, target).Scan(&v, &observed)
	if err == pgx.ErrNoRows {
		return result, nil
	}
	if err != nil {
		return result, err
	}
	s := "automatic"
	return eventTemperature{Value: &v, Source: &s, ObservedAt: &observed, FetchedAt: &now}, nil
}
func saveEventTemperature(ctx context.Context, db DBTX, eventID int, v eventTemperature) error {
	_, err := db.Exec(ctx, `UPDATE event SET sea_temp_c=$2,sea_temp_source=$3,sea_temp_observed_at=$4,sea_temp_fetched_at=$5 WHERE id=$1 AND sea_temp_source IS DISTINCT FROM 'manual'`, eventID, v.Value, v.Source, v.ObservedAt, v.FetchedAt)
	return err
}
