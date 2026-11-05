package service

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type PublicReefFilter struct {
	GroupID      int       `json:"group_id"`
	BBox         []float64 `json:"bbox"`
	Method       string    `json:"method"`
	CategoryKeys []string  `json:"category_keys"`
	FromYear     int       `json:"from_year"`
	ToYear       int       `json:"to_year"`
	Depth        *float64  `json:"depth_m"`
	EventID      string    `json:"event_id"`
	Charts       []string  `json:"charts"`
	TaxonIDs     []int     `json:"taxon_ids"`
	EventIDs     []string  `json:"event_ids"`
	Date         string    `json:"survey_date"`
	TaxonGroup   string    `json:"taxon_group"`
	Unit         string    `json:"unit"`
	SizeClass    string    `json:"size_class"`
	FishSizeMode string    `json:"fish_size_mode"`
}

func ParsePublicReefFilter(q url.Values) (PublicReefFilter, error) {
	f := PublicReefFilter{FromYear: 1900, ToYear: 2100, TaxonIDs: []int{}, Charts: []string{"substrate", "taxa", "live_coral", "impact", "bleaching"}}
	bad := func() (PublicReefFilter, error) {
		return f, fmt.Errorf("%w: 無效的圖表篩選條件", ErrValidation)
	}
	for key, dst := range map[string]*int{"from_year": &f.FromYear, "to_year": &f.ToYear} {
		if s := q.Get(key); s != "" {
			n, e := strconv.Atoi(s)
			if e != nil || n < 1900 || n > 2100 {
				return bad()
			}
			*dst = n
		}
	}
	if f.FromYear > f.ToYear {
		return bad()
	}
	if s := q.Get("depth_m"); s != "" {
		n, e := strconv.ParseFloat(s, 64)
		if e != nil || !finite(n) || n <= 0 || n > 100 {
			return bad()
		}
		f.Depth = &n
	}
	if s := q.Get("charts"); s != "" {
		f.Charts = strings.Split(s, ",")
		for _, c := range f.Charts {
			if c != "substrate" && c != "taxa" && c != "live_coral" && c != "impact" && c != "bleaching" {
				return bad()
			}
		}
	}
	if s := q.Get("taxon_ids"); s != "" {
		for _, v := range strings.Split(s, ",") {
			n, e := strconv.Atoi(v)
			if e != nil || n <= 0 {
				return bad()
			}
			f.TaxonIDs = append(f.TaxonIDs, n)
		}
		if len(f.TaxonIDs) > 100 {
			return bad()
		}
	}
	if raw := q.Get("group_id"); raw != "" {
		id, err := strconv.Atoi(raw)
		if err != nil || id <= 0 {
			return bad()
		}
		f.GroupID = id
	}
	f.EventID = q.Get("event_id")
	f.EventIDs = []string{}
	if s := q.Get("event_ids"); s != "" {
		f.EventIDs = strings.Split(s, ",")
		if len(f.EventIDs) > 500 {
			return bad()
		}
		for _, id := range f.EventIDs {
			if strings.TrimSpace(id) == "" || len(id) > 300 {
				return bad()
			}
		}
	}
	f.Date = q.Get("survey_date")
	if _, err := time.Parse("2006-01-02", f.Date); f.Date != "" && err != nil {
		return bad()
	}
	f.TaxonGroup = q.Get("taxon_group")
	if f.TaxonGroup != "" && f.TaxonGroup != "fish" && f.TaxonGroup != "invert" && f.TaxonGroup != "rare" {
		return bad()
	}
	f.Unit = q.Get("unit")
	if f.Unit != "" && !ValidPublicUnit(f.Unit) {
		return bad()
	}
	f.SizeClass = q.Get("size_class")
	if len(f.SizeClass) > 100 {
		return bad()
	}
	f.FishSizeMode = q.Get("fish_size_mode")
	if f.FishSizeMode != "" && f.FishSizeMode != "split" && f.FishSizeMode != "combined" {
		return bad()
	}
	if s := q.Get("bbox"); s != "" {
		parts := strings.Split(s, ",")
		if len(parts) != 4 {
			return bad()
		}
		for _, v := range parts {
			n, e := strconv.ParseFloat(v, 64)
			if e != nil || !finite(n) {
				return bad()
			}
			f.BBox = append(f.BBox, n)
		}
		if f.BBox[0] < -180 || f.BBox[2] > 180 || f.BBox[1] < -90 || f.BBox[3] > 90 || f.BBox[0] > f.BBox[2] || f.BBox[1] > f.BBox[3] {
			return bad()
		}
	}
	f.Method = q.Get("method")
	if f.Method != "" && f.Method != "line" && f.Method != "belt_fish" && f.Method != "belt_invert" {
		return bad()
	}
	f.CategoryKeys = []string{}
	if s := q.Get("category_keys"); s != "" {
		f.CategoryKeys = strings.Split(s, ",")
		if len(f.CategoryKeys) > 100 {
			return bad()
		}
	}
	return f, nil
}

