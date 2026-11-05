package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"coast-monitoring/internal/service"
	"github.com/jackc/pgx/v5"
)

func (r ReefDataRepository) ListContent(ctx context.Context, kind string, public bool) ([]service.PublicContent, error) {
	rows, err := r.listContentRows(ctx, kind, public)
	if err != nil {
		return nil, err
	}
	if public {
		return rows, nil
	}
	for i := range rows {
		c := &rows[i]
		if c.Kind == "article" {
			if err = r.db.QueryRow(ctx, `SELECT ARRAY(SELECT site_id FROM article_site WHERE article_id=$1 ORDER BY site_id)`, c.ID).Scan(&c.RelatedSiteIDs); err != nil {
				return nil, err
			}
		}
		if c.Kind == "activity" {
			if err = r.db.QueryRow(ctx, `SELECT (SELECT id FROM reef_check_activity WHERE activity_content_id=$1)`, c.ID).Scan(&c.EntryConfigID); err != nil {
				return nil, err
			}
		}
	}
	return rows, nil
}

func (r ReefDataRepository) SaveContent(ctx context.Context, id int, in service.PublicContentInput, actor string) (service.PublicContent, error) {
	var result service.PublicContent
	err := reefTransaction(ctx, r.db, func(db DBTX) error {
		repo := ReefDataRepository{db: db}
		if in.Kind == "chart" {
			return fmt.Errorf("%w: 請在調查樣點的圖表顯示區修改設定", service.ErrValidation)
		}
		if id > 0 {
			var raw []byte
			var version string
			if err := db.QueryRow(ctx, `SELECT data,updated_at::text FROM public_content WHERE id=$1 AND kind=$2 FOR UPDATE`, id, in.Kind).Scan(&raw, &version); err != nil {
				return translateError(err)
			}
			if version != in.Version {
				return service.ErrConflict
			}
			if in.Kind == "settings" {
				var previous service.ContentData
				if err := json.Unmarshal(raw, &previous); err != nil {
					return err
				}
				if in.Data.ArticleLimit == nil {
					in.Data.ArticleLimit = previous.ArticleLimit
				}
				if in.Data.IntroPopup == nil {
					in.Data.IntroPopup = previous.IntroPopup
				}
			}
			if in.Kind == "article" {
				var count int
				if err := db.QueryRow(ctx, `SELECT count(*) FROM article_site WHERE article_id=$1`, id).Scan(&count); err != nil {
					return err
				}
				if count > 0 && !in.Confirm {
					return fmt.Errorf("%w: 修改共用報告需確認所有關聯樣點", service.ErrValidation)
				}
			}
		}
		if err := in.Validate(); err != nil {
			return err
		}
		if in.Kind == "activity" {
			if err := repo.validateSiteIDs(ctx, in.Data.SiteIDs); err != nil {
				return err
			}
		}
		if id > 0 && in.Kind == "profile" && in.Status == "published" && in.SiteID != nil {
			if _, err := db.Exec(ctx, `SELECT pg_advisory_xact_lock(76103,$1)`, *in.SiteID); err != nil {
				return err
			}
			old, err := jsonRows[service.PublicContent](ctx, db, `SELECT to_jsonb(c)-'source_snapshot' FROM public_content c WHERE kind='profile' AND site_id=$1 AND id<>$2 AND publication_status='published' FOR UPDATE`, *in.SiteID, id)
			if err != nil {
				return err
			}
			for _, c := range old {
				if _, err = db.Exec(ctx, `UPDATE public_content SET publication_status='archived',updated_at=clock_timestamp() WHERE id=$1`, c.ID); err != nil {
					return err
				}
				if err = displayAudit(ctx, db, "profile_archive", c.ID, actor, c, map[string]string{"publication_status": "archived"}); err != nil {
					return err
				}
			}
		}
		var err error
		result, err = repo.saveContentRow(ctx, id, in, actor)
		if err != nil {
			return err
		}
		if in.Kind == "activity" {
			if err = repo.syncEntryConfig(ctx, &result, in.Data, actor); err != nil {
				return err
			}
		}
		if in.Kind == "article" {
			if err = db.QueryRow(ctx, `SELECT ARRAY(SELECT site_id FROM article_site WHERE article_id=$1 ORDER BY site_id)`, result.ID).Scan(&result.RelatedSiteIDs); err != nil {
				return err
			}
		}
		if in.Kind == "development" {
			if err = repo.saveDevelopment(ctx, result.ID, in.Data); err != nil {
				return err
			}
		}
		return nil
	})
	return result, err
}

