package service

import (
	"fmt"
	"sort"
)

var ChartKeys = []string{"substrate_depth", "taxa_density", "rare_history", "live_coral_history", "impact_history", "bleaching_history"}

type ChartSetting struct {
	ID           int      `json:"id"`
	SiteID       int      `json:"site_id"`
	ChartKey     string   `json:"chart_key"`
	Visible      *bool    `json:"is_visible"`
	SortOrder    *int     `json:"sort_order"`
	YearFrom     *int     `json:"year_from"`
	YearTo       *int     `json:"year_to"`
	DefaultDepth *float64 `json:"default_depth_m"`
	CategoryKeys []string `json:"category_keys"`
	TaxonIDs     []int    `json:"taxon_ids"`
	Status       string   `json:"publication_status"`
	Version      string   `json:"version"`
	Source       string   `json:"source,omitempty"`
	Restore      bool     `json:"restore,omitempty"`
	Confirm      bool     `json:"confirm,omitempty"`
}

func validYears(a, b *int) bool {
	return (a == nil || *a >= 1900 && *a <= 2100) && (b == nil || *b >= 1900 && *b <= 2100) && (a == nil || b == nil || *a <= *b)
}
func (c *ChartSetting) Validate() error {
	valid := false
	for _, k := range ChartKeys {
		valid = valid || k == c.ChartKey
	}
	if c.Status == "" {
		c.Status = "draft"
	}
	if !valid || c.SiteID <= 0 || !validYears(c.YearFrom, c.YearTo) || c.DefaultDepth != nil && (!finite(*c.DefaultDepth) || *c.DefaultDepth <= 0) {
		return fmt.Errorf("%w: 圖表設定無效", ErrValidation)
	}
	if c.Status != "draft" && c.Status != "published" && c.Status != "archived" {
		return fmt.Errorf("%w: 發布狀態無效", ErrValidation)
	}
	if (c.Restore || c.Status == "archived") && !c.Confirm {
		return fmt.Errorf("%w: 還原或封存需確認", ErrValidation)
	}
	seen := map[int]bool{}
	for _, id := range c.TaxonIDs {
		if id <= 0 || seen[id] {
			return fmt.Errorf("%w: 物種無效或重複", ErrValidation)
		}
		seen[id] = true
	}
	return nil
}

// Null fields inherit; an explicitly empty slice means no selected indicators.
// Depth is only an initial selection, never an implicit restriction on history.
func EffectiveChartSettings(siteID int, overrides []ChartSetting) []ChartSetting {
	out := make([]ChartSetting, 0, len(ChartKeys))
	for i, k := range ChartKeys {
		visible, order := true, i
		c := ChartSetting{SiteID: siteID, ChartKey: k, Visible: &visible, SortOrder: &order, Source: "default", Status: "published"}
		for _, v := range overrides {
			if v.ChartKey != k || v.Status != "published" {
				continue
			}
			c.ID = v.ID
			c.Version = v.Version
			c.Source = "site"
			if v.Visible != nil {
				c.Visible = v.Visible
			}
			if v.SortOrder != nil {
				c.SortOrder = v.SortOrder
			}
			c.YearFrom = v.YearFrom
			c.YearTo = v.YearTo
			c.DefaultDepth = v.DefaultDepth
			c.CategoryKeys = v.CategoryKeys
			c.TaxonIDs = v.TaxonIDs
		}
		out = append(out, c)
	}
	sort.SliceStable(out, func(i, j int) bool { return *out[i].SortOrder < *out[j].SortOrder })
	return out
}

func SeriesChartKey(row PublicReefSeries) string {
	switch row.Chart {
	case "substrate":
		return "substrate_depth"
	case "taxa":
		if row.TaxonGroup == "rare" {
			return "rare_history"
		}
		return "taxa_density"
	case "live_coral":
		return "live_coral_history"
	case "impact":
		return "impact_history"
	case "bleaching":
		return "bleaching_history"
	}
	return ""
}

