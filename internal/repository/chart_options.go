package repository

import (
	"coast-monitoring/internal/service"
	"context"
	"sort"
)

func (r ReefDataRepository) chartDataOptions(ctx context.Context, siteID int) (map[string]service.ChartDataOptions, error) {
	rows, err := r.db.Query(ctx, `WITH indicators AS (
 SELECT transect_id,CASE WHEN a.code='live_coral' THEN 'live_coral_history' ELSE 'substrate_depth' END chart,a.code key,COALESCE(s.name_zh,CASE WHEN a.code='live_coral' THEN '活珊瑚覆蓋率' ELSE a.code END) label,false taxon FROM substrate_summary a LEFT JOIN substrate_type s ON s.code=a.code
 UNION ALL SELECT a.transect_id,CASE WHEN x.taxon_group='rare' THEN 'rare_history' ELSE 'taxa_density' END,x.id::text,COALESCE(NULLIF(x.name_zh,''),x.name_en)||CASE WHEN COALESCE(x.size_class,'')<>'' THEN ' · '||x.size_class ELSE '' END,true FROM belt_summary a JOIN taxon x ON x.id=a.taxon_id
 UNION ALL SELECT a.transect_id,'impact_history',x.id::text,x.name_zh,false FROM impact_summary a JOIN impact_type x ON x.id=a.impact_type_id
 UNION ALL SELECT a.transect_id,'impact_history',x.id::text||':level',x.name_zh||'（分級）',false FROM impact_summary a JOIN impact_type x ON x.id=a.impact_type_id WHERE a.source_unit IN ('count','level') OR (a.source_unit IS NULL AND x.has_raw_count)
 UNION ALL SELECT a.transect_id,'bleaching_history',a.code,COALESCE(s.name_zh,a.code),false FROM substrate_bleaching_summary a LEFT JOIN substrate_type s ON s.code=a.code
 ) SELECT DISTINCT i.chart,i.key,i.label,i.taxon,extract(year FROM e.survey_date)::int,e.depth_m::float8
 FROM indicators i JOIN transect t ON t.id=i.transect_id JOIN event e ON e.event_id=t.event_id JOIN survey s ON s.id=e.survey_id
 WHERE s.site_id=$1 AND e.publication_status='published' ORDER BY 1,2,5,6`, siteID)
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
