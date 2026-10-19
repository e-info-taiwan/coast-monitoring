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

const activitySelect = `SELECT a.id,a.name,a.start_date::text,a.end_date::text,a.is_active,a.expires_at,
 ARRAY(SELECT s.site_id FROM reef_check_activity_site s WHERE s.activity_id=a.id ORDER BY s.site_id)
 FROM reef_check_activity a `

func (r ReefDataRepository) VerifyActivity(ctx context.Context, code string) (service.ReefCheckActivity, error) {
	var a service.ReefCheckActivity
	if len(strings.TrimSpace(code)) < 8 || len(code) > 80 {
		return a, fmt.Errorf("%w: 活動代碼無效或已失效", service.ErrValidation)
	}
	err := r.db.QueryRow(ctx, activitySelect+`WHERE a.code_hash=$1 AND a.is_active AND (a.expires_at IS NULL OR a.expires_at>now())`, activityHash(code)).Scan(&a.ID, &a.Name, &a.StartDate, &a.EndDate, &a.Active, &a.ExpiresAt, &a.SiteIDs)
	if err != nil {
		if err == pgx.ErrNoRows {
			return a, fmt.Errorf("%w: 活動代碼無效或已失效", service.ErrValidation)
		}
		return a, translateError(err)
	}
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
		if err := rows.Scan(&a.ID, &a.Name, &a.StartDate, &a.EndDate, &a.Active, &a.ExpiresAt, &a.SiteIDs); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
func (r ReefDataRepository) CreateActivity(ctx context.Context, input service.ReefCheckActivityInput, actor policy.User) (service.ReefCheckActivity, error) {
	if err := input.Validate(); err != nil {
		return service.ReefCheckActivity{}, err
	}
	code, err := newActivityCode()
	if err != nil {
		return service.ReefCheckActivity{}, err
	}
	exec := r.db
	var tx pgx.Tx
	if starter, ok := r.db.(txStarter); ok {
		tx, err = starter.Begin(ctx)
		if err != nil {
			return service.ReefCheckActivity{}, err
		}
		defer tx.Rollback(ctx)
		exec = tx
	}
	var id int
	err = exec.QueryRow(ctx, `INSERT INTO reef_check_activity(name,code_hash,start_date,end_date,expires_at,created_by) VALUES($1,$2,$3,$4,$5,$6) RETURNING id`, strings.TrimSpace(input.Name), activityHash(code), input.StartDate, input.EndDate, input.ExpiresAt, actor.ID).Scan(&id)
	if err != nil {
		return service.ReefCheckActivity{}, translateError(err)
	}
	for _, siteID := range input.SiteIDs {
		tag, err := exec.Exec(ctx, `INSERT INTO reef_check_activity_site(activity_id,site_id) SELECT $1,id FROM site WHERE id=$2 AND is_active`, id, siteID)
		if err != nil {
			return service.ReefCheckActivity{}, err
		}
		if tag.RowsAffected() != 1 {
			return service.ReefCheckActivity{}, fmt.Errorf("%w: 活動樣點不存在或已停用", service.ErrValidation)
		}
	}
	if _, err = exec.Exec(ctx, `INSERT INTO reef_check_activity_audit(activity_id,actor_id,action) VALUES($1,$2,'create')`, id, actor.ID); err != nil {
		return service.ReefCheckActivity{}, err
	}
	if tx != nil {
		if err = tx.Commit(ctx); err != nil {
			return service.ReefCheckActivity{}, err
		}
	}
	return service.ReefCheckActivity{ID: id, Name: strings.TrimSpace(input.Name), StartDate: input.StartDate, EndDate: input.EndDate, SiteIDs: input.SiteIDs, Active: true, ExpiresAt: input.ExpiresAt, Code: code}, nil
}
func (r ReefDataRepository) ChangeActivity(ctx context.Context, id int, action string, actor policy.User) (service.ReefCheckActivity, error) {
	if action != "disable" && action != "rotate" {
		return service.ReefCheckActivity{}, fmt.Errorf("%w: 活動操作無效", service.ErrValidation)
	}
	exec := r.db
	var tx pgx.Tx
	var err error
	if starter, ok := r.db.(txStarter); ok {
		tx, err = starter.Begin(ctx)
		if err != nil {
			return service.ReefCheckActivity{}, err
		}
		defer tx.Rollback(ctx)
		exec = tx
	}
	code := ""
	query := `UPDATE reef_check_activity SET is_active=false,updated_at=now() WHERE id=$1`
	args := []any{id}
	if action == "rotate" {
		code, err = newActivityCode()
		if err != nil {
			return service.ReefCheckActivity{}, err
		}
		query = `UPDATE reef_check_activity SET code_hash=$2,is_active=true,updated_at=now() WHERE id=$1`
		args = append(args, activityHash(code))
	}
	tag, err := exec.Exec(ctx, query, args...)
	if err != nil {
		return service.ReefCheckActivity{}, err
	}
	if tag.RowsAffected() != 1 {
		return service.ReefCheckActivity{}, service.ErrNotFound
	}
	if _, err = exec.Exec(ctx, `INSERT INTO reef_check_activity_audit(activity_id,actor_id,action) VALUES($1,$2,$3)`, id, actor.ID, action); err != nil {
		return service.ReefCheckActivity{}, err
	}
	if tx != nil {
		if err = tx.Commit(ctx); err != nil {
			return service.ReefCheckActivity{}, err
		}
	}
	return service.ReefCheckActivity{ID: id, Active: action == "rotate", Code: code}, nil
}

// The database rechecks scope at submission time, not only when the UI verifies.
func (r ReefDataRepository) checkActivityScope(ctx context.Context, sub service.ReefCheckSurveySubmission) (*service.ReefCheckActivity, error) {
	if sub.Format != "reefcheck-web-v4" {
		return nil, nil
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
