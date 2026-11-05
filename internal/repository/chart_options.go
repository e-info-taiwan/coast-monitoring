package repository

import (
	"coast-monitoring/internal/service"
	"context"
	"sort"
)

func (r ReefDataRepository) chartDataOptions(ctx context.Context, siteID int) (map[string]service.ChartDataOptions, error) {
	rows, err := r.db.Query(ctx, `WITH selected AS MATERIALIZED (
 SELECT t.id,e.survey_date,e.depth_m FROM transect t JOIN event e ON e.event_id=t.event_id JOIN survey s ON s.id=e.survey_id WHERE s.site_id=$1 AND e.publication_status='published'
 ) SELECT DISTINCT CASE a.chart WHEN 'substrate' THEN 'substrate_depth' WHEN 'taxa' THEN CASE WHEN a.taxon_group='rare' THEN 'rare_history' ELSE 'taxa_density' END WHEN 'live_coral' THEN 'live_coral_history' WHEN 'impact' THEN 'impact_history' ELSE 'bleaching_history' END,
 a.key,a.name||CASE WHEN a.size_class IS NOT NULL THEN ' · '||a.size_class ELSE '' END,a.chart='taxa',extract(year FROM t.survey_date)::int,t.depth_m::float8
 FROM selected t CROSS JOIN LATERAL reef_transect_series(t.id,ARRAY['substrate','taxa','live_coral','impact','bleaching']) a ORDER BY 1,2,5,6`, siteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]service.ChartDataOptions{}
	years := map[string]map[int]bool{}
	depths := map[string]map[float64]bool{}
	keys := map[string]map[string]bool{}
	for _, k := range service.ChartKeys {
		out[k] = service.ChartDataOptions{Years: []int{}, Depths: []float64{}, Categories: []service.ChartChoice{}, Taxa: []service.ChartChoice{}}
		years[k] = map[int]bool{}
		depths[k] = map[float64]bool{}
		keys[k] = map[string]bool{}
	}
	for rows.Next() {
		var chart, key, label string
		var taxon bool
		var year int
		var depth float64
		if err = rows.Scan(&chart, &key, &label, &taxon, &year, &depth); err != nil {
			return nil, err
		}
		c := out[chart]
		years[chart][year] = true
		depths[chart][depth] = true
		if !keys[chart][key] {
			choice := service.ChartChoice{Value: key, Label: label}
			if taxon {
				c.Taxa = append(c.Taxa, choice)
			} else {
				c.Categories = append(c.Categories, choice)
			}
			keys[chart][key] = true
		}
		out[chart] = c
	}
	for k, c := range out {
		for y := range years[k] {
			c.Years = append(c.Years, y)
		}
		for d := range depths[k] {
			c.Depths = append(c.Depths, d)
		}
		sort.Ints(c.Years)
		sort.Float64s(c.Depths)
		out[k] = c
	}
	return out, rows.Err()
}
