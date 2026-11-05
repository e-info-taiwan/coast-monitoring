package repository

import (
	"coast-monitoring/internal/service"
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"strings"
	"time"
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
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	// Scope the SQL deadline to this read transaction; do not limit admin imports.
	if starter, ok := r.db.(txStarter); ok {
		tx, err := starter.Begin(ctx)
		if err != nil {
			return nil, err
		}
		defer func() {
			cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
			defer stop()
			_ = tx.Rollback(cleanup)
		}()
		r.db = tx
		if _, err = tx.Exec(ctx, `SET LOCAL statement_timeout='18s'`); err != nil {
			return nil, err
		}
	}
	// Each row retains event identity; no grouping across distinct observations.
	if f.EventIDs == nil {
		f.EventIDs = []string{}
	}
	settings, err := r.chartSettings(ctx, siteID, false)
	if err != nil {
		return nil, err
	}
	effective := service.EffectiveChartSettings(siteID, settings)
	if f.GroupID > 0 {
		var from, to *int
		if err := r.db.QueryRow(ctx, `SELECT g.year_from,g.year_to FROM shared_chart_group g WHERE g.id=$1 AND g.publication_status='published' AND EXISTS(SELECT 1 FROM shared_chart_member m WHERE m.group_id=g.id AND m.site_id=$2 AND ($3::numeric IS NULL OR m.depth_m=$3))`, f.GroupID, siteID, f.Depth).Scan(&from, &to); err != nil {
			return nil, translateError(err)
		}
		if from != nil && *from > f.FromYear {
			f.FromYear = *from
		}
		if to != nil && *to < f.ToYear {
			f.ToYear = *to
		}
		includeLive := false
		for _, chart := range f.Charts {
			includeLive = includeLive || chart == "live_coral"
		}
		if !includeLive {
			return []service.PublicReefSeries{}, nil
		}
		f.Charts = []string{"live_coral"}
		effective = nil
	}
	rows, err := r.db.Query(ctx, `WITH selected AS MATERIALIZED (
 SELECT p.id site_id,p.name_zh site_name,e.event_id,e.survey_date,e.event_time,e.depth_m,e.updated_at,t.id transect_id,t.fish_size_mode
 FROM site p JOIN survey s ON s.site_id=p.id JOIN event e ON e.survey_id=s.id JOIN transect t ON t.event_id=e.event_id
 WHERE p.id=$1 AND p.is_active AND e.publication_status='published'
 AND e.survey_date>=make_date($2,1,1) AND e.survey_date<make_date($3+1,1,1)
 AND ($4::numeric IS NULL OR e.depth_m=$4) AND ($5='' OR e.event_id=$5)
 AND (cardinality($9::text[])=0 OR e.event_id=ANY($9::text[]))
 AND ($10='' OR e.survey_date=NULLIF($10,'')::date)
 AND ($14='' OR t.method::text=$14)
 AND ($16::bigint=0 OR EXISTS(SELECT 1 FROM shared_chart_member m WHERE m.group_id=$16 AND m.site_id=p.id AND m.depth_m=e.depth_m))
 ) SELECT e.site_id,e.site_name,e.event_id,e.survey_date::text,
 CASE WHEN e.event_time IN ('','na') THEN NULL ELSE replace(e.event_time,'-',':') END,e.depth_m::float8,
 a.chart,a.key,a.name,a.size_class,CASE WHEN a.chart='taxa' AND a.taxon_group='fish' THEN e.fish_size_mode END,
 a.unit,a.total,a.mean,a.value,a.sd,a.se,a.n,to_char(e.updated_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),a.taxon_id,a.taxon_group
 FROM selected e CROSS JOIN LATERAL reef_transect_series(e.transect_id,$6::text[]) a
 WHERE (cardinality($7::int[])=0 OR a.taxon_id=ANY($7::int[]))
 AND (cardinality($8::text[])=0 OR a.key=ANY($8::text[]))
 AND ($11='' OR a.taxon_group=$11) AND ($12='' OR a.size_class=$12)
 AND ($13='' OR (a.chart='taxa' AND a.taxon_group='fish' AND e.fish_size_mode=$13))
 AND ($15='' OR a.unit=$15)
 ORDER BY e.survey_date,e.event_time,e.depth_m,e.event_id,a.chart,a.key,a.unit`, siteID, f.FromYear, f.ToYear, f.Depth, f.EventID, f.Charts, f.TaxonIDs, f.CategoryKeys, f.EventIDs, f.Date, f.TaxonGroup, f.SizeClass, f.FishSizeMode, f.Method, f.Unit, f.GroupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []service.PublicReefSeries{}
	for rows.Next() {
		var a service.PublicReefSeries
		if err = rows.Scan(&a.SiteID, &a.SiteName, &a.EventID, &a.Date, &a.Time, &a.Depth, &a.Chart, &a.Key, &a.Name, &a.Size, &a.Mode, &a.Unit, &a.Total, &a.Mean, &a.Value, &a.SD, &a.SE, &a.N, &a.UpdatedAt, &a.TaxonID, &a.TaxonGroup); err != nil {
			return nil, err
		}
		a.NormalizeUnit()
		if f.Unit != "" && a.Unit != f.Unit {
			continue
		}
		allowed := true
		for _, setting := range effective {
			if setting.ChartKey == service.SeriesChartKey(a) {
				allowed = service.ChartAllows(setting, a)
				break
			}
		}
		if !allowed {
			continue
		}
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
