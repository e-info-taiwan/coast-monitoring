package service

import (
	"encoding/json"
	"fmt"
	"net/url"
	"time"
)

type PublicContentInput struct {
	Kind    string      `json:"kind"`
	Status  string      `json:"publication_status"`
	SiteID  *int        `json:"site_id"`
	Data    ContentData `json:"data"`
	Version string      `json:"version"`
	Confirm bool        `json:"confirm"`
}
type ContentData struct {
	ArticleLimit    *int              `json:"article_limit,omitempty"`
	Title           string            `json:"title"`
	Summary         string            `json:"summary,omitempty"`
	ImageURL        string            `json:"image_url,omitempty"`
	ExternalURL     string            `json:"external_url,omitempty"`
	Date            string            `json:"date,omitempty"`
	SortOrder       int               `json:"sort_order"`
	StartAt         string            `json:"start_at,omitempty"`
	EndAt           string            `json:"end_at,omitempty"`
	Location        string            `json:"location,omitempty"`
	Recruitment     string            `json:"recruitment_type,omitempty"`
	RegistrationURL string            `json:"registration_url,omitempty"`
	Status          string            `json:"status,omitempty"`
	County          string            `json:"county,omitempty"`
	Authority       string            `json:"authority,omitempty"`
	Area            *float64          `json:"area_m2,omitempty"`
	Investment      *float64          `json:"investment_ntd,omitempty"`
	SourceURL       string            `json:"source_url,omitempty"`
	Geometry        json.RawMessage   `json:"geometry,omitempty"`
	Timeline        []ContentTimeline `json:"timeline,omitempty"`
	Media           []ContentMedia    `json:"media,omitempty"`
	ReportURL       string            `json:"report_url,omitempty"`
	Chart           string            `json:"chart,omitempty"`
	SiteIDs         []int             `json:"site_ids,omitempty"`
	Depths          []float64         `json:"depths_m,omitempty"`
	Categories      []string          `json:"category_keys,omitempty"`
	YearFrom        *int              `json:"year_from,omitempty"`
	YearTo          *int              `json:"year_to,omitempty"`
	Visible         bool              `json:"is_visible"`
}
type ContentTimeline struct {
	Date        string `json:"date"`
	Precision   string `json:"date_precision"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
	SourceURL   string `json:"source_url"`
	Published   bool   `json:"is_published"`
	SortOrder   int    `json:"sort_order"`
}
type ContentMedia struct {
	URL       string `json:"url"`
	Alt       string `json:"alt"`
	Published bool   `json:"is_published"`
	Valid     bool   `json:"is_valid"`
	Licensed  bool   `json:"license_confirmed"`
}

func safePublicURL(s string) bool {
	if s == "" {
		return true
	}
	u, e := url.Parse(s)
	return e == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != "" && u.User == nil
}
func (c *PublicContentInput) Validate() error {
	bad := func(s string) error { return fmt.Errorf("%w: %s", ErrValidation, s) }
	if c.Status == "" {
		c.Status = "draft"
	}
	if c.Status != "draft" && c.Status != "published" && c.Status != "archived" {
		return bad("無效發布狀態")
	}
	if c.Status == "archived" && !c.Confirm {
		return bad("封存需二次確認")
	}
	if c.Data.Title == "" {
		return bad("請填標題")
	}
	for _, s := range []string{c.Data.ImageURL, c.Data.ExternalURL, c.Data.RegistrationURL, c.Data.SourceURL, c.Data.ReportURL} {
		if !safePublicURL(s) {
			return bad("網址需為 HTTP 或 HTTPS")
		}
	}
	switch c.Kind {
	case "settings":
		if c.Data.ArticleLimit == nil || *c.Data.ArticleLimit < 1 || *c.Data.ArticleLimit > 50 {
			return bad("首頁文章數量需介於 1–50")
		}
	case "article":
		if c.Data.ExternalURL == "" {
			return bad("文章需有外部網址")
		}
		if _, err := time.Parse("2006-01-02", c.Data.Date); err != nil {
			return bad("文章發布日期格式錯誤")
		}
	case "activity":
		a, e := time.Parse(time.RFC3339, c.Data.StartAt)
		b, e2 := time.Parse(time.RFC3339, c.Data.EndAt)
		if e != nil || e2 != nil || b.Before(a) {
			return bad("活動起訖時間無效")
		}
		if c.Data.Recruitment != "public" && c.Data.Recruitment != "internal" {
			return bad("需指定招募類型")
		}
		if c.Data.Status != "scheduled" && c.Data.Status != "open" && c.Data.Status != "closed" && c.Data.Status != "cancelled" && c.Data.Status != "completed" {
			return bad("無效活動狀態")
		}
		if c.Data.Status == "open" && c.Data.Recruitment == "public" && c.Data.RegistrationURL == "" {
			return bad("報名中活動需有報名網址")
		}
	case "development":
		if err := validateGeometry(c.Data.Geometry); err != nil {
			return err
		}
		for _, v := range []*float64{c.Data.Area, c.Data.Investment} {
			if v != nil && (!finite(*v) || *v < 0) {
				return bad("面積／金額需為非負數")
			}
		}
		for _, t := range c.Data.Timeline {
			if !safePublicURL(t.SourceURL) || t.Title == "" {
				return bad("時間軸標題或來源無效")
			}
			layout := map[string]string{"day": "2006-01-02", "month": "2006-01", "year": "2006"}[t.Precision]
			if t.Precision != "unknown" {
				if layout == "" {
					return bad("時間軸日期精度無效")
				}
				if _, err := time.Parse(layout, t.Date); err != nil {
					return bad("時間軸日期無效")
				}
			}
			if t.Published && t.SourceURL == "" {
				return bad("已發布時間軸需有查證來源")
			}
		}
		for _, m := range c.Data.Media {
			if !safePublicURL(m.URL) || m.URL == "" || m.Alt == "" {
				return bad("圖片需有有效網址與替代文字")
			}
			if m.Published && (!m.Valid || !m.Licensed) {
				return bad("圖片需確認有效性與授權才可發布")
			}
		}
	case "annotation":
		if c.SiteID == nil || c.Data.SourceURL == "" {
			return bad("樣點事件需有樣點與來源")
		}
		if _, err := time.Parse("2006-01-02", c.Data.Date); err != nil {
			return bad("事件日期無效")
		}
	case "profile":
		if c.SiteID == nil {
			return bad("需選擇樣點")
		}
	case "chart":
		if c.Data.Chart != "substrate" && c.Data.Chart != "taxa" && c.Data.Chart != "live_coral" && c.Data.Chart != "impact" && c.Data.Chart != "bleaching" {
			return bad("無效圖表類型")
		}
		if c.SiteID == nil && len(c.Data.SiteIDs) == 0 {
			return bad("需指定樣點或共用樣點群組")
		}
		if c.Data.YearFrom != nil && (*c.Data.YearFrom < 1900 || *c.Data.YearFrom > 2100) {
			return bad("年份無效")
		}
		if c.Data.YearTo != nil && (*c.Data.YearTo < 1900 || *c.Data.YearTo > 2100) {
			return bad("年份無效")
		}
		if c.Data.YearFrom != nil && c.Data.YearTo != nil && *c.Data.YearFrom > *c.Data.YearTo {
			return bad("年份順序無效")
		}
		for _, d := range c.Data.Depths {
			if !finite(d) || d <= 0 || d > 100 {
				return bad("深度無效")
			}
		}
	default:
		return bad("無效內容類型")
	}
	return nil
}
func validateGeometry(raw json.RawMessage) error {
	bad := func() error {
		return fmt.Errorf("%w: 幾何需為有效 Point／Polygon／MultiPolygon", ErrValidation)
	}
	var g struct {
		Type        string          `json:"type"`
		Coordinates json.RawMessage `json:"coordinates"`
	}
	if json.Unmarshal(raw, &g) != nil {
		return bad()
	}
	point := func(p []float64) bool {
		return len(p) == 2 && finite(p[0]) && finite(p[1]) && p[0] >= -180 && p[0] <= 180 && p[1] >= -90 && p[1] <= 90
	}
	polygon := func(p [][][]float64) bool {
		if len(p) == 0 {
			return false
		}
		for _, ring := range p {
			if len(ring) < 4 {
				return false
			}
			for _, v := range ring {
				if !point(v) {
					return false
				}
			}
			a, b := ring[0], ring[len(ring)-1]
			if a[0] != b[0] || a[1] != b[1] {
				return false
			}
		}
		return true
	}
	switch g.Type {
	case "Point":
		var p []float64
		if json.Unmarshal(g.Coordinates, &p) != nil || !point(p) {
			return bad()
		}
	case "Polygon":
		var p [][][]float64
		if json.Unmarshal(g.Coordinates, &p) != nil || !polygon(p) {
			return bad()
		}
	case "MultiPolygon":
		var p [][][][]float64
		if json.Unmarshal(g.Coordinates, &p) != nil || len(p) == 0 {
			return bad()
		}
		for _, v := range p {
			if !polygon(v) {
				return bad()
			}
		}
	default:
		return bad()
	}
	return nil
}
