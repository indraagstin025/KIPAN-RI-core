package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
)

// OutboxFilter menyaring daftar antrian email (scope server-side).
type OutboxFilter struct {
	Jenis       string
	Status      string
	ProvinsiID  *int
	KabupatenID *int
	Limit       int
	Offset      int
}

// EmailOutboxRepository adalah antrian persisten pengiriman email (outbox).
type EmailOutboxRepository interface {
	Enqueue(ctx context.Context, item *domain.EmailOutbox) error
	ClaimNext(ctx context.Context, limit int) ([]domain.EmailOutbox, error)
	MarkSent(ctx context.Context, id int64) error
	MarkFailed(ctx context.Context, id int64, errMsg string, nextRetry time.Time, dead bool) error
	List(ctx context.Context, f OutboxFilter) ([]domain.EmailOutbox, int, error)
	RetryNow(ctx context.Context, id int64) error
	RetryPending(ctx context.Context, provinsiID, kabupatenID *int) (int64, error)
	RetryMany(ctx context.Context, ids []int64) (int64, error)
}

type emailOutboxRepo struct{ db *sqlx.DB }

func NewEmailOutboxRepository(db *sqlx.DB) EmailOutboxRepository { return &emailOutboxRepo{db: db} }

const outboxColumns = `id, jenis, pendaftaran_id, user_id, provinsi_id, kabupaten_id,
	to_email, subject, text_body, html_body, status, attempts, next_retry_at,
	sent_at, last_error, created_at, updated_at`

func (r *emailOutboxRepo) Enqueue(ctx context.Context, item *domain.EmailOutbox) error {
	query := `INSERT INTO email_outbox
		(jenis, pendaftaran_id, user_id, provinsi_id, kabupaten_id, to_email, subject, text_body, html_body)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, status, attempts, next_retry_at, created_at, updated_at`
	return r.db.QueryRowxContext(ctx, query,
		item.Jenis, item.PendaftaranID, item.UserID, item.ProvinsiID, item.KabupatenID,
		item.ToEmail, item.Subject, item.TextBody, item.HTMLBody,
	).Scan(&item.ID, &item.Status, &item.Attempts, &item.NextRetryAt, &item.CreatedAt, &item.UpdatedAt)
}

// ClaimNext melakukan "lease" pada sejumlah baris pending siap kirim: attempts+1
// dan next_retry_at digeser ke depan, sehingga worker paralel tidak mengambil
// baris yang sama (pola SKIP LOCKED).
func (r *emailOutboxRepo) ClaimNext(ctx context.Context, limit int) ([]domain.EmailOutbox, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	items := make([]domain.EmailOutbox, 0)
	query := `UPDATE email_outbox o
		SET attempts = o.attempts + 1,
		    next_retry_at = CURRENT_TIMESTAMP + INTERVAL '2 minutes',
		    updated_at = CURRENT_TIMESTAMP
		WHERE o.id IN (
			SELECT id FROM email_outbox
			WHERE status = 'pending' AND next_retry_at <= CURRENT_TIMESTAMP
			ORDER BY id
			FOR UPDATE SKIP LOCKED
			LIMIT $1
		)
		RETURNING ` + outboxColumns
	if err := r.db.SelectContext(ctx, &items, query, limit); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *emailOutboxRepo) MarkSent(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE email_outbox
		 SET status = 'sent', sent_at = CURRENT_TIMESTAMP, last_error = NULL, updated_at = CURRENT_TIMESTAMP
		 WHERE id = $1`, id)
	return err
}

// MarkFailed: dead=true → status 'failed' (DLQ, tak di-retry otomatis);
// dead=false → tetap 'pending' dengan next_retry_at berikutnya (backoff).
func (r *emailOutboxRepo) MarkFailed(ctx context.Context, id int64, errMsg string, nextRetry time.Time, dead bool) error {
	status := domain.EmailOutboxPending
	if dead {
		status = domain.EmailOutboxFailed
	}
	if len(errMsg) > 1000 {
		errMsg = errMsg[:1000]
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE email_outbox SET status = $2, last_error = $3, next_retry_at = $4, updated_at = CURRENT_TIMESTAMP
		 WHERE id = $1`, id, status, errMsg, nextRetry)
	return err
}

func outboxWhere(f OutboxFilter) (string, []interface{}) {
	where := []string{"1 = 1"}
	args := []interface{}{}
	if v := strings.TrimSpace(f.Jenis); v != "" {
		args = append(args, v)
		where = append(where, fmt.Sprintf("jenis = $%d", len(args)))
	}
	if v := strings.TrimSpace(f.Status); v != "" {
		args = append(args, v)
		where = append(where, fmt.Sprintf("status = $%d", len(args)))
	}
	if f.ProvinsiID != nil {
		args = append(args, *f.ProvinsiID)
		where = append(where, fmt.Sprintf("provinsi_id = $%d", len(args)))
	}
	if f.KabupatenID != nil {
		args = append(args, *f.KabupatenID)
		where = append(where, fmt.Sprintf("kabupaten_id = $%d", len(args)))
	}
	return strings.Join(where, " AND "), args
}

func (r *emailOutboxRepo) List(ctx context.Context, f OutboxFilter) ([]domain.EmailOutbox, int, error) {
	if f.Limit <= 0 {
		f.Limit = 25
	}
	if f.Limit > 100 {
		f.Limit = 100
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
	where, args := outboxWhere(f)
	var total int
	if err := r.db.GetContext(ctx, &total, `SELECT COUNT(*) FROM email_outbox WHERE `+where, args...); err != nil {
		return nil, 0, err
	}
	items := make([]domain.EmailOutbox, 0)
	args = append(args, f.Limit, f.Offset)
	query := `SELECT ` + outboxColumns + ` FROM email_outbox WHERE ` + where +
		` ORDER BY created_at DESC` + fmt.Sprintf(` LIMIT $%d OFFSET $%d`, len(args)-1, len(args))
	if err := r.db.SelectContext(ctx, &items, query, args...); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *emailOutboxRepo) RetryNow(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE email_outbox SET status = 'pending', next_retry_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		 WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// RetryPending menggeser SEMUA pending (dalam scope) agar segera diproses worker.
func (r *emailOutboxRepo) RetryPending(ctx context.Context, provinsiID, kabupatenID *int) (int64, error) {
	where := []string{"status = 'pending'"}
	args := []interface{}{}
	if provinsiID != nil {
		args = append(args, *provinsiID)
		where = append(where, fmt.Sprintf("provinsi_id = $%d", len(args)))
	}
	if kabupatenID != nil {
		args = append(args, *kabupatenID)
		where = append(where, fmt.Sprintf("kabupaten_id = $%d", len(args)))
	}
	res, err := r.db.ExecContext(ctx,
		`UPDATE email_outbox SET next_retry_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE `+strings.Join(where, " AND "), args...)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// RetryMany mengembalikan baris terpilih (mis. failed) ke pending untuk kirim ulang.
func (r *emailOutboxRepo) RetryMany(ctx context.Context, ids []int64) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	ph := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		ph[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}
	res, err := r.db.ExecContext(ctx,
		`UPDATE email_outbox SET status = 'pending', next_retry_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		 WHERE id IN (`+strings.Join(ph, ",")+`)`, args...)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}
