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

func (r ReefCheckSurveyRepository) GetReefCheckSurvey(ctx context.Context, id uuid.UUID) (service.ReefCheckSurveyDetail, error) {
	survey, err := scanReefCheckSurvey(r.db.QueryRow(ctx, `
		SELECT id, survey_date, site_id, depth_m, country_island, team_leader, survey_time,
			visibility, temperature, general_comments, substrate_comments, rkc_reason, rkc_bleaching_percent,
			fish_length_mode, COALESCE(created_by, '00000000-0000-0000-0000-000000000000'::uuid),
			COALESCE(updated_by, '00000000-0000-0000-0000-000000000000'::uuid), created_at, updated_at
		FROM reef_check_surveys WHERE id = $1
	`, id))
	if err != nil {
		return service.ReefCheckSurveyDetail{}, err
	}
	detail := service.ReefCheckSurveyDetail{Survey: survey}
	if detail.Recorders, err = r.getRecorders(ctx, id); err != nil {
		return service.ReefCheckSurveyDetail{}, err
	}
	if detail.Segments, err = r.getSegments(ctx, id); err != nil {
		return service.ReefCheckSurveyDetail{}, err
	}
	if detail.SubstratePoints, err = r.getSubstratePoints(ctx, id); err != nil {
		return service.ReefCheckSurveyDetail{}, err
	}
	if detail.SubstrateBleaching, err = r.getSubstrateBleaching(ctx, id); err != nil {
		return service.ReefCheckSurveyDetail{}, err
	}
	if detail.MetricCounts, err = r.getMetricCounts(ctx, id); err != nil {
		return service.ReefCheckSurveyDetail{}, err
	}
	return detail, nil
}

func (r ReefCheckSurveyRepository) UpdateReefCheckSurvey(ctx context.Context, id uuid.UUID, record service.ReefCheckSurveyRecord) (service.ReefCheckSurvey, error) {
	survey, err := scanReefCheckSurvey(r.db.QueryRow(ctx, `
		UPDATE reef_check_surveys SET survey_date=$2, site_id=$3, depth_m=$4, country_island=$5,
			team_leader=$6, survey_time=$7, visibility=$8, temperature=$9, general_comments=$10,
			substrate_comments=$11, rkc_reason=$12, rkc_bleaching_percent=$13, fish_length_mode=$14,
			updated_by=$15, updated_at=now() WHERE id=$1
		RETURNING id, survey_date, site_id, depth_m, country_island, team_leader, survey_time,
			visibility, temperature, general_comments, substrate_comments, rkc_reason, rkc_bleaching_percent,
			fish_length_mode, COALESCE(created_by, '00000000-0000-0000-0000-000000000000'::uuid),
			COALESCE(updated_by, '00000000-0000-0000-0000-000000000000'::uuid), created_at, updated_at
	`, id, record.SurveyDate, record.SiteID, record.DepthM, record.CountryIsland, record.TeamLeader,
		record.SurveyTime, record.Visibility, record.Temperature, record.GeneralComments, record.SubstrateComments,
		record.RKCReason, nullableFloat64(record.RKCBleachingPercent), string(record.FishLengthMode), nullableUUID(record.UpdatedBy)))
	if err != nil {
		return service.ReefCheckSurvey{}, err
	}
	if _, err = r.db.Exec(ctx, `DELETE FROM reef_check_survey_recorders WHERE survey_id=$1`, id); err != nil {
		return service.ReefCheckSurvey{}, translateError(err)
	}
	if _, err = r.db.Exec(ctx, `DELETE FROM reef_check_segments WHERE survey_id=$1`, id); err != nil {
		return service.ReefCheckSurvey{}, translateError(err)
	}
	if err = r.insertSurveyRecorders(ctx, id, record.Recorders); err != nil {
		return service.ReefCheckSurvey{}, err
	}
	if err = r.insertSurveySegments(ctx, id, record.Segments); err != nil {
		return service.ReefCheckSurvey{}, err
	}
	if err = r.insertSubstratePoints(ctx, id, record.SubstratePoints); err != nil {
		return service.ReefCheckSurvey{}, err
	}
	if err = r.insertSubstrateBleaching(ctx, id, record.SubstrateBleaching); err != nil {
		return service.ReefCheckSurvey{}, err
	}
	if err = r.insertMetricCounts(ctx, id, record.MetricCounts); err != nil {
		return service.ReefCheckSurvey{}, err
	}
	return survey, nil
}

