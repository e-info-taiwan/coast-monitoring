package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ReefData types describe the imported v1.7 domain, independent of the legacy UUID surveys.
type ReefDataEvent struct {
	MissingReason     string   `json:"missing_reason"`
	PublicationStatus string   `json:"publication_status"`
	ID                int      `json:"id"`
	EventID           string   `json:"event_id"`
	SurveyID          int      `json:"survey_id"`
	SurveyDate        string   `json:"survey_date"`
	EventTime         string   `json:"event_time"`
	DepthM            float64  `json:"depth_m"`
	SiteID            int      `json:"site_id"`
	SiteName          string   `json:"site_name"`
	SiteEnglish       string   `json:"site_english"`
	Region            string   `json:"region"`
	County            string   `json:"county"`
	Location          string   `json:"location"`
	Latitude          *float64 `json:"latitude"`
	Longitude         *float64 `json:"longitude"`
	StartDate         string   `json:"start_date"`
	EndDate           string   `json:"end_date"`
	Label             string   `json:"label"`
	Methods           []string `json:"methods"`
	CWAStationID      *string  `json:"cwa_station_id,omitempty"`
	CWAStationName    *string  `json:"cwa_station_name,omitempty"`
	WaterTemp         *float64 `json:"water_temp_c,omitempty"`
}

type ReefDataMetadata struct {
	StartTime     *string  `json:"start_time"`
	WaterTemp     *float64 `json:"water_temp_c"`
	VisibilityMin *float64 `json:"visibility_min_m"`
	VisibilityMax *float64 `json:"visibility_max_m"`
	Comments      *string  `json:"comments"`
	RKCNote       *string  `json:"rkc_bleaching_note"`
}

type ReefDataPoint struct {
	ID       int64   `json:"id"`
	Segment  int     `json:"segment"`
	Position float64 `json:"position_m"`
	Layer    string  `json:"substrate_layer"`
	Code     string  `json:"substrate_code"`
}

type ReefDataBleaching struct {
	HCPercent *float64 `json:"hc_percent"`
	SCPercent *float64 `json:"sc_percent"`
	HCStatus  string   `json:"hc_record_status"`
	SCStatus  string   `json:"sc_record_status"`
	ID        int64    `json:"id"`
	Segment   int      `json:"segment"`
	HC        int      `json:"hc_bleached_count"`
	SC        int      `json:"sc_bleached_count"`
}

type ReefDataBelt struct {
	RecordStatus string `json:"record_status"`
	ID           int64  `json:"id"`
	Segment      int    `json:"segment"`
	TaxonID      int    `json:"taxon_id"`
	Group        string `json:"taxon_group"`
	NameZH       string `json:"name_zh"`
	NameEN       string `json:"name_en"`
	Size         string `json:"size_class"`
	Aggregate    bool   `json:"is_aggregate"`
	Count        int    `json:"count"`
}

type ReefDataImpact struct {
	DerivedLevel *float64 `json:"derived_level"`
	RecordStatus string   `json:"record_status"`
	ID           int64    `json:"id"`
	Segment      int      `json:"segment"`
	TypeID       int      `json:"impact_type_id"`
	Group        string   `json:"impact_group"`
	NameZH       string   `json:"name_zh"`
	NameEN       string   `json:"name_en"`
	ValueType    string   `json:"value_type"`
	HasRawCount  bool     `json:"has_raw_count"`
	RawValue     float64  `json:"raw_value"`
}

type ReefDataParticipant struct {
	ID        int        `json:"id"`
	DiverID   int        `json:"diver_id"`
	UserID    *uuid.UUID `json:"user_id,omitempty"`
	Role      string     `json:"role"`
	NameZH    string     `json:"name_zh"`
	NameEN    string     `json:"name_en"`
	Code      string     `json:"reef_check_code"`
	UserEmail string     `json:"user_email,omitempty"`
	UserName  string     `json:"user_name,omitempty"`
}

