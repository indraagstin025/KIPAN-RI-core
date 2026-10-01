package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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
	// GetByID mengambil anggota berdasarkan ID.
	GetByID(ctx context.Context, id int) (*domain.Anggota, error)
	// ListAnggota mengambil daftar terfilter wilayah + status + pencarian
	// nama/NIA dengan proyeksi non-PII. limit dibatasi 1-100.
	ListAnggota(ctx context.Context, provinsiID, kabupatenID *int, status, search string, limit, offset int) ([]domain.AnggotaListItem, error)
	// CountAnggota menghitung total baris filter yang sama untuk meta pagination.
	CountAnggota(ctx context.Context, provinsiID, kabupatenID *int, status, search string) (int, error)
	// SetKTAPDFKey menyimpan object key PDF KTA hasil render server.
	SetKTAPDFKey(ctx context.Context, id int, key string) error
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

// anggotaColumns adalah proyeksi eksplisit (item 2: lanjutan BE-005 di
// tabel anggota) — hindari SELECT * agar kolom sensitif baru tidak ikut
// transit memori. NIK tetap aman di JSON via tag `json:"-"`.
const anggotaColumns = `id, nia, nama_lengkap, nik_hash, nik_encrypted,
	tempat_lahir, tanggal_lahir, jenis_kelamin, agama, pendidikan, pekerjaan,
	alamat, provinsi_id, kabupaten_id, kecamatan, desa, kode_pos, email,
	whatsapp, foto_key, ktp_key, cv_key, sk_key, surat_pernyataan_key,
	surat_sehat_key, status, angkatan, kta_qr_hash, kta_pdf_key,
	pendaftaran_id, user_id, tanggal_daftar, tanggal_angkat,
	created_at, updated_at`

func (r *anggotaRepo) GetByID(ctx context.Context, id int) (*domain.Anggota, error) {
	var a domain.Anggota
	query := `SELECT ` + anggotaColumns + ` FROM anggota WHERE id = $1`
	if err := r.db.GetContext(ctx, &a, query, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &a, nil
}

func (r *anggotaRepo) SetKTAPDFKey(ctx context.Context, id int, key string) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE anggota SET kta_pdf_key = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`,
		key, id)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *anggotaRepo) GetByNIA(ctx context.Context, nia string) (*domain.Anggota, error) {
	var a domain.Anggota
	query := `SELECT ` + anggotaColumns + ` FROM anggota WHERE nia = $1`
	if err := r.db.GetContext(ctx, &a, query, strings.TrimSpace(nia)); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &a, nil
}

// anggotaWhere membangun klausa WHERE + args dengan placeholder $n.
// Filter wilayah berasal dari ActorContext server-side (bukan client).
// Pencarian hanya nama/NIA (ILIKE); NIK tidak bisa dicari parsial karena
// terenkripsi GCM + blind index exact-match.
func anggotaWhere(provinsiID, kabupatenID *int, status, search string) (string, []interface{}) {
	where := []string{"1 = 1"}
	args := []interface{}{}
	if provinsiID != nil {
		args = append(args, *provinsiID)
		where = append(where, fmt.Sprintf("a.provinsi_id = $%d", len(args)))
	}
	if kabupatenID != nil {
		args = append(args, *kabupatenID)
		where = append(where, fmt.Sprintf("a.kabupaten_id = $%d", len(args)))
	}
	if s := strings.TrimSpace(status); s != "" {
		args = append(args, s)
		where = append(where, fmt.Sprintf("a.status = $%d", len(args)))
	}
	if q := strings.TrimSpace(search); q != "" {
		args = append(args, "%"+q+"%")
		where = append(where, fmt.Sprintf("(a.nama_lengkap ILIKE $%d OR a.nia ILIKE $%d)", len(args), len(args)))
	}
	return strings.Join(where, " AND "), args
}

const anggotaListColumns = `a.id, a.nia, a.nama_lengkap, a.status,
	a.provinsi_id, p.nama AS provinsi_nama,
	a.kabupaten_id, k.nama AS kabupaten_nama,
	a.tanggal_angkat, a.created_at`

const anggotaListJoins = `FROM anggota a
	JOIN wilayah_provinsi p ON p.id = a.provinsi_id
	JOIN wilayah_kabupaten k ON k.id = a.kabupaten_id`

func (r *anggotaRepo) ListAnggota(ctx context.Context, provinsiID, kabupatenID *int, status, search string, limit, offset int) ([]domain.AnggotaListItem, error) {
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	items := make([]domain.AnggotaListItem, 0)
	where, args := anggotaWhere(provinsiID, kabupatenID, status, search)
	args = append(args, limit, offset)
	query := `SELECT ` + anggotaListColumns + ` ` + anggotaListJoins +
		` WHERE ` + where + ` ORDER BY a.created_at DESC` +
		fmt.Sprintf(` LIMIT $%d OFFSET $%d`, len(args)-1, len(args))
	if err := r.db.SelectContext(ctx, &items, query, args...); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *anggotaRepo) CountAnggota(ctx context.Context, provinsiID, kabupatenID *int, status, search string) (int, error) {
	var total int
	where, args := anggotaWhere(provinsiID, kabupatenID, status, search)
	query := `SELECT COUNT(*) ` + anggotaListJoins + ` WHERE ` + where
	if err := r.db.GetContext(ctx, &total, query, args...); err != nil {
		return 0, err
	}
	return total, nil
}
