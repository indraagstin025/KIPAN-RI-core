package repository

// organisasi_repository.go — akses profil organisasi (tabel 1-baris).

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jmoiron/sqlx"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
)

// OrganisasiRepository mengelola profil organisasi.
type OrganisasiRepository interface {
	Get(ctx context.Context) (*domain.OrganisasiProfile, error)
	Update(ctx context.Context, in domain.OrganisasiUpdateRequest, updatedBy string) (*domain.OrganisasiProfile, error)
}

type organisasiRepo struct{ db *sqlx.DB }

func NewOrganisasiRepository(db *sqlx.DB) OrganisasiRepository { return &organisasiRepo{db: db} }

const organisasiColumns = `id, nama, singkatan, deskripsi, visi, misi, alamat, email, telepon,
	whatsapp, website, instagram, facebook, youtube, tiktok, logo_url, updated_by, updated_at`

// Get mengambil profil organisasi (baris id = 1).
func (r *organisasiRepo) Get(ctx context.Context) (*domain.OrganisasiProfile, error) {
	var p domain.OrganisasiProfile
	if err := r.db.GetContext(ctx, &p, `SELECT `+organisasiColumns+` FROM organisasi_profile WHERE id = 1`); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

// Update menyimpan perubahan profil organisasi (baris id = 1).
func (r *organisasiRepo) Update(ctx context.Context, in domain.OrganisasiUpdateRequest, updatedBy string) (*domain.OrganisasiProfile, error) {
	var p domain.OrganisasiProfile
	var by interface{}
	if updatedBy != "" {
		by = updatedBy
	}
	err := r.db.GetContext(ctx, &p, `
		UPDATE organisasi_profile SET
			nama = $1, singkatan = $2, deskripsi = $3, visi = $4, misi = $5, alamat = $6,
			email = $7, telepon = $8, whatsapp = $9, website = $10, instagram = $11,
			facebook = $12, youtube = $13, tiktok = $14, logo_url = $15,
			updated_by = $16, updated_at = CURRENT_TIMESTAMP
		WHERE id = 1
		RETURNING `+organisasiColumns,
		in.Nama, in.Singkatan, in.Deskripsi, in.Visi, in.Misi, in.Alamat,
		in.Email, in.Telepon, in.Whatsapp, in.Website, in.Instagram,
		in.Facebook, in.Youtube, in.Tiktok, in.LogoURL, by)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}