type ReefDataSite struct {
	ID             int      `json:"id"`
	NameZH         string   `json:"name_zh"`
	NameEN         string   `json:"name_en"`
	Region         string   `json:"region"`
	County         string   `json:"county"`
	Location       string   `json:"location"`
	Latitude       *float64 `json:"latitude"`
	Longitude      *float64 `json:"longitude"`
	IsActive       bool     `json:"is_active"`
	CWAStationID   *string  `json:"cwa_station_id,omitempty"`
	CWAStationName *string  `json:"cwa_station_name,omitempty"`
}

type ReefDataSiteInput struct {
	Region       string   `json:"region"`
	County       string   `json:"county"`
	Location     string   `json:"location"`
	NameZH       string   `json:"name_zh"`
	NameEN       string   `json:"name_en"`
	Latitude     *float64 `json:"latitude,omitempty"`
	Longitude    *float64 `json:"longitude,omitempty"`
	IsActive     *bool    `json:"is_active,omitempty"`
	CWAStationID *string  `json:"cwa_station_id,omitempty"`
}

type ReefDataUser struct {
	ID    uuid.UUID `json:"id"`
	Email string    `json:"email"`
	Name  string    `json:"name"`
	Role  string    `json:"role"`
}

type ReefDataDiver struct {
	ID            int        `json:"id"`
	NameZH        string     `json:"name_zh"`
	NameEN        string     `json:"name_en"`
	ReefCheckCode string     `json:"reef_check_code"`
	UserID        *uuid.UUID `json:"user_id,omitempty"`
	UserEmail     string     `json:"user_email,omitempty"`
	IsActive      bool       `json:"is_active"`
}

type ReefDataDiverInput struct {
	NameZH        string     `json:"name_zh"`
	NameEN        string     `json:"name_en"`
	ReefCheckCode string     `json:"reef_check_code"`
	UserID        *uuid.UUID `json:"user_id,omitempty"`
	IsActive      *bool      `json:"is_active,omitempty"`
}

type ReefDataTaxon struct {
	ID          int    `json:"id"`
	TaxonGroup  string `json:"taxon_group"`
	NameZH      string `json:"name_zh"`
	NameEN      string `json:"name_en"`
	SizeClass   string `json:"size_class"`
	IsAggregate bool   `json:"is_aggregate"`
	AggregateOf string `json:"aggregate_of"`
	SortOrder   int    `json:"sort_order"`
	IsActive    bool   `json:"is_active"`
}

type ReefDataTaxonInput struct {
	TaxonGroup  string `json:"taxon_group"`
	NameZH      string `json:"name_zh"`
	NameEN      string `json:"name_en"`
	SizeClass   string `json:"size_class"`
	IsAggregate *bool  `json:"is_aggregate,omitempty"`
	AggregateOf string `json:"aggregate_of"`
	SortOrder   *int   `json:"sort_order,omitempty"`
	IsActive    *bool  `json:"is_active,omitempty"`
}

type ReefDataSubstrateType struct {
	Code        string `json:"code"`
	NumericCode int    `json:"numeric_code"`
	NameZH      string `json:"name_zh"`
	NameEN      string `json:"name_en"`
	SortOrder   int    `json:"sort_order"`
	IsActive    bool   `json:"is_active"`
}

type ReefDataSubstrateTypeInput struct {
	Code        string `json:"code"`
	NumericCode *int   `json:"numeric_code,omitempty"`
	NameZH      string `json:"name_zh"`
	NameEN      string `json:"name_en"`
	SortOrder   *int   `json:"sort_order,omitempty"`
	IsActive    *bool  `json:"is_active,omitempty"`
}

type ReefDataImpactType struct {
	ID          int    `json:"id"`
	ImpactGroup string `json:"impact_group"`
	NameZH      string `json:"name_zh"`
	NameEN      string `json:"name_en"`
	ValueType   string `json:"value_type"`
	HasRawCount bool   `json:"has_raw_count"`
	SortOrder   int    `json:"sort_order"`
	IsActive    bool   `json:"is_active"`
}