func (r ReefCheckSurveyRepository) DeleteReefCheckSurvey(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM reef_check_surveys WHERE id=$1`, id)
	if err != nil {
		return translateError(err)
	}
	if tag.RowsAffected() == 0 {
		return service.ErrNotFound
	}
	return nil
}

func (r ReefCheckSurveyRepository) getRecorders(ctx context.Context, id uuid.UUID) ([]service.ReefCheckRecorderInput, error) {
	rows, err := r.db.Query(ctx, `SELECT role, COALESCE(user_id, '00000000-0000-0000-0000-000000000000'::uuid), recorder_name FROM reef_check_survey_recorders WHERE survey_id=$1 ORDER BY role`, id)
	if err != nil {
		return nil, translateError(err)
	}
	defer rows.Close()
	var out []service.ReefCheckRecorderInput
	for rows.Next() {
		var v service.ReefCheckRecorderInput
		var role string
		if err := rows.Scan(&role, &v.UserID, &v.RecorderName); err != nil {
			return nil, translateError(err)
		}
		v.Role = service.ReefCheckRecorderRole(role)
		out = append(out, v)
	}
	return out, translateError(rows.Err())
}

func (r ReefCheckSurveyRepository) getSegments(ctx context.Context, id uuid.UUID) ([]service.ReefCheckSegmentInput, error) {
	rows, err := r.db.Query(ctx, `SELECT segment_index,label,start_m,end_m FROM reef_check_segments WHERE survey_id=$1 ORDER BY segment_index`, id)
	if err != nil {
		return nil, translateError(err)
	}
	defer rows.Close()
	var out []service.ReefCheckSegmentInput
	for rows.Next() {
		var v service.ReefCheckSegmentInput
		if err := rows.Scan(&v.Index, &v.Label, &v.StartM, &v.EndM); err != nil {
			return nil, translateError(err)
		}
		out = append(out, v)
	}
	return out, translateError(rows.Err())
}

func (r ReefCheckSurveyRepository) getSubstratePoints(ctx context.Context, id uuid.UUID) ([]service.SubstratePointInput, error) {
	rows, err := r.db.Query(ctx, `SELECT segment_index,point_index,transect_m,substrate_code FROM substrate_points WHERE survey_id=$1 ORDER BY segment_index,point_index`, id)
	if err != nil {
		return nil, translateError(err)
	}
	defer rows.Close()
	var out []service.SubstratePointInput
	for rows.Next() {
		var v service.SubstratePointInput
		if err := rows.Scan(&v.SegmentIndex, &v.PointIndex, &v.TransectM, &v.Code); err != nil {
			return nil, translateError(err)
		}
		out = append(out, v)
	}
	return out, translateError(rows.Err())
}

func (r ReefCheckSurveyRepository) getSubstrateBleaching(ctx context.Context, id uuid.UUID) ([]service.SubstrateBleachingInput, error) {
	rows, err := r.db.Query(ctx, `SELECT segment_index,hc_bleached_count,sc_bleached_count FROM substrate_bleaching_counts WHERE survey_id=$1 ORDER BY segment_index`, id)
	if err != nil {
		return nil, translateError(err)
	}
	defer rows.Close()
	var out []service.SubstrateBleachingInput
	for rows.Next() {
		var v service.SubstrateBleachingInput
		if err := rows.Scan(&v.SegmentIndex, &v.HCBleachedCount, &v.SCBleachedCount); err != nil {
			return nil, translateError(err)
		}
		out = append(out, v)
	}
	return out, translateError(rows.Err())
}

func (r ReefCheckSurveyRepository) getMetricCounts(ctx context.Context, id uuid.UUID) ([]service.ReefCheckMetricCountInput, error) {
	rows, err := r.db.Query(ctx, `SELECT m.module,m.key,c.segment_index,c.count,c.comment FROM reef_check_metric_counts c JOIN reef_check_metrics m ON m.id=c.metric_id WHERE c.survey_id=$1 ORDER BY m.module,m.sort_order,c.segment_index`, id)
	if err != nil {
		return nil, translateError(err)
	}
	defer rows.Close()
	var out []service.ReefCheckMetricCountInput
	for rows.Next() {
		var v service.ReefCheckMetricCountInput
		var module string
		if err := rows.Scan(&module, &v.MetricKey, &v.SegmentIndex, &v.Count, &v.Comment); err != nil {
			return nil, translateError(err)
		}
		v.Module = service.ReefCheckModule(module)
		out = append(out, v)
	}
	return out, translateError(rows.Err())
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
