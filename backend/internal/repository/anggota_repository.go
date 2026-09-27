package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/jmoiron/sqlx"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
)

// AnggotaRepository menangani query minimal kader resmi untuk kebutuhan
// Fase 2 (cek duplikat NIK lintas tabel). Repository penuh anggota
// (list/detail/status) dibangun di batch berikutnya.
type AnggotaRepository interface {
	// ExistsByNikHash mengembalikan true bila NIK (blind index) sudah
	// terdaftar sebagai anggota resmi. Tidak pernah ErrNotFound.
	ExistsByNikHash(ctx context.Context, nikHash string) (bool, error)
	// GetByNIA mengambil anggota berdasarkan NIA untuk verifikasi KTA.
	GetByNIA(ctx context.Context, nia string) (*domain.Anggota, error)
}

type anggotaRepo struct {
	db *sqlx.DB
}

func NewAnggotaRepository(db *sqlx.DB) AnggotaRepository {
	return &anggotaRepo{db: db}
}

func (r *anggotaRepo) ExistsByNikHash(ctx context.Context, nikHash string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM anggota WHERE nik_hash = $1)`
	if err := r.db.GetContext(ctx, &exists, query, strings.TrimSpace(nikHash)); err != nil {
		return false, err
	}
	return exists, nil
}

func (r *anggotaRepo) GetByNIA(ctx context.Context, nia string) (*domain.Anggota, error) {
	var a domain.Anggota
	query := `SELECT * FROM anggota WHERE nia = $1`
	if err := r.db.GetContext(ctx, &a, query, strings.TrimSpace(nia)); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &a, nil
}