type ReefDataImpactTypeInput struct {
	ImpactGroup string `json:"impact_group"`
	NameZH      string `json:"name_zh"`
	NameEN      string `json:"name_en"`
	ValueType   string `json:"value_type"`
	HasRawCount *bool  `json:"has_raw_count,omitempty"`
	SortOrder   *int   `json:"sort_order,omitempty"`
	IsActive    *bool  `json:"is_active,omitempty"`
}

type ReefDataParticipantInput struct {
	DiverID       *int       `json:"diver_id,omitempty"`
	UserID        *uuid.UUID `json:"user_id,omitempty"`
	NameZH        string     `json:"name_zh,omitempty"`
	NameEN        string     `json:"name_en,omitempty"`
	ReefCheckCode string     `json:"reef_check_code,omitempty"`
	Role          string     `json:"role"`
}

type ReefDataCreateInput struct {
	FishSizeMode string   `json:"fish_size_mode"`
	SiteID       int      `json:"site_id"`
	SurveyDate   string   `json:"survey_date"`
	StartDate    string   `json:"start_date,omitempty"`
	EndDate      string   `json:"end_date,omitempty"`
	EventTime    string   `json:"event_time"`
	DepthM       float64  `json:"depth_m"`
	Label        string   `json:"label,omitempty"`
	Methods      []string `json:"methods"`
	CWAStationID *string  `json:"cwa_station_id,omitempty"`
	WaterTemp    *float64 `json:"water_temp_c,omitempty"`
}

func (c *ReefDataCreateInput) Validate(validSiteIDs map[int]bool) error {
	if c.SiteID <= 0 || (validSiteIDs != nil && !validSiteIDs[c.SiteID]) {
		return fmt.Errorf("%w: 請選擇有效樣點", ErrValidation)
	}
	if _, err := time.Parse("2006-01-02", c.SurveyDate); err != nil {
		return fmt.Errorf("%w: 調查日期格式錯誤 (YYYY-MM-DD)", ErrValidation)
	}
	if c.StartDate == "" {
		c.StartDate = c.SurveyDate
	}
	if c.EndDate == "" {
		c.EndDate = c.StartDate
	}
	if _, err := time.Parse("15:04", strings.ReplaceAll(c.EventTime, "-", ":")); err != nil {
		return fmt.Errorf("%w: 請填氣瓶開始時間 HH:MM", ErrValidation)
	}
	if !finite(c.DepthM) || c.DepthM <= 0 || c.DepthM > 100 {
		return fmt.Errorf("%w: 水深必須介於 0–100 公尺", ErrValidation)
	}
	if len(c.Methods) == 0 {
		return fmt.Errorf("%w: 請至少勾選一種調查方法", ErrValidation)
	}
	allowedMethods := map[string]bool{"line": true, "belt_fish": true, "belt_invert": true}
	seen := map[string]bool{}
	for _, m := range c.Methods {
		if seen[m] {
			return fmt.Errorf("%w: 調查方法重複", ErrValidation)
		}
		seen[m] = true
		if m == "belt_fish" && c.FishSizeMode != "split" && c.FishSizeMode != "combined" {
			return fmt.Errorf("%w: 請指定魚類體長模式", ErrValidation)
		}
		if !allowedMethods[m] {
			return fmt.Errorf("%w: 未知的調查方法 %s", ErrValidation, m)
		}
	}
	return nil
}

type ReefCheckEventInput struct {
	EventKey         string  `json:"event_key"`
	EventID          string  `json:"event_id"`
	SiteID           int     `json:"site_id,omitempty"`
	SiteNameENLookup string  `json:"site_name_en__lookup"`
	SiteNameZH       string  `json:"site_name_zh"`
	SurveyDate       string  `json:"survey_date"`
	EventTime        string  `json:"event_time"`
	DepthM           float64 `json:"depth_m"`
	CWAStationID     *string `json:"cwa_station_id,omitempty"`
}

