package repository

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"coast-monitoring/internal/policy"
	"coast-monitoring/internal/service"
	"github.com/jackc/pgx/v5"
)

func activityHash(code string) string {
	sum := sha256.Sum256([]byte(strings.ToUpper(strings.TrimSpace(code))))
	return hex.EncodeToString(sum[:])
}
func newActivityCode() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "RC-" + strings.ToUpper(hex.EncodeToString(b)), nil
}

const activitySelect = `SELECT a.id,a.activity_content_id,a.name,a.start_date::text,a.end_date::text,a.is_active,a.expires_at,
 ARRAY(SELECT s.site_id FROM reef_check_activity_site s WHERE s.activity_id=a.id ORDER BY s.site_id)
 FROM reef_check_activity a `

func (r ReefDataRepository) VerifyActivity(ctx context.Context, code string) (service.ReefCheckActivity, error) {
	var a service.ReefCheckActivity
	if len(strings.TrimSpace(code)) < 8 || len(code) > 80 {
		return a, fmt.Errorf("%w: 活動代碼無效或已失效", service.ErrValidation)
	}
	err := r.db.QueryRow(ctx, activitySelect+`WHERE a.code_hash=$1 AND a.is_active AND (a.expires_at IS NULL OR a.expires_at>now())`, activityHash(code)).Scan(&a.ID, &a.ActivityID, &a.Name, &a.StartDate, &a.EndDate, &a.Active, &a.ExpiresAt, &a.SiteIDs)
	if err != nil {
		if err == pgx.ErrNoRows {
			return a, fmt.Errorf("%w: 活動代碼無效或已失效", service.ErrValidation)
		}
		return a, translateError(err)
	}
	a.EntryConfigID = a.ID
	return a, nil
}
func (r ReefDataRepository) ListActivities(ctx context.Context) ([]service.ReefCheckActivity, error) {
	rows, err := r.db.Query(ctx, activitySelect+`ORDER BY a.id DESC`)
	if err != nil {
		return nil, translateError(err)
	}
	defer rows.Close()
	out := []service.ReefCheckActivity{}
	for rows.Next() {
		var a service.ReefCheckActivity
		if err := rows.Scan(&a.ID, &a.ActivityID, &a.Name, &a.StartDate, &a.EndDate, &a.Active, &a.ExpiresAt, &a.SiteIDs); err != nil {
			return nil, err
		}
		a.EntryConfigID = a.ID
		out = append(out, a)
	}
	return out, rows.Err()
}
func (r ReefDataRepository) CreateActivity(ctx context.Context, input service.ReefCheckActivityInput, actor policy.User) (service.ReefCheckActivity, error) {
	if err := input.Validate(); err != nil {
		return service.ReefCheckActivity{}, err
	}
	c, err := r.SaveContent(ctx, 0, service.PublicContentInput{Kind: "activity", Data: service.ContentData{Title: input.Name, StartAt: input.StartDate + "T00:00:00+08:00", EndAt: input.EndDate + "T23:59:59+08:00", Recruitment: "internal", Status: "scheduled", EntryEnabled: true, SiteIDs: input.SiteIDs, ExpiresAt: input.ExpiresAt}}, actor.ID.String())
	if err != nil {
		return service.ReefCheckActivity{}, err
	}
	return service.ReefCheckActivity{ID: *c.EntryConfigID, EntryConfigID: *c.EntryConfigID, ActivityID: c.ID, Name: input.Name, StartDate: input.StartDate, EndDate: input.EndDate, SiteIDs: input.SiteIDs, Active: true, ExpiresAt: input.ExpiresAt, Code: c.EntryCode}, nil
}
func (r ReefDataRepository) ChangeActivity(ctx context.Context, id int, action string, actor policy.User) (service.ReefCheckActivity, error) {
	result := service.ReefCheckActivity{ID: id, EntryConfigID: id, Active: action == "rotate"}
	if action != "disable" && action != "rotate" {
		return result, service.ErrValidation
	}
	err := reefTransaction(ctx, r.db, func(db DBTX) error {
		var cid int
		var before []byte
		if err := db.QueryRow(ctx, `SELECT activity_content_id FROM reef_check_activity WHERE id=$1`, id).Scan(&cid); err != nil {
			return translateError(err)
		}
		if _, err := db.Exec(ctx, `SELECT id FROM public_content WHERE id=$1 FOR UPDATE`, cid); err != nil {
			return err
		}
		if err := db.QueryRow(ctx, `SELECT to_jsonb(a)-'code_hash' FROM reef_check_activity a WHERE id=$1 FOR UPDATE`, id).Scan(&before); err != nil {
			return err
		}
		if action == "rotate" {
			code, err := newActivityCode()
			if err != nil {
				return err
			}
			result.Code = code
			if _, err = db.Exec(ctx, `UPDATE reef_check_activity SET code_hash=$2,is_active=true,updated_at=clock_timestamp() WHERE id=$1`, id, activityHash(code)); err != nil {
				return err
			}
		} else {
			if _, err := db.Exec(ctx, `UPDATE reef_check_activity SET is_active=false,updated_at=clock_timestamp() WHERE id=$1`, id); err != nil {
				return err
			}
		}
		if _, err := db.Exec(ctx, `WITH old AS(SELECT * FROM public_content WHERE id=$1),changed AS(UPDATE public_content SET data=jsonb_set(data,'{entry_enabled}',to_jsonb($2::boolean)),updated_at=clock_timestamp() WHERE id=$1 RETURNING *) INSERT INTO public_content_audit(content_id,actor_id,before_data,after_data) SELECT $1,$3::uuid,to_jsonb(old),to_jsonb(changed) FROM old,changed`, cid, result.Active, actor.ID); err != nil {
			return err
		}
		if _, err := db.Exec(ctx, `INSERT INTO reef_check_activity_audit(activity_id,actor_id,action,before_data,after_data) SELECT id,$2::uuid,$3,$4::jsonb,to_jsonb(a)-'code_hash' FROM reef_check_activity a WHERE id=$1`, id, actor.ID, action, before); err != nil {
			return err
		}
		result.ActivityID = cid
		return nil
	})
	return result, err
}

// The database rechecks scope at submission time, not only when the UI verifies.
func (r ReefDataRepository) checkActivityScope(ctx context.Context, sub service.ReefCheckSurveySubmission) (*service.ReefCheckActivity, error) {
	if sub.Format != "reefcheck-web-v4" {
		return nil, nil
	}
	var locked int
	if err := r.db.QueryRow(ctx, `SELECT id FROM reef_check_activity WHERE code_hash=$1 FOR SHARE`, activityHash(sub.ActivityCode)).Scan(&locked); err != nil {
		if err == pgx.ErrNoRows {
			return nil, service.ErrValidation
		}
		return nil, err
	}
	a, err := r.VerifyActivity(ctx, sub.ActivityCode)
	if err != nil {
		return nil, err
	}
	if !a.Allows(sub.Event.SiteID, sub.Event.SurveyDate, time.Now()) {
		return nil, fmt.Errorf("%w: 樣點或日期不在本活動範圍", service.ErrValidation)
	}
	return &a, nil
}