func (r ReefDataRepository) syncEntryConfig(ctx context.Context, c *service.PublicContent, d service.ContentData, actor string) error {
	var id int
	var before []byte
	err := r.db.QueryRow(ctx, `SELECT id,(to_jsonb(a)-'code_hash')||jsonb_build_object('site_ids',ARRAY(SELECT site_id FROM reef_check_activity_site WHERE activity_id=a.id ORDER BY site_id)) FROM reef_check_activity a WHERE activity_content_id=$1 FOR UPDATE`, c.ID).Scan(&id, &before)
	if err != nil && err != pgx.ErrNoRows {
		return err
	}
	if err == pgx.ErrNoRows && !d.EntryEnabled {
		return nil
	}
	start, _ := time.Parse(time.RFC3339, d.StartAt)
	end, _ := time.Parse(time.RFC3339, d.EndAt)
	taipei := time.FixedZone("Asia/Taipei", 8*3600)
	action := "update"
	if id == 0 {
		code, e := newActivityCode()
		if e != nil {
			return e
		}
		err = r.db.QueryRow(ctx, `INSERT INTO reef_check_activity(activity_content_id,name,code_hash,start_date,end_date,is_active,expires_at,created_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8::uuid) RETURNING id`, c.ID, d.Title, activityHash(code), start.In(taipei).Format("2006-01-02"), end.In(taipei).Format("2006-01-02"), d.EntryEnabled, d.ExpiresAt, actor).Scan(&id)
		if err != nil {
			return translateError(err)
		}
		c.EntryCode = code
		action = "create"
	} else {
		_, err = r.db.Exec(ctx, `UPDATE reef_check_activity SET name=$2,start_date=$3,end_date=$4,is_active=$5,expires_at=$6,updated_at=clock_timestamp() WHERE id=$1`, id, d.Title, start.In(taipei).Format("2006-01-02"), end.In(taipei).Format("2006-01-02"), d.EntryEnabled, d.ExpiresAt)
		if err != nil {
			return err
		}
	}
	c.EntryConfigID = &id
	if _, err = r.db.Exec(ctx, `DELETE FROM reef_check_activity_site WHERE activity_id=$1`, id); err != nil {
		return err
	}
	for _, siteID := range d.SiteIDs {
		if _, err = r.db.Exec(ctx, `INSERT INTO reef_check_activity_site(activity_id,site_id) VALUES($1,$2)`, id, siteID); err != nil {
			return err
		}
	}
	_, err = r.db.Exec(ctx, `INSERT INTO reef_check_activity_audit(activity_id,actor_id,action,before_data,after_data) SELECT id,$2::uuid,$3,$4::jsonb,(to_jsonb(a)-'code_hash')||jsonb_build_object('site_ids',$5::int[]) FROM reef_check_activity a WHERE id=$1`, id, actor, action, before, d.SiteIDs)
	return err
}

func (r ReefDataRepository) saveDevelopment(ctx context.Context, id int, d service.ContentData) error {
	raw, err := json.Marshal(d)
	if err != nil {
		return err
	}
	if _, err = r.db.Exec(ctx, `INSERT INTO development_case(id,data) VALUES($1,$2::jsonb-'timeline'-'media') ON CONFLICT(id) DO UPDATE SET data=EXCLUDED.data,updated_at=clock_timestamp()`, id, raw); err != nil {
		return err
	}
	if _, err = r.db.Exec(ctx, `DELETE FROM development_case_timeline WHERE case_id=$1`, id); err != nil {
		return err
	}
	if _, err = r.db.Exec(ctx, `DELETE FROM development_case_media WHERE case_id=$1`, id); err != nil {
		return err
	}
	for i, t := range d.Timeline {
		v, e := json.Marshal(t)
		if e != nil {
			return e
		}
		if _, err = r.db.Exec(ctx, `INSERT INTO development_case_timeline(case_id,sort_order,data) VALUES($1,$2,$3::jsonb)`, id, i, v); err != nil {
			return err
		}
	}
	for i, m := range d.Media {
		v, e := json.Marshal(m)
		if e != nil {
			return e
		}
		if _, err = r.db.Exec(ctx, `INSERT INTO development_case_media(case_id,sort_order,data) VALUES($1,$2,$3::jsonb)`, id, i, v); err != nil {
			return err
		}
	}
	return nil
}