type ReefCheckTransectInput struct {
	FishSizeMode      string   `json:"fish_size_mode"`
	ClassifyHardCoral bool     `json:"classify_hard_coral"`
	TransectKey       string   `json:"transect_key"`
	EventID           string   `json:"event_id"`
	Method            string   `json:"method"`
	StartTime         string   `json:"start_time"`
	WaterTempC        *float64 `json:"water_temp_c"`
	VisibilityM       *float64 `json:"visibility_m,omitempty"`
	VisibilityMinM    *float64 `json:"visibility_min_m,omitempty"`
	VisibilityMaxM    *float64 `json:"visibility_max_m,omitempty"`
	Recorders         []string `json:"recorders"`
	TeamLeader        string   `json:"team_leader"`
	TeamScientist     string   `json:"team_scientist"`
	Comments          string   `json:"comments"`
}

type ReefCheckSubstratePointInput struct {
	TransectKey    string  `json:"transect_key"`
	Segment        int     `json:"segment"`
	PositionM      float64 `json:"position_m"`
	SubstrateCode  string  `json:"substrate_code"`
	SubstrateLayer string  `json:"substrate_layer"`
}

type ReefCheckBleachingInput struct {
	RecordStatus   string `json:"record_status"`
	TransectKey    string `json:"transect_key"`
	SubstrateCode  string `json:"substrate_code"`
	Segment        int    `json:"segment"`
	BleachedPoints *int   `json:"bleached_points"`
}

type ReefCheckBeltObservationInput struct {
	TransectKey          string `json:"transect_key"`
	TaxonGroup           string `json:"taxon_group"`
	TaxonNameENLookup    string `json:"taxon_name_en__lookup"`
	TaxonSizeClassLookup string `json:"taxon_size_class__lookup"`
	TaxonLocalRowID      string `json:"taxon_local_row_id"`
	Segment              int    `json:"segment"`
	Count                *int   `json:"count"`
	RecordStatus         string `json:"record_status"`
}

type ReefCheckImpactObservationInput struct {
	RecordStatus       string   `json:"record_status"`
	TransectKey        string   `json:"transect_key"`
	ImpactGroup        string   `json:"impact_group"`
	ImpactNameENLookup string   `json:"impact_name_en__lookup"`
	ImpactValueType    string   `json:"impact_value_type"`
	Segment            int      `json:"segment"`
	RawValue           *float64 `json:"raw_value"`
}

type ReefCheckMudAuditInput struct {
	PositionM float64 `json:"position_m"`
	Segment   int     `json:"segment"`
	Surface   string  `json:"surface"`
	Down      string  `json:"down"`
	Canonical string  `json:"canonical"`
}

type ReefCheckSurveySubmission struct {
	Format             string                            `json:"format"`
	Status             string                            `json:"status"`
	MissingReason      string                            `json:"missing_reason"`
	ValidationIssues   []string                          `json:"validation_issues"`
	Event              ReefCheckEventInput               `json:"event"`
	Transects          []ReefCheckTransectInput          `json:"transects"`
	SubstratePoints    []ReefCheckSubstratePointInput    `json:"substrate_points"`
	SubstrateBleaching []ReefCheckBleachingInput         `json:"substrate_bleaching"`
	BeltObservations   []ReefCheckBeltObservationInput   `json:"belt_observations"`
	ImpactObservations []ReefCheckImpactObservationInput `json:"impact_observations"`
	MudAudit           []ReefCheckMudAuditInput          `json:"mud_audit"`
}

func (s *ReefCheckSurveySubmission) Validate() error {
	bad := func(msg string) error { return fmt.Errorf("%w: %s", ErrValidation, msg) }
	if s.Event.SurveyDate == "" {
		return bad("缺少調查日期")
	}
	if _, err := time.Parse("2006-01-02", s.Event.SurveyDate); err != nil {
		return bad("調查日期格式錯誤 (YYYY-MM-DD)")
	}
	if !finite(s.Event.DepthM) || s.Event.DepthM <= 0 || s.Event.DepthM > 100 {
		return bad("深度必須介於 0–100 公尺")
	}
	if len(s.Transects) == 0 {
		return bad("請至少包含一種調查方法")
	}
	allowedMethods := map[string]bool{"line": true, "belt_fish": true, "belt_invert": true}
	for _, t := range s.Transects {
		if !allowedMethods[t.Method] {
			return bad(fmt.Sprintf("未知的調查方法 %s", t.Method))
		}
		visibilityMin, visibilityMax := t.VisibilityMinM, t.VisibilityMaxM
		if visibilityMin == nil {
			visibilityMin = t.VisibilityM
		}
		if visibilityMax == nil {
			visibilityMax = t.VisibilityM
		}
		if t.WaterTempC != nil && (!finite(*t.WaterTempC) || *t.WaterTempC < 0 || *t.WaterTempC > 45) {
			return bad("水溫必須介於 0–45 °C")
		}
		for _, v := range []*float64{visibilityMin, visibilityMax} {
			if v != nil && (!finite(*v) || *v < 0 || *v > 100) {
				return bad("能見度必須介於 0–100 公尺")
			}
		}
		if visibilityMin != nil && visibilityMax != nil && *visibilityMin > *visibilityMax {
			return bad("最低能見度不能大於最高能見度")
		}
	}
	return s.validateObservations()
}

