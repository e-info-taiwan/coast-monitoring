package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"coast-monitoring/internal/service"
	"github.com/jackc/pgx/v5"
)

func reefTransaction(ctx context.Context, db DBTX, fn func(DBTX) error) error {
	if starter, ok := db.(txStarter); ok {
		tx, err := starter.Begin(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)
		if err = fn(tx); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	return fn(db)
}
func jsonRows[T any](ctx context.Context, db DBTX, q string, args ...any) ([]T, error) {
	rows, err := db.Query(ctx, q, args...)
	if err != nil {
		return nil, translateError(err)
	}
	defer rows.Close()
	out := []T{}
	for rows.Next() {
		var raw []byte
		var v T
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func displayAudit(ctx context.Context, db DBTX, entity string, id int, actor string, before, after any) error {
	b, err := json.Marshal(before)
	if err != nil {
		return err
	}
	a, err := json.Marshal(after)
	if err != nil {
		return err
	}
	_, err = db.Exec(ctx, `INSERT INTO reef_site_display_audit(entity,entity_id,actor_id,before_data,after_data) VALUES($1,$2,$3::uuid,$4::jsonb,$5::jsonb)`, entity, id, actor, b, a)
	return err
}
func (r ReefDataRepository) chartSettings(ctx context.Context, siteID int, admin bool) ([]service.ChartSetting, error) {
	return jsonRows[service.ChartSetting](ctx, r.db, `SELECT to_jsonb(s)||jsonb_build_object('version',updated_at::text) FROM site_chart_setting s WHERE site_id=$1 AND (publication_status='published' OR ($2 AND publication_status='draft')) ORDER BY id`, siteID, admin)
}
func (r ReefDataRepository) SiteDisplay(ctx context.Context, siteID int, admin bool) (service.SiteDisplay, error) {
	out := service.SiteDisplay{SiteID: siteID, Reports: []service.PublicContent{}, Annotations: []service.PublicContent{}, Groups: []service.SharedChartGroup{}}
	var exists bool
	if err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM site WHERE id=$1 AND ($2 OR is_active))`, siteID, admin).Scan(&exists); err != nil {
		return out, err
	}
	if !exists {
		return out, service.ErrNotFound
	}
	settings, err := r.chartSettings(ctx, siteID, admin)
	if err != nil {
		return out, err
	}
	out.EffectiveSettings = service.EffectiveChartSettings(siteID, settings)
	if admin {
		out.DraftSettings = []service.ChartSetting{}
		for _, s := range settings {
			if s.Status == "draft" {
				out.DraftSettings = append(out.DraftSettings, s)
			}
		}
	}
	content, err := jsonRows[service.PublicContent](ctx, r.db, `SELECT to_jsonb(c) FROM public_content c WHERE site_id=$1 AND kind IN ('profile','annotation') AND publication_status='published' ORDER BY updated_at DESC,id DESC`, siteID)
	if err != nil {
		return out, err
	}
	for _, c := range content {
		if c.Kind == "profile" && out.Profile == nil {
			v := c
			out.Profile = &v
		} else if c.Kind == "annotation" {
			out.Annotations = append(out.Annotations, c)
		}
	}
	out.Reports, err = jsonRows[service.PublicContent](ctx, r.db, `SELECT to_jsonb(c)-'source_snapshot' FROM public_content c JOIN article_site a ON a.article_id=c.id WHERE a.site_id=$1 AND c.kind='article' AND c.data->>'category'='observation_report' AND ($2 OR (a.is_visible AND c.publication_status='published')) ORDER BY NULLIF(c.data->>'date','')::date DESC NULLS LAST,c.updated_at DESC,c.id DESC`, siteID, admin)
	if err != nil {
		return out, err
	}
	// The latest report is always selected from the public visible set, even in admin.
	for _, c := range out.Reports {
		if c.Status != "published" {
			continue
		}
		var visible bool
		if err = r.db.QueryRow(ctx, `SELECT is_visible FROM article_site WHERE article_id=$1 AND site_id=$2`, c.ID, siteID).Scan(&visible); err != nil {
			return out, err
		}
		if visible {
			v := c
			out.LatestReport = &v
			break
		}
	}
	if admin {
		out.ReportLinks, err = jsonRows[service.ReportLink](ctx, r.db, `SELECT to_jsonb(a)||jsonb_build_object('version',updated_at::text) FROM article_site a WHERE site_id=$1 ORDER BY sort_order,article_id`, siteID)
		if err != nil {
			return out, err
		}
	}
	out.Groups, err = jsonRows[service.SharedChartGroup](ctx, r.db, `SELECT to_jsonb(g)||jsonb_build_object('version',g.updated_at::text,'members',(SELECT jsonb_agg(to_jsonb(m) ORDER BY m.sort_order,m.site_id,m.depth_m) FROM shared_chart_member m WHERE m.group_id=g.id)) FROM shared_chart_group g WHERE ($2 OR g.publication_status='published') AND g.publication_status<>'archived' AND EXISTS(SELECT 1 FROM shared_chart_member WHERE group_id=g.id AND site_id=$1) ORDER BY g.id`, siteID, admin)
	return out, err
}

func (r ReefDataRepository) SaveChartSetting(ctx context.Context, in service.ChartSetting, actor string) (service.ChartSetting, error) {
	if err := in.Validate(); err != nil {
		return in, err
	}
	err := reefTransaction(ctx, r.db, func(db DBTX) error {
		if _, err := db.Exec(ctx, `SELECT pg_advisory_xact_lock(76101,$1)`, in.SiteID); err != nil {
			return err
		}
		var before *service.ChartSetting
		allBefore, err := jsonRows[service.ChartSetting](ctx, db, `SELECT to_jsonb(s) FROM site_chart_setting s WHERE site_id=$1 AND chart_key=$2 AND publication_status<>'archived'`, in.SiteID, in.ChartKey)
		if err != nil {
			return err
		}
		if in.ID == 0 && len(allBefore) > 0 {
			return service.ErrConflict
		}
		if in.ID > 0 {
			rows, err := jsonRows[service.ChartSetting](ctx, db, `SELECT to_jsonb(s)||jsonb_build_object('version',updated_at::text) FROM site_chart_setting s WHERE id=$1 AND site_id=$2 AND chart_key=$3 FOR UPDATE`, in.ID, in.SiteID, in.ChartKey)
			if err != nil {
				return err
			}
			if len(rows) != 1 || rows[0].Version != in.Version || rows[0].Status == "archived" {
				return service.ErrConflict
			}
			before = &rows[0]
		}
		for _, id := range in.TaxonIDs {
			var valid bool
			if err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM taxon WHERE id=$1 AND (CASE WHEN $2='rare_history' THEN taxon_group='rare' ELSE taxon_group IN ('fish','invert') END))`, id, in.ChartKey).Scan(&valid); err != nil {
				return err
			}
			if !valid {
				return service.ErrInvalidReference
			}
		}
		if in.Restore {
			_, err := db.Exec(ctx, `UPDATE site_chart_setting SET publication_status='archived',updated_at=clock_timestamp(),updated_by=$3::uuid WHERE site_id=$1 AND chart_key=$2 AND publication_status<>'archived'`, in.SiteID, in.ChartKey, actor)
			if err != nil {
				return err
			}
			return displayAudit(ctx, db, "chart_restore", in.SiteID, actor, allBefore, nil)
		}
		// Publishing replaces the old published row atomically; editing it as draft
		// creates a separate revision and never hides the live configuration.
		if in.Status == "published" {
			if _, err := db.Exec(ctx, `UPDATE site_chart_setting SET publication_status='archived',updated_at=clock_timestamp(),updated_by=$3::uuid WHERE site_id=$1 AND chart_key=$2 AND publication_status IN ('published','draft')`, in.SiteID, in.ChartKey, actor); err != nil {
				return err
			}
		}
		if before != nil && before.Status == "draft" && in.Status == "draft" {
			_, err := db.Exec(ctx, `UPDATE site_chart_setting SET publication_status='archived',updated_at=clock_timestamp() WHERE id=$1`, before.ID)
			if err != nil {
				return err
			}
		} else if before != nil && in.Status == "archived" {
			_, err := db.Exec(ctx, `UPDATE site_chart_setting SET publication_status='archived',updated_at=clock_timestamp(),updated_by=$2::uuid WHERE id=$1`, before.ID, actor)
			if err != nil {
				return err
			}
			return displayAudit(ctx, db, "chart", in.ID, actor, allBefore, in)
		}
		err = db.QueryRow(ctx, `INSERT INTO site_chart_setting(site_id,chart_key,is_visible,sort_order,year_from,year_to,default_depth_m,category_keys,taxon_ids,publication_status,updated_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11::uuid) RETURNING id,updated_at::text`, in.SiteID, in.ChartKey, in.Visible, in.SortOrder, in.YearFrom, in.YearTo, in.DefaultDepth, in.CategoryKeys, in.TaxonIDs, in.Status, actor).Scan(&in.ID, &in.Version)
		if err != nil {
			return translateError(err)
		}
		return displayAudit(ctx, db, "chart", in.ID, actor, allBefore, in)
	})
	return in, err
}

