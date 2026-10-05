package repository

import (
	"coast-monitoring/internal/service"
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"strings"
)

// Public queries deliberately do not select participants, comments or snapshots.
func (r ReefDataRepository) PublicSites(ctx context.Context, f service.PublicReefFilter) ([]json.RawMessage, error) {
	if f.BBox == nil {
		f.BBox = []float64{}
	}
	rows, err := r.db.Query(ctx, `SELECT jsonb_build_object('id',p.id,'slug',p.id::text,'name_zh',p.name_zh,'name_en',p.name_en,'region',p.region,'county',p.county,'location',p.location,'latitude',p.latitude,'longitude',p.longitude,'event_count',count(DISTINCT e.id),'survey_count',count(DISTINCT s.id),'first_survey_date',min(e.survey_date),'latest_survey_date',max(e.survey_date),'data_updated_at',max(e.updated_at))
 FROM site p JOIN survey s ON s.site_id=p.id JOIN event e ON e.survey_id=s.id
 WHERE p.is_active AND e.publication_status='published' AND extract(year FROM e.survey_date) BETWEEN $1 AND $2 AND ($3::numeric IS NULL OR e.depth_m=$3)
 AND ($4='' OR EXISTS(SELECT 1 FROM transect t WHERE t.event_id=e.event_id AND t.method::text=$4)) AND (cardinality($5::float8[])=0 OR (p.longitude BETWEEN ($5::float8[])[1] AND ($5::float8[])[3] AND p.latitude BETWEEN ($5::float8[])[2] AND ($5::float8[])[4])) GROUP BY p.id ORDER BY p.name_zh LIMIT 1000`, f.FromYear, f.ToYear, f.Depth, f.Method, f.BBox)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []json.RawMessage{}
	for rows.Next() {
		var raw json.RawMessage
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		out = append(out, raw)
	}
	return out, rows.Err()
}