type ReefCheckSubmissionResult struct {
	Status       string   `json:"status"`
	ReceiptID    string   `json:"receipt_id"`
	EventID      string   `json:"event_id"`
	EventDBID    int      `json:"id"`
	SurveyDBID   int      `json:"survey_id"`
	SiteNameZH   string   `json:"site_name_zh"`
	SiteNameEN   string   `json:"site_name_en"`
	SurveyDate   string   `json:"survey_date"`
	EventTime    string   `json:"event_time"`
	DepthM       float64  `json:"depth_m"`
	Methods      []string `json:"methods"`
	ReviewStatus string   `json:"review_status"`
	SavedAt      string   `json:"saved_at"`
}

type ReefCheckConfig struct {
	Sites   []ReefDataSite       `json:"sites"`
	Codes   []ReefDataCode       `json:"codes"`
	Taxa    []ReefDataTaxon      `json:"taxa"`
	Impacts []ReefDataImpactType `json:"impacts"`
}

type ReefDataTransect struct {
	FishSizeMode      *string         `json:"fish_size_mode"`
	ClassifyHardCoral bool            `json:"classify_hard_coral"`
	Summaries         json.RawMessage `json:"summaries"`
	ID                int             `json:"id"`
	EventID           string          `json:"event_id"`
	Method            string          `json:"method"`
	ReefDataMetadata
	Points       []ReefDataPoint       `json:"points"`
	Bleaching    []ReefDataBleaching   `json:"bleaching"`
	Belt         []ReefDataBelt        `json:"belt"`
	Impacts      []ReefDataImpact      `json:"impacts"`
	Participants []ReefDataParticipant `json:"participants"`
	Version      string                `json:"version"`
}

type ReefDataDetail struct {
	Event     ReefDataEvent      `json:"event"`
	Transects []ReefDataTransect `json:"transects"`
}

type ReefDataCode struct {
	Code        string `json:"code"`
	NumericCode int    `json:"numeric_code"`
	NameZH      string `json:"name_zh"`
	Active      bool   `json:"is_active"`
}

type ReefDataChange struct {
	RecordStatus string   `json:"record_status,omitempty"`
	HCStatus     string   `json:"hc_record_status,omitempty"`
	SCStatus     string   `json:"sc_record_status,omitempty"`
	Kind         string   `json:"kind"`
	ID           int64    `json:"id"`
	Code         string   `json:"code,omitempty"`
	Value        *float64 `json:"value,omitempty"`
	HC           *int     `json:"hc,omitempty"`
	SC           *int     `json:"sc,omitempty"`
}

type ReefDataUpdate struct {
	Version  string            `json:"version"`
	Metadata *ReefDataMetadata `json:"metadata,omitempty"`
	Changes  []ReefDataChange  `json:"changes"`
}

