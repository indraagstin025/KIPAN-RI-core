package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
)

// AuditLogRepository menulis & menelusuri jejak audit forensik (RULES 21).
//
// APPEND-ONLY by design: tidak ada Update/Delete di interface ini, dan mutasi
// DML juga ditolak trigger trg_activity_logs_no_update di database (000005).
//
// PII (NIK utuh, token, password, hash) TIDAK BOLEH masuk Metadata —
// pemanggil wajib memasking sebelum Create.
type AuditLogRepository interface {
	Create(ctx context.Context, e *domain.ActivityLog) error
	// List menelusuri audit dengan filter + pagination (TDD §7.2).
	List(ctx context.Context, f domain.AuditFilter) ([]domain.ActivityLog, int, error)
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

const activityLogColumns = `id, actor_id, actor_name, actor_role, ip_address, user_agent,
	entity_name, entity_id, action, metadata, request_id, created_at`

// List menelusuri audit terfilter (aksi, entitas, aktor, rentang waktu) dengan
// pagination offset. Total dihitung hanya bila diminta.
func (r *auditLogRepo) List(ctx context.Context, f domain.AuditFilter) ([]domain.ActivityLog, int, error) {
	where := []string{"1 = 1"}
	args := []interface{}{}
	if v := strings.TrimSpace(f.Action); v != "" {
		args = append(args, v)
		where = append(where, fmt.Sprintf("action = $%d", len(args)))
	}
	if v := strings.TrimSpace(f.Entity); v != "" {
		args = append(args, v)
		where = append(where, fmt.Sprintf("entity_name = $%d", len(args)))
	}
	if v := strings.TrimSpace(f.Actor); v != "" {
		args = append(args, "%"+v+"%")
		where = append(where, fmt.Sprintf("actor_name ILIKE $%d", len(args)))
	}
	if f.From != nil {
		args = append(args, *f.From)
		where = append(where, fmt.Sprintf("created_at >= $%d", len(args)))
	}
	if f.To != nil {
		args = append(args, *f.To)
		where = append(where, fmt.Sprintf("created_at <= $%d", len(args)))
	}
	clause := strings.Join(where, " AND ")

	total := 0
	if f.WithTotal {
		if err := r.db.GetContext(ctx, &total, `SELECT COUNT(*) FROM activity_logs WHERE `+clause, args...); err != nil {
			return nil, 0, err
		}
	}

	limit := f.Limit
	if limit <= 0 {
		limit = 25
	}
	if limit > 200 {
		limit = 200
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}

	query := `SELECT ` + activityLogColumns + ` FROM activity_logs WHERE ` + clause +
		` ORDER BY created_at DESC, id DESC` +
		fmt.Sprintf(` LIMIT $%d OFFSET $%d`, len(args)+1, len(args)+2)
	args = append(args, limit, offset)

	items := make([]domain.ActivityLog, 0)
	if err := r.db.SelectContext(ctx, &items, query, args...); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}
