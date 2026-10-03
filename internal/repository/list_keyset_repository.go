package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
)

// ListKeysetRepository menyediakan varian keyset (seek) pagination untuk
// daftar besar. Dipisah dari interface utama agar konsumen lama tak berubah.
type ListKeysetRepository interface {
	ListAnggotaKeyset(ctx context.Context, provinsiID, kabupatenID *int, status, search string, at time.Time, id, limit int) ([]domain.AnggotaListItem, error)
	ListQueueKeyset(ctx context.Context, provinsiID, kabupatenID *int, status string, at time.Time, id, limit int) ([]domain.PendaftaranQueueItem, error)
}

type listKeysetRepo struct{ db *sqlx.DB }

func NewListKeysetRepository(db *sqlx.DB) ListKeysetRepository { return &listKeysetRepo{db: db} }

// ListAnggotaKeyset: keyset `(created_at, id) < (at, id)` memakai indeks
// komposit idx_anggota_created_id — tanpa OFFSET besar.
func (r *listKeysetRepo) ListAnggotaKeyset(ctx context.Context, provinsiID, kabupatenID *int, status, search string, at time.Time, id, limit int) ([]domain.AnggotaListItem, error) {
	limit = boundLimit(limit)
	where, args := anggotaWhere(provinsiID, kabupatenID, status, search)
	args = append(args, at, id)
	where = fmt.Sprintf("%s AND (a.created_at, a.id) < ($%d, $%d)", where, len(args)-1, len(args))
	args = append(args, limit)
	items := make([]domain.AnggotaListItem, 0)
	query := `SELECT ` + anggotaListColumns + ` ` + anggotaListJoins +
		` WHERE ` + where + ` ORDER BY a.created_at DESC, a.id DESC` +
		fmt.Sprintf(` LIMIT $%d`, len(args))
	if err := r.db.SelectContext(ctx, &items, query, args...); err != nil {
		return nil, err
	}
	return items, nil
}

// ListQueueKeyset: keyset pada pendaftaran memakai idx_pendaftaran_status_created
// / idx_pendaftaran_wilayah_created.
func (r *listKeysetRepo) ListQueueKeyset(ctx context.Context, provinsiID, kabupatenID *int, status string, at time.Time, id, limit int) ([]domain.PendaftaranQueueItem, error) {
	limit = boundLimit(limit)
	where, args := queueWhere(provinsiID, kabupatenID, status)
	args = append(args, at, id)
	where = fmt.Sprintf("%s AND (created_at, id) < ($%d, $%d)", where, len(args)-1, len(args))
	args = append(args, limit)
	query := `SELECT ` + queueColumns + ` FROM pendaftaran WHERE ` + where +
		` ORDER BY created_at DESC, id DESC` + fmt.Sprintf(` LIMIT $%d`, len(args))
	rows, err := r.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.PendaftaranQueueItem, 0)
	for rows.Next() {
		var item domain.PendaftaranQueueItem
		if err := rows.StructScan(&item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