func (t ReefDataTransect) Revision() string {
	t.Version = ""
	b, _ := json.Marshal(t)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// Validate checks changes against the stored row and master semantics, never client-supplied types.
func (u ReefDataUpdate) Validate(t ReefDataTransect, codes []ReefDataCode) error {
	bad := func(message string) error { return fmt.Errorf("%w: %s", ErrValidation, message) }
	if u.Version == "" {
		return bad("缺少資料版本，請重新載入")
	}
	if u.Version != t.Revision() {
		return ErrConflict
	}
	if u.Metadata == nil && len(u.Changes) == 0 {
		return bad("沒有修改內容")
	}
	if len(u.Changes) > 2000 {
		return bad("單次修改筆數過多")
	}
	if m := u.Metadata; m != nil {
		for _, v := range []*float64{m.WaterTemp, m.VisibilityMin, m.VisibilityMax} {
			if v != nil && (!finite(*v) || *v < 0) {
				return bad("水溫與能見度必須是非負數")
			}
		}
		if m.WaterTemp != nil && *m.WaterTemp > 40 {
			return bad("水溫必須介於 0–40°C")
		}
		if m.VisibilityMin != nil && m.VisibilityMax != nil && *m.VisibilityMin > *m.VisibilityMax {
			return bad("能見度下限不得大於上限")
		}
	}
	seen := map[string]bool{}
	for _, c := range u.Changes {
		key := fmt.Sprintf("%s:%d", c.Kind, c.ID)
		if c.ID <= 0 || seen[key] {
			return bad("重複或無效的觀測編號")
		}
		seen[key] = true
		if c.RecordStatus == "not_recorded" && c.Value != nil {
			return bad("NA 不接受數值")
		}
		for _, status := range []string{c.HCStatus, c.SCStatus} {
			if status != "" && status != "recorded" && status != "not_recorded" {
				return bad("無效白化紀錄狀態")
			}
		}
		if c.RecordStatus != "" && c.RecordStatus != "recorded" && c.RecordStatus != "not_recorded" {
			return bad("無效紀錄狀態")
		}
		found := false
		switch c.Kind {
		case "point":
			if t.Method != "line" {
				return bad("底質只適用於 Line")
			}
			for _, p := range t.Points {
				if p.ID == c.ID {
					found = true
					validCode := false
					for _, code := range codes {
						if code.Code == c.Code && (code.Active || c.Code == p.Code) {
							validCode = true
						}
					}
					if !validCode {
						return bad("無效底質代碼")
					}
				}
			}
		case "bleaching":
			if t.Method != "line" || c.HCStatus != "not_recorded" && (c.HC == nil || *c.HC < 0 || *c.HC > 40) || c.SCStatus != "not_recorded" && (c.SC == nil || *c.SC < 0 || *c.SC > 40) {
				return bad("每段白化點數必須介於 0–40")
			}
			for _, row := range t.Bleaching {
				if row.ID == c.ID {
					found = true
				}
			}
		case "belt":
			if c.RecordStatus != "not_recorded" && (c.Value == nil || !integer(*c.Value) || *c.Value < 0 || *c.Value > 2147483647) {
				return bad("生物數量必須是非負整數")
			}
			for _, row := range t.Belt {
				if row.ID == c.ID {
					found = true
					if row.Aggregate || (row.Group == "fish" && t.Method != "belt_fish") || (row.Group != "fish" && t.Method != "belt_invert") {
						return bad("物種類別與調查方法不符，總數列不能手填")
					}
				}
			}
		case "impact":
			if t.Method != "belt_invert" || c.RecordStatus != "not_recorded" && (c.Value == nil || !finite(*c.Value) || *c.Value < 0) {
				return bad("影響原始值必須是非負數")
			}
			for _, row := range t.Impacts {
				if row.ID == c.ID {
					found = true
					if c.RecordStatus == "not_recorded" {
						continue
					}
					if row.ValueType == "percent" {
						if *c.Value > 100 {
							return bad("百分比必須介於 0–100")
						}
					} else if !integer(*c.Value) || (row.ValueType == "level" && !row.HasRawCount && *c.Value > 3) {
						return bad("件數必須是整數；直接等級必須介於 0–3")
					}
				}
			}
		default:
			return bad("未知的觀測類型")
		}
		if !found {
			return bad("觀測不屬於這條穿越線")
		}
	}
	return nil
}
func finite(v float64) bool  { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func integer(v float64) bool { return finite(v) && math.Trunc(v) == v }
