package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/nia"
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
	// SetUserID menghubungkan anggota ke akun USER (dibuat saat approve).
	SetUserID(ctx context.Context, id int, userID string) error
	// GetByUserID mengambil anggota milik akun USER (layanan mandiri).
	GetByUserID(ctx context.Context, userID string) (*domain.Anggota, error)
	// SetStatus mengubah status keanggotaan (mis. MENINGGAL saat PAW).
	SetStatus(ctx context.Context, id int, status domain.AnggotaStatus) error
	// RiwayatByAnggotaIDs menghitung kolom RIWAYAT (TDD §5.5) per anggota.
	RiwayatByAnggotaIDs(ctx context.Context, ids []int) (map[int]string, error)
	// LatestPendaftaranAksiByAnggotaIDs mengembalikan aksi pendaftaran
	// TERAKHIR per anggota (fallback kolom RIWAYAT bila tanpa riwayat
	// kepengurusan). Anggota tanpa pendaftaran tidak ada di map.
	LatestPendaftaranAksiByAnggotaIDs(ctx context.Context, ids []int) (map[int]string, error)
	// AllocateNIA mengalokasikan NIA baru (KIPAN-IND-...) secara atomik.
	AllocateNIA(ctx context.Context, provinsiID, kabupatenID, year int) (string, error)
	// Create menyimpan anggota baru (NIA/NIK telah disiapkan service).
	Create(ctx context.Context, a *domain.Anggota) (*domain.Anggota, error)
	// Update menyimpan perubahan data anggota (NIK/NIA tidak diubah).
	Update(ctx context.Context, a *domain.Anggota) error
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
	surat_sehat_key, status, tipe, angkatan, kta_qr_hash, kta_pdf_key,
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

// SetUserID menghubungkan anggota ke akun USER. Idempoten (update biasa).
func (r *anggotaRepo) SetUserID(ctx context.Context, id int, userID string) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE anggota SET user_id = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`,
		strings.TrimSpace(userID), id)
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

// SetStatus mengubah status keanggotaan (mis. MENINGGAL saat PAW meninggal).
func (r *anggotaRepo) SetStatus(ctx context.Context, id int, status domain.AnggotaStatus) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE anggota SET status = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`,
		string(status), id)
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

// RiwayatByAnggotaIDs menghitung teks kolom RIWAYAT untuk sekumpulan anggota
// (TDD §5.5) memakai status efektif (TDD §5.4). Anggota tanpa riwayat = "-".
func (r *anggotaRepo) RiwayatByAnggotaIDs(ctx context.Context, ids []int) (map[int]string, error) {
	out := make(map[int]string, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	query, args, err := sqlx.In(`
		SELECT p.anggota_id, p.level, j.nama AS jabatan,
		       wp.nama AS provinsi_nama, wk.nama AS kabupaten_nama,
		       sk.tanggal_terbit, sk.tanggal_berakhir,
		       pengurus_status_efektif(p.status, sk.status, sk.tanggal_berakhir, CURRENT_DATE) AS status_efektif,
		       p.tanggal_mulai
		FROM pengurus p
		JOIN surat_keputusan sk ON sk.id = p.surat_keputusan_id
		JOIN jabatan j ON j.id = p.jabatan_id
		LEFT JOIN wilayah_provinsi wp ON wp.id = p.provinsi_id
		LEFT JOIN wilayah_kabupaten wk ON wk.id = p.kabupaten_id
		WHERE p.anggota_id IN (?)`, ids)
	if err != nil {
		return nil, err
	}
	query = r.db.Rebind(query)
	rows := make([]domain.PengurusRiwayatRow, 0)
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, err
	}
	grouped := make(map[int][]domain.PengurusRiwayatRow)
	for i := range rows {
		grouped[rows[i].AnggotaID] = append(grouped[rows[i].AnggotaID], rows[i])
	}
	for _, id := range ids {
		if rs, ok := grouped[id]; ok {
			out[id] = domain.FormatRiwayat(rs)
		} else {
			out[id] = "-"
		}
	}
	return out, nil
}

// LatestPendaftaranAksiByAnggotaIDs mengambil aksi pendaftaran terakhir per
// anggota (satu baris terbaru dari pendaftaran_riwayat via pendaftaran_id
// anggota). Dipakai sebagai fallback kolom RIWAYAT daftar.
func (r *anggotaRepo) LatestPendaftaranAksiByAnggotaIDs(ctx context.Context, ids []int) (map[int]string, error) {
	out := make(map[int]string, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	query, args, err := sqlx.In(`
		SELECT DISTINCT ON (a.id) a.id AS anggota_id, pr.aksi
		FROM anggota a
		JOIN pendaftaran_riwayat pr ON pr.pendaftaran_id = a.pendaftaran_id
		WHERE a.id IN (?)
		ORDER BY a.id, pr.created_at DESC, pr.id DESC`, ids)
	if err != nil {
		return nil, err
	}
	query = r.db.Rebind(query)
	rows := make([]struct {
		AnggotaID int    `db:"anggota_id"`
		Aksi      string `db:"aksi"`
	}, 0)
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.AnggotaID] = row.Aksi
	}
	return out, nil
}