func (r ReefDataRepository) SaveReportLink(ctx context.Context, in service.ReportLink, actor string) error {
	if in.ArticleID <= 0 || in.SiteID <= 0 {
		return service.ErrValidation
	}
	return reefTransaction(ctx, r.db, func(db DBTX) error {
		if _, err := db.Exec(ctx, `SELECT pg_advisory_xact_lock(76102,$1)`, in.SiteID); err != nil {
			return err
		}
		rows, err := jsonRows[service.ReportLink](ctx, db, `SELECT to_jsonb(a)||jsonb_build_object('version',updated_at::text) FROM article_site a WHERE article_id=$1 AND site_id=$2 FOR UPDATE`, in.ArticleID, in.SiteID)
		if err != nil {
			return err
		}
		var before *service.ReportLink
		if len(rows) > 0 {
			before = &rows[0]
			if before.Version != in.Version {
				return service.ErrConflict
			}
		}
		if in.Remove {
			_, err = db.Exec(ctx, `DELETE FROM article_site WHERE article_id=$1 AND site_id=$2`, in.ArticleID, in.SiteID)
		} else {
			err = db.QueryRow(ctx, `INSERT INTO article_site(article_id,site_id,is_visible,sort_order,updated_by) VALUES($1,$2,$3,$4,$5::uuid) ON CONFLICT(article_id,site_id) DO UPDATE SET is_visible=EXCLUDED.is_visible,sort_order=EXCLUDED.sort_order,updated_at=clock_timestamp(),updated_by=EXCLUDED.updated_by RETURNING updated_at::text`, in.ArticleID, in.SiteID, in.Visible, in.SortOrder, actor).Scan(&in.Version)
		}
		if err != nil {
			return translateError(err)
		}
		return displayAudit(ctx, db, "report_link", in.ArticleID, actor, before, in)
	})
}
func (r ReefDataRepository) SaveSharedGroup(ctx context.Context, in service.SharedChartGroup, actor string) (service.SharedChartGroup, error) {
	if err := in.Validate(); err != nil {
		return in, err
	}
	err := reefTransaction(ctx, r.db, func(db DBTX) error {
		var before *service.SharedChartGroup
		if in.ID > 0 {
			rows, err := jsonRows[service.SharedChartGroup](ctx, db, `SELECT to_jsonb(g)||jsonb_build_object('version',updated_at::text,'members',(SELECT jsonb_agg(to_jsonb(m)) FROM shared_chart_member m WHERE m.group_id=g.id)) FROM shared_chart_group g WHERE id=$1 FOR UPDATE`, in.ID)
			if err != nil {
				return err
			}
			if len(rows) != 1 || rows[0].Version != in.Version {
				return service.ErrConflict
			}
			before = &rows[0]
			if _, err = db.Exec(ctx, `UPDATE shared_chart_group SET title=$2,year_from=$3,year_to=$4,publication_status=$5,updated_by=$6::uuid,updated_at=clock_timestamp() WHERE id=$1`, in.ID, in.Title, in.YearFrom, in.YearTo, in.Status, actor); err != nil {
				return err
			}
			if _, err = db.Exec(ctx, `DELETE FROM shared_chart_member WHERE group_id=$1`, in.ID); err != nil {
				return err
			}
		} else {
			if err := db.QueryRow(ctx, `INSERT INTO shared_chart_group(title,year_from,year_to,publication_status,updated_by) VALUES($1,$2,$3,$4,$5::uuid) RETURNING id`, in.Title, in.YearFrom, in.YearTo, in.Status, actor).Scan(&in.ID); err != nil {
				return err
			}
		}
		for _, m := range in.Members {
			if _, err := db.Exec(ctx, `INSERT INTO shared_chart_member(group_id,site_id,depth_m,sort_order) VALUES($1,$2,$3,$4)`, in.ID, m.SiteID, m.Depth, m.SortOrder); err != nil {
				return translateError(err)
			}
		}
		if err := db.QueryRow(ctx, `SELECT updated_at::text FROM shared_chart_group WHERE id=$1`, in.ID).Scan(&in.Version); err != nil {
			return err
		}
		old := []service.SharedChartMember{}
		if before != nil {
			old = before.Members
		}
		in.AffectedSiteIDs = service.AffectedSites(old, in.Members)
		return displayAudit(ctx, db, "shared_group", in.ID, actor, before, in)
	})
	return in, err
}

func (r ReefDataRepository) validateSiteIDs(ctx context.Context, ids []int) error {
	for _, id := range ids {
		var ok bool
		if err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM site WHERE id=$1 AND is_active)`, id).Scan(&ok); err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("%w: 樣點不存在或停用", service.ErrInvalidReference)
		}
	}
	return nil
}

// Compile-time assertions document the transaction capability used here.
var _ DBTX = (pgx.Tx)(nil)
