package repository

import (
	"context"
	"encoding/json"
	"strings"
)

func (r CWAMarineRepository) BeginSync(ctx context.Context) (int, error) {
	var id int
	err := r.db.QueryRow(ctx, `INSERT INTO cwa_sync_run DEFAULT VALUES RETURNING id`).Scan(&id)
	return id, err
}
func (r CWAMarineRepository) FinishSync(ctx context.Context, id int, errors []string) error {
	_, err := r.db.Exec(ctx, `UPDATE cwa_sync_run SET finished_at=clock_timestamp(),success=$2,error_message=NULLIF($3,''),range_from=(SELECT min(observed_date) FROM cwa_sea_temperature),range_to=(SELECT max(observed_date) FROM cwa_sea_temperature) WHERE id=$1`, id, len(errors) == 0, strings.Join(errors, "; "))
	return err
}
func (r CWAMarineRepository) SyncStatus(ctx context.Context) (any, error) {
	var raw json.RawMessage
	err := r.db.QueryRow(ctx, `SELECT jsonb_build_object('last_run',(SELECT to_jsonb(s) FROM cwa_sync_run s ORDER BY id DESC LIMIT 1),'last_success_at',(SELECT max(finished_at) FROM cwa_sync_run WHERE success))`).Scan(&raw)
	return raw, err
}