func (r ReefDataRepository) PublicSeries(ctx context.Context, siteID int, f service.PublicReefFilter) ([]service.PublicReefSeries, error) {
	// Each row retains event identity; no grouping across distinct observations.
	rows, err := r.db.Query(ctx, `WITH summaries AS (
 SELECT transect_id,CASE WHEN code='live_coral' THEN 'live_coral' ELSE 'substrate' END AS chart,code AS key,code AS name,''::text AS size_class,'%'::text AS unit,total::float8,mean::float8,coverage_percent::float8 AS value,sd::float8,se::float8,n FROM substrate_summary
 UNION ALL SELECT b.transect_id,'taxa',x.id::text,x.name_zh,COALESCE(x.size_class,''),CASE WHEN x.taxon_group='rare' THEN '隻／子樣區' ELSE '隻／100 m²' END,b.total::float8,b.mean::float8,b.mean::float8,b.sd::float8,b.se::float8,b.n FROM belt_summary b JOIN taxon x ON x.id=b.taxon_id
 UNION ALL SELECT i.transect_id,'impact',x.id::text,x.name_zh,'',CASE WHEN x.value_type='percent' THEN '%' WHEN x.has_raw_count OR x.value_type='count' THEN '原始件數' ELSE '歷史分級' END,i.total::float8,i.mean::float8,i.mean::float8,i.sd::float8,i.se::float8,i.n FROM impact_summary i JOIN impact_type x ON x.id=i.impact_type_id
 UNION ALL SELECT i.transect_id,'impact',x.id::text||':level',x.name_zh,'','系統衍生分級',NULL::float8,i.level_mean::float8,i.level_mean::float8,i.level_sd::float8,i.level_se::float8,i.n FROM impact_summary i JOIN impact_type x ON x.id=i.impact_type_id WHERE x.has_raw_count
 UNION ALL SELECT transect_id,'bleaching',code,code,'','白化點數',total::float8,mean::float8,mean::float8,sd::float8,se::float8,n FROM substrate_bleaching_summary
 ) SELECT p.id,p.name_zh,e.event_id,e.survey_date::text,e.event_time,e.depth_m::float8,a.chart,a.key,a.name,a.size_class,t.fish_size_mode,a.unit,a.total,a.mean,a.value,a.sd,a.se,a.n,e.updated_at::text
 FROM summaries a JOIN transect t ON t.id=a.transect_id JOIN event e ON e.event_id=t.event_id JOIN survey s ON s.id=e.survey_id JOIN site p ON p.id=s.site_id
 WHERE p.id=$1 AND p.is_active AND e.publication_status='published' AND extract(year FROM e.survey_date) BETWEEN $2 AND $3 AND ($4::numeric IS NULL OR e.depth_m=$4) AND ($5='' OR e.event_id=$5)
 AND a.chart=ANY($6::text[]) AND (a.chart<>'taxa' OR cardinality($7::int[])=0 OR a.key=ANY(SELECT v::text FROM unnest($7::int[]) v))
 AND (cardinality($8::text[])=0 OR a.key=ANY($8::text[]))
 AND NOT EXISTS (SELECT 1 FROM public_content c WHERE c.kind='chart' AND c.publication_status='published' AND c.site_id=p.id AND c.data->>'chart'=a.chart AND (
 NOT COALESCE((c.data->>'is_visible')::boolean,false)
 OR extract(year FROM e.survey_date)<COALESCE((c.data->>'year_from')::int,1900)
 OR extract(year FROM e.survey_date)>COALESCE((c.data->>'year_to')::int,2100)
 OR (jsonb_array_length(COALESCE(c.data->'depths_m','[]'))>0 AND NOT (c.data->'depths_m' @> jsonb_build_array(e.depth_m)))
 OR (jsonb_array_length(COALESCE(c.data->'category_keys','[]'))>0 AND NOT (c.data->'category_keys' ? a.key)))) ORDER BY e.survey_date,e.event_time,e.depth_m,e.event_id,a.chart,a.key`, siteID, f.FromYear, f.ToYear, f.Depth, f.EventID, f.Charts, f.TaxonIDs, f.CategoryKeys)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []service.PublicReefSeries{}
	for rows.Next() {
		var a service.PublicReefSeries
		if err = rows.Scan(&a.SiteID, &a.SiteName, &a.EventID, &a.Date, &a.Time, &a.Depth, &a.Chart, &a.Key, &a.Name, &a.Size, &a.Mode, &a.Unit, &a.Total, &a.Mean, &a.Value, &a.SD, &a.SE, &a.N, &a.UpdatedAt); err != nil {
			return nil, err
		}
		a.NormalizeUnit()
		a.Status = "ok"
		if a.N == 0 {
			a.Status = "not_calculable"
			a.MissingReason = "無有效觀測"
		} else if a.N < 2 {
			a.MissingReason = "有效段數不足，SD／SE 無法計算"
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
func (r ReefDataRepository) SetPublication(ctx context.Context, id int, status string, actor string) error {
	if status != "draft" && status != "published" && status != "archived" {
		return fmt.Errorf("%w: 無效發布狀態", service.ErrValidation)
	}
	exec := r.db
	var ownTx pgx.Tx
	if starter, ok := r.db.(txStarter); ok {
		tx, err := starter.Begin(ctx)
		if err != nil {
			return err
		}
		ownTx = tx
		exec = tx
		defer tx.Rollback(ctx)
	}
	var eventID string
	if err := exec.QueryRow(ctx, `SELECT event_id FROM event WHERE id=$1 FOR UPDATE`, id).Scan(&eventID); err != nil {
		return translateError(err)
	}
	if status == "published" {
		if _, err := exec.Exec(ctx, `SELECT id FROM transect WHERE event_id=$1 ORDER BY id FOR UPDATE`, eventID); err != nil {
			return err
		}
		if err := validateStoredEvent(ctx, exec, eventID); err != nil {
			return err
		}
	}
	tag, err := exec.Exec(ctx, `WITH previous AS (SELECT id,event_id,publication_status FROM event WHERE id=$1 FOR UPDATE), changed AS (UPDATE event e SET publication_status=$2,updated_at=now() FROM previous p WHERE e.id=p.id RETURNING e.id) INSERT INTO reef_publication_audit(event_id,old_status,new_status,actor_id) SELECT p.id,p.publication_status,$2,$3::uuid FROM previous p JOIN changed c ON c.id=p.id`, id, status, actor)
	if err = requireRowsAffected(tag, err); err != nil {
		return err
	}
	if ownTx != nil {
		return ownTx.Commit(ctx)
	}
	return nil
}

func validateStoredEvent(ctx context.Context, db DBTX, eventID string) error {
	var issues []string
	if err := db.QueryRow(ctx, `SELECT reef_event_completeness($1)`, eventID).Scan(&issues); err != nil {
		return err
	}
	if len(issues) > 0 {
		return fmt.Errorf("%w: %s", service.ErrValidation, strings.Join(issues, "；"))
	}
	return nil
}
