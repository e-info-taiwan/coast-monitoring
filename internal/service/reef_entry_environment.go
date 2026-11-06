package service

import (
	"fmt"
	"time"
)

type ReefEntryEnvironmentInput struct {
	ActivityCode string `json:"activity_code"`
	SiteID       int    `json:"site_id"`
	SurveyDate   string `json:"survey_date"`
	EventTime    string `json:"event_time"`
}

func (in ReefEntryEnvironmentInput) Validate() error {
	if in.ActivityCode == "" || in.SiteID <= 0 {
		return fmt.Errorf("%w: 請先驗證活動並選擇樣點", ErrValidation)
	}
	if _, err := time.Parse("2006-01-02", in.SurveyDate); err != nil {
		return fmt.Errorf("%w: 調查日期無效", ErrValidation)
	}
	if _, err := time.Parse("15:04", in.EventTime); err != nil || len(in.EventTime) != 5 {
		return fmt.Errorf("%w: 開始時間請填 24 小時制 HH:MM", ErrValidation)
	}
	return nil
}

type ReefEntryEnvironment struct {
	SiteID      int        `json:"site_id"`
	StationID   *string    `json:"station_id"`
	StationName *string    `json:"station_name"`
	SeaTempC    *float64   `json:"sea_temp_c"`
	ObservedAt  *time.Time `json:"observed_at"`
	Source      string     `json:"source"`
	Message     string     `json:"message"`
}
