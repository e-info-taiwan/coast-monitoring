package repository

import (
	"context"

	"coast-monitoring/internal/service"

	"github.com/google/uuid"
)

type ReefCheckSurveyRepository struct {
	db DBTX
}

func NewReefCheckSurveyRepository(db DBTX) ReefCheckSurveyRepository {
	return ReefCheckSurveyRepository{db: db}
}

func (r ReefCheckSurveyRepository) ListReefCheckSurveys(ctx context.Context) ([]service.ReefCheckSurvey, error) {
	return r.listReefCheckSurveys(ctx, "")
}

func (r ReefCheckSurveyRepository) ListReefCheckSurveysByCreator(ctx context.Context, creatorID uuid.UUID) ([]service.ReefCheckSurvey, error) {
	return r.listReefCheckSurveys(ctx, "WHERE created_by = $1", creatorID)
}

func (r ReefCheckSurveyRepository) CreateReefCheckSurvey(ctx context.Context, record service.ReefCheckSurveyRecord) (service.ReefCheckSurvey, error) {
	siteID := record.SiteID
	if siteID == uuid.Nil {
		locationID, err := r.createSurveyLocation(ctx, record)
		if err != nil {
			return service.ReefCheckSurvey{}, err
		}
		siteID, err = r.createSurveySite(ctx, locationID, record)
		if err != nil {
			return service.ReefCheckSurvey{}, err
		}
	}

	survey, err := scanReefCheckSurvey(r.db.QueryRow(ctx, `
		INSERT INTO reef_check_surveys (
			survey_date, site_id, depth_m, country_island, team_leader, survey_time, visibility,
			temperature, general_comments, substrate_comments, rkc_reason, rkc_bleaching_percent,
			fish_length_mode, created_by, updated_by
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $14)
		RETURNING id, survey_date, site_id, depth_m, country_island, team_leader, survey_time,
			visibility, temperature, general_comments, substrate_comments, rkc_reason, rkc_bleaching_percent,
			fish_length_mode,
			COALESCE(created_by, '00000000-0000-0000-0000-000000000000'::uuid),
			COALESCE(updated_by, '00000000-0000-0000-0000-000000000000'::uuid),
			created_at, updated_at
	`, record.SurveyDate, siteID, record.DepthM, record.CountryIsland, record.TeamLeader, record.SurveyTime,
		record.Visibility, record.Temperature, record.GeneralComments, record.SubstrateComments,
		record.RKCReason, nullableFloat64(record.RKCBleachingPercent), string(record.FishLengthMode),
		nullableUUID(record.CreatedBy)))
	if err != nil {
		return service.ReefCheckSurvey{}, err
	}

	if err := r.insertSurveyRecorders(ctx, survey.ID, record.Recorders); err != nil {
		return service.ReefCheckSurvey{}, err
	}
	if err := r.insertSurveySegments(ctx, survey.ID, record.Segments); err != nil {
		return service.ReefCheckSurvey{}, err
	}
	if err := r.insertSubstratePoints(ctx, survey.ID, record.SubstratePoints); err != nil {
		return service.ReefCheckSurvey{}, err
	}
	if err := r.insertSubstrateBleaching(ctx, survey.ID, record.SubstrateBleaching); err != nil {
		return service.ReefCheckSurvey{}, err
	}
	if err := r.insertMetricCounts(ctx, survey.ID, record.MetricCounts); err != nil {
		return service.ReefCheckSurvey{}, err
	}
	return survey, nil
}

func (r ReefCheckSurveyRepository) createSurveyLocation(ctx context.Context, record service.ReefCheckSurveyRecord) (uuid.UUID, error) {
	var locationID uuid.UUID
	err := r.db.QueryRow(ctx, `
		INSERT INTO locations (chinese_name, english_name, created_by, updated_by)
		VALUES ($1, $2, $3, $3)
		RETURNING id
	`, record.Site.LocationName, record.Site.LocationName, nullableUUID(record.CreatedBy)).Scan(&locationID)
	return locationID, translateError(err)
}

func (r ReefCheckSurveyRepository) createSurveySite(ctx context.Context, locationID uuid.UUID, record service.ReefCheckSurveyRecord) (uuid.UUID, error) {
	var siteID uuid.UUID
	err := r.db.QueryRow(ctx, `
		INSERT INTO sites (location_id, county, chinese_name, english_name, latitude, longitude, created_by, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $7)
		RETURNING id
	`, locationID, record.Site.County, record.Site.SiteName, record.Site.SiteEnglishName, record.Site.Latitude, record.Site.Longitude, nullableUUID(record.CreatedBy)).Scan(&siteID)
	return siteID, translateError(err)
}

func (r ReefCheckSurveyRepository) insertSurveyRecorders(ctx context.Context, surveyID uuid.UUID, recorders []service.ReefCheckRecorderInput) error {
	for _, recorder := range recorders {
		_, err := r.db.Exec(ctx, `
			INSERT INTO reef_check_survey_recorders (survey_id, role, user_id, recorder_name)
			VALUES ($1, $2, $3, $4)
		`, surveyID, string(recorder.Role), nullableUUID(recorder.UserID), recorder.RecorderName)
		if err != nil {
			return translateError(err)
		}
	}
	return nil
}

