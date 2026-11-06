package repository

import (
	"context"
	"fmt"
	"time"

	"coast-monitoring/internal/service"
)

// Read-only preview of the same cached CWA observation used at submission.
// Never scrapes/backfills or changes site station assignments on a volunteer request.
func (r ReefDataRepository) EntryEnvironment(ctx context.Context, in service.ReefEntryEnvironmentInput) (service.ReefEntryEnvironment, error) {
	out := service.ReefEntryEnvironment{SiteID: in.SiteID, Source: "cwa_reference"}
	if err := in.Validate(); err != nil {
		return out, err
	}
	activity, err := r.VerifyActivity(ctx, in.ActivityCode)
	if err != nil {
		return out, err
	}
	if !activity.Allows(in.SiteID, in.SurveyDate, time.Now()) {
		return out, fmt.Errorf("%w: 樣點或日期不在活動範圍", service.ErrValidation)
	}
	if err = r.db.QueryRow(ctx, `SELECT s.cwa_station_id,c.name_zh FROM site s LEFT JOIN cwa_marine_station c ON c.id=s.cwa_station_id WHERE s.id=$1 AND s.is_active`, in.SiteID).Scan(&out.StationID, &out.StationName); err != nil {
		return out, translateError(err)
	}
	if out.StationID == nil || *out.StationID == "" {
		out.Message = "此樣點尚未設定氣象署測站，請依手板填寫實測水溫。"
		return out, nil
	}
	value, err := resolveEventTemperature(ctx, r.db, out.StationID, in.SurveyDate, in.EventTime, nil, "automatic")
	if err != nil {
		return out, err
	}
	out.SeaTempC, out.ObservedAt = value.Value, value.ObservedAt
	if out.SeaTempC == nil {
		out.Message = "此測站在調查當日沒有可用海溫，請依手板填寫；不改用其他日期。"
	}
	return out, nil
}
