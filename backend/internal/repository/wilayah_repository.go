package repository

import (
	"context"

	"github.com/jmoiron/sqlx"
)

// WilayahRepository menangani lookup master wilayah untuk validasi
// server-side (RULES 6: provinsi/kabupaten dari client wajib dicek ke
// master, termasuk relasi kabupaten∈provinsi dan flag aktif).
type WilayahRepository interface {
	// ExistsProvinsi true bila provinsi ada dan aktif.
	ExistsProvinsi(ctx context.Context, id int) (bool, error)
	// KabupatenInProvinsi true bila kabupaten ada, aktif, dan milik
	// provinsi tersebut.
	KabupatenInProvinsi(ctx context.Context, kabupatenID, provinsiID int) (bool, error)
}

type wilayahRepo struct {
	db *sqlx.DB
}

func NewWilayahRepository(db *sqlx.DB) WilayahRepository {
	return &wilayahRepo{db: db}
}

func (r *wilayahRepo) ExistsProvinsi(ctx context.Context, id int) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM wilayah_provinsi WHERE id = $1 AND is_active = TRUE)`
	if err := r.db.GetContext(ctx, &exists, query, id); err != nil {
		return false, err
	}
	return exists, nil
}

func (r *wilayahRepo) KabupatenInProvinsi(ctx context.Context, kabupatenID, provinsiID int) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM wilayah_kabupaten
		WHERE id = $1 AND provinsi_id = $2 AND is_active = TRUE)`
	if err := r.db.GetContext(ctx, &exists, query, kabupatenID, provinsiID); err != nil {
		return false, err
	}
	return exists, nil
}