type PublicReefSeries struct {
	TaxonID         *int     `json:"taxon_id"`
	TaxonGroup      string   `json:"taxon_group"`
	SiteID          int      `json:"site_id"`
	SiteName        string   `json:"site_name"`
	EventID         string   `json:"event_id"`
	Date            string   `json:"survey_date"`
	Time            *string  `json:"event_time"`
	Depth           float64  `json:"depth_m"`
	Chart           string   `json:"chart"`
	Key             string   `json:"key"`
	Name            string   `json:"name"`
	Size            *string  `json:"size_class"`
	Mode            *string  `json:"fish_size_mode"`
	Unit            string   `json:"unit"`
	UnitLabel       string   `json:"unit_label"`
	DensityPer100M2 *float64 `json:"density_per_100m2"`
	Total           *float64 `json:"total"`
	Mean            *float64 `json:"mean"`
	Value           *float64 `json:"value"`
	SD              *float64 `json:"sd"`
	SE              *float64 `json:"se"`
	N               int      `json:"n"`
	Status          string   `json:"calculation_status"`
	MissingReason   string   `json:"missing_reason"`
	UpdatedAt       string   `json:"data_updated_at"`
}

type PublicContent struct {
	EntryCode      string          `json:"entry_code,omitempty"`
	EntryConfigID  *int            `json:"entry_config_id,omitempty"`
	RelatedSiteIDs []int           `json:"related_site_ids,omitempty"`
	SiteID         *int            `json:"site_id"`
	ID             int             `json:"id"`
	Kind           string          `json:"kind"`
	Status         string          `json:"publication_status"`
	Data           json.RawMessage `json:"data"`
	UpdatedAt      string          `json:"updated_at"`
}

func ValidPublicUnit(s string) bool {
	switch s {
	case "percent", "individuals_per_100m2", "individuals_per_subsample", "count", "historical_level", "derived_level", "bleached_points":
		return true
	}
	return false
}

// Keep display text separate from stable machine-readable units.
func (s *PublicReefSeries) NormalizeUnit() {
	s.UnitLabel = s.Unit
	labels := map[string]string{"percent": "%", "individuals_per_100m2": "隻／100 m²", "individuals_per_subsample": "隻／子樣區", "count": "原始件數", "historical_level": "歷史分級", "derived_level": "系統衍生分級", "bleached_points": "白化點數"}
	if label, ok := labels[s.Unit]; ok {
		s.UnitLabel = label
	}
	if s.Unit == "individuals_per_100m2" {
		s.DensityPer100M2 = s.Mean
	}
	switch s.Unit {
	case "%":
		s.Unit = "percent"
	case "隻／100 m²":
		s.Unit = "individuals_per_100m2"
		s.DensityPer100M2 = s.Mean
	case "隻／子樣區":
		s.Unit = "individuals_per_subsample"
	case "原始件數":
		s.Unit = "count"
	case "歷史分級":
		s.Unit = "historical_level"
	case "系統衍生分級":
		s.Unit = "derived_level"
	case "白化點數":
		s.Unit = "bleached_points"
	}
}
