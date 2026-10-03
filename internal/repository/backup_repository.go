package repository

// backup_repository.go — riwayat backup database.

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jmoiron/sqlx"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
)

// BackupRepository mengelola riwayat backup.
type BackupRepository interface {
	Create(ctx context.Context, b *domain.Backup) (*domain.Backup, error)
	List(ctx context.Context, limit, offset int) ([]domain.Backup, int, error)
	GetByID(ctx context.Context, id int) (*domain.Backup, error)
	Delete(ctx context.Context, id int) error
}

type backupRepo struct{ db *sqlx.DB }

func NewBackupRepository(db *sqlx.DB) BackupRepository { return &backupRepo{db: db} }

const backupColumns = `id, filename, object_key, size_bytes, status, error, created_by, created_at`

// Create menyimpan entri backup baru.
func (r *backupRepo) Create(ctx context.Context, b *domain.Backup) (*domain.Backup, error) {
	var out domain.Backup
	if err := r.db.GetContext(ctx, &out, `
		INSERT INTO backups (filename, object_key, size_bytes, status, error, created_by)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING `+backupColumns,
		b.Filename, b.ObjectKey, b.SizeBytes, b.Status, b.Error, b.CreatedBy); err != nil {
		return nil, err
	}
	return &out, nil
}

// List mengembalikan riwayat backup terbaru + total.
func (r *backupRepo) List(ctx context.Context, limit, offset int) ([]domain.Backup, int, error) {
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	var total int
	if err := r.db.GetContext(ctx, &total, `SELECT COUNT(*) FROM backups`); err != nil {
		return nil, 0, err
	}
	items := make([]domain.Backup, 0)
	if err := r.db.SelectContext(ctx, &items,
		`SELECT `+backupColumns+` FROM backups ORDER BY created_at DESC, id DESC LIMIT $1 OFFSET $2`,
		limit, offset); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// GetByID mengambil satu entri backup.
func (r *backupRepo) GetByID(ctx context.Context, id int) (*domain.Backup, error) {
	var b domain.Backup
	if err := r.db.GetContext(ctx, &b, `SELECT `+backupColumns+` FROM backups WHERE id = $1`, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &b, nil
}

// Delete menghapus entri backup.
func (r *backupRepo) Delete(ctx context.Context, id int) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM backups WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrNotFound
	}
	return nil
}