// AllocateNIA mengalokasikan NIA baru: kode kabupaten (BPS) + sequence per
// kabupaten/tahun secara atomik (UPSERT ... RETURNING). Aman terhadap race.
func (r *anggotaRepo) AllocateNIA(ctx context.Context, provinsiID, kabupatenID, year int) (string, error) {
	var kabKode string
	if err := r.db.GetContext(ctx, &kabKode, `SELECT kode FROM wilayah_kabupaten WHERE id = $1`, kabupatenID); err != nil {
		return "", fmt.Errorf("gagal mengambil kode kabupaten: %w", err)
	}
	var seq int
	if err := r.db.GetContext(ctx, &seq, `
		INSERT INTO anggota_nia_sequence (provinsi_id, kabupaten_id, tahun, next_value)
		VALUES ($1, $2, $3, 2)
		ON CONFLICT (provinsi_id, kabupaten_id, tahun)
		DO UPDATE SET next_value = anggota_nia_sequence.next_value + 1
		RETURNING next_value - 1`,
		provinsiID, kabupatenID, year); err != nil {
		return "", fmt.Errorf("gagal menghasilkan sequence NIA: %w", err)
	}
	return nia.GenerateNIA(kabKode, year, seq)
}

// Create menyimpan anggota baru (di luar alur pendaftaran) dan mengembalikan
// baris tersimpan. NIK/NIA duplikat dipetakan ke 409.
func (r *anggotaRepo) Create(ctx context.Context, a *domain.Anggota) (*domain.Anggota, error) {
	var out domain.Anggota
	query := `INSERT INTO anggota (
		nia, nama_lengkap, nik_hash, nik_encrypted, tempat_lahir, tanggal_lahir,
		jenis_kelamin, agama, pendidikan, pekerjaan, alamat, provinsi_id, kabupaten_id,
		kecamatan, desa, kode_pos, email, whatsapp,
		foto_key, ktp_key, cv_key, sk_key, surat_pernyataan_key, surat_sehat_key,
		status, tipe, angkatan,
		tanggal_daftar, tanggal_angkat, created_at, updated_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,
		$19,$20,$21,$22,$23,$24,$25,$26,$27,
		CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	RETURNING ` + anggotaColumns
	if err := r.db.GetContext(ctx, &out, query,
		a.NIA, strings.TrimSpace(a.NamaLengkap), a.NIKHash, a.NIKEncrypted, strings.TrimSpace(a.TempatLahir),
		a.TanggalLahir, strings.TrimSpace(a.JenisKelamin), a.Agama, a.Pendidikan, a.Pekerjaan,
		a.Alamat, a.ProvinsiID, a.KabupatenID, a.Kecamatan, a.Desa, a.KodePos, a.Email,
		a.Whatsapp, a.FotoKey, a.KTPKey, a.CVKey, a.SKKey, a.SuratPernyataanKey, a.SuratSehatKey,
		a.Status, a.Tipe, a.Angkatan); err != nil {
		return nil, mapDBError(err, "Data anggota sudah terdaftar (NIK/NIA duplikat)")
	}
	return &out, nil
}

// Update menyimpan perubahan data anggota (NIK/NIA identitas tidak diubah).
func (r *anggotaRepo) Update(ctx context.Context, a *domain.Anggota) error {
	res, err := r.db.ExecContext(ctx, `UPDATE anggota SET
		nama_lengkap = $2, tempat_lahir = $3, tanggal_lahir = $4, jenis_kelamin = $5,
		agama = $6, pendidikan = $7, pekerjaan = $8, alamat = $9, provinsi_id = $10,
		kabupaten_id = $11, kecamatan = $12, desa = $13, kode_pos = $14, email = $15,
		whatsapp = $16, angkatan = $17, status = $18, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1`,
		a.ID, strings.TrimSpace(a.NamaLengkap), strings.TrimSpace(a.TempatLahir), a.TanggalLahir,
		strings.TrimSpace(a.JenisKelamin), a.Agama, a.Pendidikan, a.Pekerjaan, a.Alamat,
		a.ProvinsiID, a.KabupatenID, a.Kecamatan, a.Desa, a.KodePos, a.Email, a.Whatsapp,
		a.Angkatan, a.Status)
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

// GetByUserID mengambil anggota milik satu akun USER (satu user = satu kader).
func (r *anggotaRepo) GetByUserID(ctx context.Context, userID string) (*domain.Anggota, error) {
	var a domain.Anggota
	query := `SELECT ` + anggotaColumns + ` FROM anggota WHERE user_id = $1`
	if err := r.db.GetContext(ctx, &a, query, strings.TrimSpace(userID)); err != nil {
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

const anggotaListColumns = `a.id, a.nia, a.nama_lengkap, a.status, a.pekerjaan,
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