func (r ReefCheckSurveyRepository) insertSurveySegments(ctx context.Context, surveyID uuid.UUID, segments []service.ReefCheckSegmentInput) error {
	for _, segment := range segments {
		_, err := r.db.Exec(ctx, `
			INSERT INTO reef_check_segments (survey_id, segment_index, label, start_m, end_m)
			VALUES ($1, $2, $3, $4, $5)
		`, surveyID, segment.Index, segment.Label, segment.StartM, segment.EndM)
		if err != nil {
			return translateError(err)
		}
	}
	return nil
}

func (r ReefCheckSurveyRepository) insertSubstratePoints(ctx context.Context, surveyID uuid.UUID, points []service.SubstratePointInput) error {
	for _, point := range points {
		_, err := r.db.Exec(ctx, `
			INSERT INTO substrate_points (survey_id, segment_index, point_index, transect_m, substrate_code)
			VALUES ($1, $2, $3, $4, $5)
		`, surveyID, point.SegmentIndex, point.PointIndex, point.TransectM, point.Code)
		if err != nil {
			return translateError(err)
		}
	}
	return nil
}

func (r ReefCheckSurveyRepository) insertSubstrateBleaching(ctx context.Context, surveyID uuid.UUID, rows []service.SubstrateBleachingInput) error {
	for _, row := range rows {
		_, err := r.db.Exec(ctx, `
			INSERT INTO substrate_bleaching_counts (survey_id, segment_index, hc_bleached_count, sc_bleached_count)
			VALUES ($1, $2, $3, $4)
		`, surveyID, row.SegmentIndex, row.HCBleachedCount, row.SCBleachedCount)
		if err != nil {
			return translateError(err)
		}
	}
	return nil
}

func (r ReefCheckSurveyRepository) insertMetricCounts(ctx context.Context, surveyID uuid.UUID, counts []service.ReefCheckMetricCountInput) error {
	metricIDs := map[service.ReefCheckMetricID]uuid.UUID{}
	for _, count := range counts {
		id := service.ReefCheckMetricID{Module: count.Module, MetricKey: count.MetricKey}
		metricID, ok := metricIDs[id]
		if !ok {
			var err error
			metricID, err = r.metricID(ctx, id)
			if err != nil {
				return err
			}
			metricIDs[id] = metricID
		}
		_, err := r.db.Exec(ctx, `
			INSERT INTO reef_check_metric_counts (survey_id, segment_index, metric_id, count, comment)
			VALUES ($1, $2, $3, $4, $5)
		`, surveyID, count.SegmentIndex, metricID, count.Count, count.Comment)
		if err != nil {
			return translateError(err)
		}
	}
	return nil
}

func (r ReefCheckSurveyRepository) metricID(ctx context.Context, id service.ReefCheckMetricID) (uuid.UUID, error) {
	var metricID uuid.UUID
	err := r.db.QueryRow(ctx, `
		SELECT id
		FROM reef_check_metrics
		WHERE module = $1 AND key = $2
	`, string(id.Module), id.MetricKey).Scan(&metricID)
	return metricID, translateError(err)
}

func (r ReefCheckSurveyRepository) listReefCheckSurveys(ctx context.Context, where string, args ...any) ([]service.ReefCheckSurvey, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, survey_date, site_id, depth_m, country_island, team_leader, survey_time,
			visibility, temperature, general_comments, substrate_comments, rkc_reason, rkc_bleaching_percent,
			fish_length_mode,
			COALESCE(created_by, '00000000-0000-0000-0000-000000000000'::uuid),
			COALESCE(updated_by, '00000000-0000-0000-0000-000000000000'::uuid),
			created_at, updated_at
		FROM reef_check_surveys
		`+where+`
		ORDER BY survey_date DESC, created_at DESC
	`, args...)
	if err != nil {
		return nil, translateError(err)
	}
	defer rows.Close()

	var surveys []service.ReefCheckSurvey
	for rows.Next() {
		survey, err := scanReefCheckSurvey(rows)
		if err != nil {
			return nil, err
		}
		surveys = append(surveys, survey)
	}
	return surveys, translateError(rows.Err())
}

func scanReefCheckSurvey(row observationScanner) (service.ReefCheckSurvey, error) {
	var survey service.ReefCheckSurvey
	var fishLengthMode string
	err := row.Scan(
		&survey.ID,
		&survey.SurveyDate,
		&survey.SiteID,
		&survey.DepthM,
		&survey.CountryIsland,
		&survey.TeamLeader,
		&survey.SurveyTime,
		&survey.Visibility,
		&survey.Temperature,
		&survey.GeneralComments,
		&survey.SubstrateComments,
		&survey.RKCReason,
		&survey.RKCBleachingPercent,
		&fishLengthMode,
		&survey.CreatedBy,
		&survey.UpdatedBy,
		&survey.CreatedAt,
		&survey.UpdatedAt,
	)
	survey.FishLengthMode = service.ReefCheckFishLengthMode(fishLengthMode)
	return survey, translateError(err)
}

func nullableUUID(value uuid.UUID) any {
	if value == uuid.Nil {
		return nil
	}
	return value
}

func nullableFloat64(value *float64) any {
	if value == nil {
		return nil
	}
	return *value
}
