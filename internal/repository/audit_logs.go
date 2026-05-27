package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type AuditAction string

const (
	AuditActionCreate AuditAction = "create"
	AuditActionUpdate AuditAction = "update"
	AuditActionDelete AuditAction = "delete"
)

type AuditLog struct {
	ID          uuid.UUID
	Action      AuditAction
	TargetTable string
	TargetID    uuid.UUID
	ActorUserID *uuid.UUID
	ActorEmail  string
	BeforeData  json.RawMessage
	AfterData   json.RawMessage
	Method      string
	Path        string
	IP          string
	UserAgent   string
	LoggedAt    time.Time
}

type CreateAuditLogRecord struct {
	Action      AuditAction
	TargetTable string
	TargetID    uuid.UUID
	ActorUserID *uuid.UUID
	ActorEmail  string
	BeforeData  any
	AfterData   any
	Method      string
	Path        string
	IP          string
	UserAgent   string
}

type AuditLogRepository struct {
	db DBTX
}

func NewAuditLogRepository(db DBTX) AuditLogRepository {
	return AuditLogRepository{db: db}
}

func (r AuditLogRepository) ListAuditLogs(ctx context.Context) ([]AuditLog, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, action::text, target_table, target_id, actor_user_id, actor_email, before_data, after_data, method, path, COALESCE(ip::text, ''), user_agent, logged_at
		FROM audit_logs
		ORDER BY logged_at DESC
	`)
	if err != nil {
		return nil, translateError(err)
	}
	defer rows.Close()

	var logs []AuditLog
	for rows.Next() {
		log, err := scanAuditLog(rows)
		if err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}
	return logs, translateError(rows.Err())
}

func (r AuditLogRepository) CreateAuditLog(ctx context.Context, input CreateAuditLogRecord) (AuditLog, error) {
	beforeData, err := marshalNullableJSON(input.BeforeData)
	if err != nil {
		return AuditLog{}, err
	}
	afterData, err := marshalNullableJSON(input.AfterData)
	if err != nil {
		return AuditLog{}, err
	}
	var ip any
	if input.IP != "" {
		ip = input.IP
	}
	return scanAuditLog(r.db.QueryRow(ctx, `
		INSERT INTO audit_logs (action, target_table, target_id, actor_user_id, actor_email, before_data, after_data, method, path, ip, user_agent)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, action::text, target_table, target_id, actor_user_id, actor_email, before_data, after_data, method, path, COALESCE(ip::text, ''), user_agent, logged_at
	`, input.Action, input.TargetTable, input.TargetID, input.ActorUserID, input.ActorEmail, beforeData, afterData, input.Method, input.Path, ip, input.UserAgent))
}

type auditLogScanner interface {
	Scan(dest ...any) error
}

func scanAuditLog(row auditLogScanner) (AuditLog, error) {
	var log AuditLog
	var action string
	var actorUserID pgtype.UUID
	err := row.Scan(
		&log.ID,
		&action,
		&log.TargetTable,
		&log.TargetID,
		&actorUserID,
		&log.ActorEmail,
		&log.BeforeData,
		&log.AfterData,
		&log.Method,
		&log.Path,
		&log.IP,
		&log.UserAgent,
		&log.LoggedAt,
	)
	if actorUserID.Valid {
		id := uuid.UUID(actorUserID.Bytes)
		log.ActorUserID = &id
	}
	log.Action = AuditAction(action)
	return log, translateError(err)
}

func marshalNullableJSON(value any) ([]byte, error) {
	if value == nil {
		return nil, nil
	}
	return json.Marshal(value)
}
