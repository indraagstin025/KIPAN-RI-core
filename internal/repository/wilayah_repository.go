package repository

import (
	"context"

	"github.com/jmoiron/sqlx"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
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
	// ListProvinsi mengembalikan provinsi aktif (untuk dropdown publik).
	ListProvinsi(ctx context.Context) ([]domain.WilayahProvinsi, error)
	// ListKabupaten mengembalikan kabupaten aktif dalam satu provinsi.
	ListKabupaten(ctx context.Context, provinsiID int) ([]domain.WilayahKabupaten, error)
	// GetNames mengembalikan nama provinsi + kabupaten untuk DTO tampil
	// (track publik, daftar anggota). Nama kosong bila ID tak dikenal.
	GetNames(ctx context.Context, provinsiID, kabupatenID int) (provinsi, kabupaten string, err error)
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

func (r *wilayahRepo) ListProvinsi(ctx context.Context) ([]domain.WilayahProvinsi, error) {
	items := make([]domain.WilayahProvinsi, 0)
	query := `SELECT id, kode, nama, is_active, created_at, updated_at
		FROM wilayah_provinsi WHERE is_active = TRUE ORDER BY nama`
	if err := r.db.SelectContext(ctx, &items, query); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *wilayahRepo) ListKabupaten(ctx context.Context, provinsiID int) ([]domain.WilayahKabupaten, error) {
	items := make([]domain.WilayahKabupaten, 0)
	query := `SELECT id, provinsi_id, kode, nama, is_active, created_at, updated_at
		FROM wilayah_kabupaten
		WHERE provinsi_id = $1 AND is_active = TRUE ORDER BY nama`
	if err := r.db.SelectContext(ctx, &items, query, provinsiID); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *wilayahRepo) GetNames(ctx context.Context, provinsiID, kabupatenID int) (string, string, error) {
	var prov, kab string
	query := `SELECT
		COALESCE((SELECT nama FROM wilayah_provinsi WHERE id = $1), ''),
		COALESCE((SELECT nama FROM wilayah_kabupaten WHERE id = $2), '')`
	if err := r.db.QueryRowxContext(ctx, query, provinsiID, kabupatenID).Scan(&prov, &kab); err != nil {
		return "", "", err
	}
	return prov, kab, nil
}
