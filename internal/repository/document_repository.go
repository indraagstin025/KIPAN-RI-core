package repository

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"

	"github.com/jmoiron/sqlx"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
)

// DocumentOwner merepresentasikan pemilik (jurisdiksi) sebuah object key
// dokumen. Dipakai untuk MENGOTORISASI akses baca dokumen — bukan sekadar
// mengautentikasi pemanggil (SEC-STORE-BOLA / RULES 7).
type DocumentOwner struct {
	Entity      string // "pendaftaran" | "anggota"
	EntityID    string
	ProvinsiID  int
	KabupatenID int
}

// DocumentRepository memetakan object key dokumen ke entitas pemiliknya.
type DocumentRepository interface {
	// ResolveOwner mengembalikan pemilik key, atau domain.ErrNotFound bila
	// key tidak dirujuk oleh entitas mana pun.
	ResolveOwner(ctx context.Context, key string) (*DocumentOwner, error)
}

type documentRepo struct {
	db *sqlx.DB
}

func NewDocumentRepository(db *sqlx.DB) DocumentRepository {
	return &documentRepo{db: db}
}

// Kolom dokumen bucket uploads (bukan kta_pdf_key — itu bucket private dan
// tidak dilayani endpoint presign-view). Klausa OR memakai placeholder yang
// sama ($1) sehingga satu argumen cukup.
const docOwnerPendaftaranSQL = `
	SELECT id, provinsi_id, kabupaten_id
	FROM pendaftaran
	WHERE foto_key = $1 OR ktp_key = $1 OR cv_key = $1 OR sk_key = $1
	   OR surat_pernyataan_key = $1 OR surat_sehat_key = $1
	ORDER BY id
	LIMIT 1`

const docOwnerAnggotaSQL = `
	SELECT id, provinsi_id, kabupaten_id
	FROM anggota
	WHERE foto_key = $1 OR ktp_key = $1 OR cv_key = $1 OR sk_key = $1
	   OR surat_pernyataan_key = $1 OR surat_sehat_key = $1
	ORDER BY id
	LIMIT 1`

func (r *documentRepo) ResolveOwner(ctx context.Context, key string) (*DocumentOwner, error) {
	k := strings.TrimSpace(key)
	if k == "" {
		return nil, domain.ErrNotFound
	}
	// Pendaftaran diperiksa lebih dulu: dokumen paling sering dibuka saat
	// verifikasi. Kegagalan selain "no rows" diteruskan apa adanya.
	if owner, err := r.lookup(ctx, docOwnerPendaftaranSQL, "pendaftaran", k); err == nil {
		return owner, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if owner, err := r.lookup(ctx, docOwnerAnggotaSQL, "anggota", k); err == nil {
		return owner, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	return nil, domain.ErrNotFound
}

func (r *documentRepo) lookup(ctx context.Context, query, entity, key string) (*DocumentOwner, error) {
	var row struct {
		ID          int `db:"id"`
		ProvinsiID  int `db:"provinsi_id"`
		KabupatenID int `db:"kabupaten_id"`
	}
	if err := r.db.GetContext(ctx, &row, query, key); err != nil {
		return nil, err
	}
	return &DocumentOwner{
		Entity:      entity,
		EntityID:    strconv.Itoa(row.ID),
		ProvinsiID:  row.ProvinsiID,
		KabupatenID: row.KabupatenID,
	}, nil
}
