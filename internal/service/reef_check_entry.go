package service

import (
	"fmt"
	"strings"
	"time"
)

type ReefCheckEntryMetadata struct {
	Weather        string   `json:"weather"`
	AirTemperature *float64 `json:"air_temperature_c"`
	Latitude       *float64 `json:"latitude"`
	Longitude      *float64 `json:"longitude"`
}
type ReefCheckPointBleachingSummary struct {
	Segment int  `json:"segment"`
	HC      *int `json:"derived_hc_bleached_point_count"`
	SC      *int `json:"derived_sc_bleached_point_count"`
}

type ReefCheckActivity struct {
	ID        int        `json:"id"`
	Name      string     `json:"name"`
	StartDate string     `json:"start_date"`
	EndDate   string     `json:"end_date"`
	SiteIDs   []int      `json:"site_ids"`
	Active    bool       `json:"is_active"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	Code      string     `json:"code,omitempty"` // returned only on creation/rotation
}

type ReefCheckActivityInput struct {
	Name      string     `json:"name"`
	StartDate string     `json:"start_date"`
	EndDate   string     `json:"end_date"`
	SiteIDs   []int      `json:"site_ids"`
	ExpiresAt *time.Time `json:"expires_at"`
}

func (a ReefCheckActivityInput) Validate() error {
	bad := func(m string) error { return fmt.Errorf("%w: %s", ErrValidation, m) }
	start, err := time.Parse("2006-01-02", a.StartDate)
	if err != nil {
		return bad("活動開始日期無效")
	}
	end, err := time.Parse("2006-01-02", a.EndDate)
	if err != nil || end.Before(start) {
		return bad("活動結束日期無效")
	}
	if strings.TrimSpace(a.Name) == "" || len(a.SiteIDs) == 0 {
		return bad("請填活動名稱及至少一個樣點")
	}
	seen := map[int]bool{}
	for _, id := range a.SiteIDs {
		if id <= 0 || seen[id] {
			return bad("活動樣點無效或重複")
		}
		seen[id] = true
	}
	return nil
}

func (a ReefCheckActivity) Allows(siteID int, date string, now time.Time) bool {
	if !a.Active || a.ExpiresAt != nil && !now.Before(*a.ExpiresAt) || date < a.StartDate || date > a.EndDate {
		return false
	}
	for _, id := range a.SiteIDs {
		if id == siteID {
			return true
		}
	}
	return false
}

func (s *ReefCheckSurveySubmission) validateTabletEntry() error {
	bad := func(m string) error { return fmt.Errorf("%w: %s", ErrValidation, m) }
	if strings.TrimSpace(s.ActivityCode) == "" {
		return bad("缺少活動代碼")
	}
	if s.Event.SiteID <= 0 {
		return bad("請選正式樣點")
	}
	if _, err := time.Parse("15:04", s.Event.EventTime); err != nil {
		return bad("缺少有效氣瓶開始時間")
	}
	m := s.EntryMetadata
	if (m.Latitude == nil) != (m.Longitude == nil) {
		return bad("經緯度需同時填寫")
	}
	if m.Latitude != nil && (!finite(*m.Latitude) || !finite(*m.Longitude) || *m.Latitude < -90 || *m.Latitude > 90 || *m.Longitude < -180 || *m.Longitude > 180) {
		return bad("經緯度範圍無效")
	}
	if m.AirTemperature != nil && (!finite(*m.AirTemperature) || *m.AirTemperature < -20 || *m.AirTemperature > 60) {
		return bad("氣溫範圍無效")
	}
	methods := map[string]bool{}
	morphology := ""
	for _, t := range s.Transects {
		if methods[t.Method] {
			return bad("同一方法不得重複")
		}
		methods[t.Method] = true
		if len(t.Recorders) == 0 {
			return bad("缺少記錄者")
		}
		for _, n := range t.Recorders {
			if strings.TrimSpace(n) == "" {
				return bad("記錄者姓名不可空白")
			}
		}
		if t.Method == "belt_fish" && t.FishLengthMode != "split" && t.FishLengthMode != "combined" {
			return bad("請指定魚類體長模式")
		}
		if t.Method == "line" {
			morphology = t.CoralMorphology
			if morphology != "general" && morphology != "classified" {
				return bad("請指定硬珊瑚型態模式")
			}
		}
	}
	unknown := false
	if methods["line"] {
		if s.BleachingMode != "point_marks" {
			return bad("新填報需以逐格白化標記計算")
		}
		seen := map[string]bool{}
		valid, rkc := 0, 0
		for _, p := range s.SubstratePoints {
			if p.SubstrateLayer != "surface" {
				return bad("泥下原值需放在 audit，不得增加底質點數")
			}
			start := float64((p.Segment - 1) * 25)
			offset := (p.PositionM - start) * 2
			if offset < 0 || offset > 39 || !integer(offset) {
				return bad("底質位置不是指定的 0.5 m 測點")
			}
			key := fmt.Sprintf("%d:%g", p.Segment, p.PositionM)
			if seen[key] {
				return bad("底質測點重複")
			}
			seen[key] = true
			coral := p.SubstrateCode == "SC" || p.SubstrateCode == "HC" || strings.HasPrefix(p.SubstrateCode, "HC-")
			if coral && p.IsBleached == nil {
				return bad("珊瑚點缺少白化核對")
			}
			if morphology == "general" && strings.HasPrefix(p.SubstrateCode, "HC-") {
				return bad("未分類型態的手板不能填 a／b／c")
			}
			if p.SubstrateCode == "NA" {
				unknown = true
			} else {
				valid++
			}
			if p.SubstrateCode == "RKC" {
				rkc++
			}
		}
		if len(seen) != 160 {
			return bad("底質必須包含四個子樣區的 160 個指定測點")
		}
		if valid > 0 && rkc*10 >= valid && strings.TrimSpace(s.RKCNote) == "" {
			return bad("RKC 達 10%，請填可能原因")
		}
	}
	groups := map[string]map[int]bool{}
	for _, b := range s.BeltObservations {
		if b.TaxonGroup == "fish" && !methods["belt_fish"] || b.TaxonGroup != "fish" && !methods["belt_invert"] {
			return bad("Belt 資料與所選方法不符")
		}
		if b.TaxonGroup != "fish" && b.TaxonGroup != "invert" && b.TaxonGroup != "rare" {
			return bad("Belt 類群無效")
		}
		key := b.TaxonGroup + "|" + b.TaxonNameENLookup + "|" + b.TaxonSizeClassLookup
		if groups[key] == nil {
			groups[key] = map[int]bool{}
		}
		if groups[key][b.Segment] {
			return bad("Belt 同一類群段次重複")
		}
		groups[key][b.Segment] = true
		if b.Count == nil {
			unknown = true
			if b.RecordStatus != "not_recorded" {
				return bad("缺值需標記 NA")
			}
		} else if b.RecordStatus != "recorded" {
			return bad("數量與記錄狀態不符")
		}
	}
	for _, segments := range groups {
		if len(segments) != 4 {
			return bad("每個 Belt 類群需記錄四個子樣區")
		}
	}
	if methods["belt_invert"] {
		for _, name := range []string{"Sharks", "Turtles", "Mantas"} {
			if len(groups["rare|"+name+"|"]) != 4 {
				return bad("鯊魚、海龜、魟均需記錄四個子樣區")
			}
		}
	}
	for _, group := range []string{"fish", "invert", "rare"} {
		method := "belt_invert"
		if group == "fish" {
			method = "belt_fish"
		}
		if methods[method] {
			found := false
			for key := range groups {
				if strings.HasPrefix(key, group+"|") {
					found = true
				}
			}
			if !found {
				return bad("已選方法缺少必要的 Belt 手板")
			}
		}
	}
	impacts := map[string]map[int]bool{}
	for _, i := range s.ImpactObservations {
		if !methods["belt_invert"] {
			return bad("環境衝擊與所選方法不符")
		}
		key := i.ImpactGroup + "|" + i.ImpactNameENLookup
		if impacts[key] == nil {
			impacts[key] = map[int]bool{}
		}
		if impacts[key][i.Segment] {
			return bad("環境衝擊段次重複")
		}
		impacts[key][i.Segment] = true
		if i.RawValue == nil {
			unknown = true
		}
		if i.ImpactGroup == "coral_damage" && i.ImpactNameENLookup == "Coral damage: other" && i.RawValue != nil && *i.RawValue > 0 && strings.TrimSpace(s.OtherDamageNote) == "" {
			return bad("其他珊瑚損害需填獨立說明")
		}
	}
	if methods["belt_invert"] {
		for _, name := range []string{"coral_damage|Coral damage: boat/anchor", "coral_damage|Coral damage: dynamite", "coral_damage|Coral damage: other", "trash|Trash: fish nets", "trash|Trash: general"} {
			if len(impacts[name]) != 4 {
				return bad("環境衝擊需有五個必要項目及四個段次")
			}
		}
	}
	if unknown && strings.TrimSpace(s.MissingReason) == "" {
		return bad("NA 需附未記錄原因")
	}
	return nil
}