func ChartAllows(c ChartSetting, row PublicReefSeries) bool {
	if c.Visible != nil && !*c.Visible {
		return false
	}
	var year int
	fmt.Sscanf(row.Date, "%d-", &year)
	if c.YearFrom != nil && year < *c.YearFrom || c.YearTo != nil && year > *c.YearTo {
		return false
	}
	if c.CategoryKeys != nil {
		found := false
		for _, k := range c.CategoryKeys {
			found = found || k == row.Key
		}
		if !found {
			return false
		}
	}
	if row.Chart == "taxa" && c.TaxonIDs != nil {
		found := false
		for _, id := range c.TaxonIDs {
			found = found || row.TaxonID != nil && id == *row.TaxonID
		}
		if !found {
			return false
		}
	}
	return true
}

type SharedChartMember struct {
	SiteID    int     `json:"site_id"`
	Depth     float64 `json:"depth_m"`
	SortOrder int     `json:"sort_order"`
}
type SharedChartGroup struct {
	ID              int                 `json:"id"`
	Title           string              `json:"title"`
	ChartKey        string              `json:"chart_key"`
	YearFrom        *int                `json:"year_from"`
	YearTo          *int                `json:"year_to"`
	Status          string              `json:"publication_status"`
	Members         []SharedChartMember `json:"members"`
	Version         string              `json:"version"`
	Confirm         bool                `json:"confirm"`
	AffectedSiteIDs []int               `json:"affected_site_ids"`
}

func (g *SharedChartGroup) Validate() error {
	if g.ChartKey == "" {
		g.ChartKey = "live_coral_history"
	}
	if g.Status == "" {
		g.Status = "draft"
	}
	if g.Title == "" || g.ChartKey != "live_coral_history" || !validYears(g.YearFrom, g.YearTo) || len(g.Members) == 0 {
		return fmt.Errorf("%w: 群組設定無效", ErrValidation)
	}
	if g.Status != "draft" && g.Status != "published" && g.Status != "archived" {
		return fmt.Errorf("%w: 發布狀態無效", ErrValidation)
	}
	if g.ID > 0 && !g.Confirm {
		return fmt.Errorf("%w: 共用群組異動需確認影響範圍", ErrValidation)
	}
	seen := map[string]bool{}
	for _, m := range g.Members {
		k := fmt.Sprintf("%d:%g", m.SiteID, m.Depth)
		if m.SiteID <= 0 || !finite(m.Depth) || m.Depth <= 0 || seen[k] {
			return fmt.Errorf("%w: 群組成員無效或重複", ErrValidation)
		}
		seen[k] = true
	}
	return nil
}
func AffectedSites(before, after []SharedChartMember) []int {
	set := map[int]bool{}
	for _, rows := range [][]SharedChartMember{before, after} {
		for _, m := range rows {
			set[m.SiteID] = true
		}
	}
	ids := []int{}
	for id := range set {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids
}

type ReportLink struct {
	ArticleID int    `json:"article_id"`
	SiteID    int    `json:"site_id"`
	Visible   bool   `json:"is_visible"`
	SortOrder int    `json:"sort_order"`
	Remove    bool   `json:"remove"`
	Version   string `json:"version"`
}
type SiteDisplay struct {
	SiteID            int                `json:"site_id"`
	Profile           *PublicContent     `json:"profile"`
	Reports           []PublicContent    `json:"reports"`
	LatestReport      *PublicContent     `json:"latest_report"`
	ReportLinks       []ReportLink       `json:"report_links,omitempty"`
	Annotations       []PublicContent    `json:"annotations"`
	EffectiveSettings []ChartSetting     `json:"effective_settings"`
	DraftSettings     []ChartSetting     `json:"draft_settings,omitempty"`
	Groups            []SharedChartGroup `json:"groups"`
}
