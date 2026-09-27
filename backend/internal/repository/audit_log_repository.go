package repository

import (
	"context"

	"github.com/jmoiron/sqlx"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
)

// AuditLogRepository menulis jejak audit forensik (RULES 21).
//
// APPEND-ONLY by design: hanya ada Create. Tidak ada Update/Delete di
// interface ini, dan mutasi DML juga ditolak trigger
// trg_activity_logs_no_update di database (migrasi 000005).
//
// PII (NIK utuh, token, password, hash) TIDAK BOLEH masuk Metadata —
// pemanggil wajib memasking sebelum Create.
type AuditLogRepository interface {
	Create(ctx context.Context, e *domain.ActivityLog) error
}

type auditLogRepo struct {
	db *sqlx.DB
}

func NewAuditLogRepository(db *sqlx.DB) AuditLogRepository {
	return &auditLogRepo{db: db}
}

func (r *auditLogRepo) Create(ctx context.Context, e *domain.ActivityLog) error {
	query := `INSERT INTO activity_logs
		(actor_id, actor_name, actor_role, ip_address, user_agent,
		 entity_name, entity_id, action, metadata, request_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb, $10)
		RETURNING id, created_at`

	// Metadata wajib JSON valid bila diisi (kolom JSONB menolak string
	// mentah); NULL bila tidak ada.
	var metadata interface{}
	if e.Metadata != nil {
		metadata = *e.Metadata
	}

	return r.db.QueryRowxContext(ctx, query,
		e.ActorID, e.ActorName, e.ActorRole, e.IPAddress, e.UserAgent,
		e.EntityName, e.EntityID, e.Action, metadata, e.RequestID,
	).Scan(&e.ID, &e.CreatedAt)
}
