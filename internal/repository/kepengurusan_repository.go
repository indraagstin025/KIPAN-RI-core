package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
)

// ============================================================
// JABATAN (MASTER)
// ============================================================

// JabatanRepository mengelola master jabatan struktural.
type JabatanRepository interface {
	List(ctx context.Context, includeInactive bool, level string) ([]domain.Jabatan, error)
	GetByID(ctx context.Context, id int) (*domain.Jabatan, error)
	Create(ctx context.Context, in domain.JabatanRequest) (*domain.Jabatan, error)
	Update(ctx context.Context, id int, in domain.JabatanRequest) (*domain.Jabatan, error)
	// CountPengurus menghitung pengurus yang memakai jabatan ini (untuk
	// mencegah perubahan level jabatan yang sedang dipakai).
	CountPengurus(ctx context.Context, jabatanID int) (int, error)
}

type jabatanRepo struct{ db *sqlx.DB }

func NewJabatanRepository(db *sqlx.DB) JabatanRepository { return &jabatanRepo{db: db} }

const jabatanColumns = `id, nama, level, is_inti, is_active, urutan, created_at, updated_at`

func (r *jabatanRepo) List(ctx context.Context, includeInactive bool, level string) ([]domain.Jabatan, error) {
	items := make([]domain.Jabatan, 0)
	where := []string{"($1 OR is_active)"}
	args := []interface{}{includeInactive}
	if l := strings.TrimSpace(level); l != "" {
		args = append(args, l)
		where = append(where, fmt.Sprintf("level = $%d", len(args)))
	}
	query := `SELECT ` + jabatanColumns + ` FROM jabatan WHERE ` + strings.Join(where, " AND ") + ` ORDER BY urutan, nama`
	if err := r.db.SelectContext(ctx, &items, query, args...); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *jabatanRepo) GetByID(ctx context.Context, id int) (*domain.Jabatan, error) {
	var j domain.Jabatan
	query := `SELECT ` + jabatanColumns + ` FROM jabatan WHERE id = $1`
	if err := r.db.GetContext(ctx, &j, query, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &j, nil
}

func (r *jabatanRepo) Create(ctx context.Context, in domain.JabatanRequest) (*domain.Jabatan, error) {
	var j domain.Jabatan
	query := `INSERT INTO jabatan (nama, level, is_inti, is_active, urutan)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING ` + jabatanColumns
	if err := r.db.GetContext(ctx, &j, query,
		strings.TrimSpace(in.Nama), in.Level, in.IsInti, in.IsActive, in.Urutan); err != nil {
		return nil, mapDBError(err, "Jabatan dengan nama & level tersebut sudah ada")
	}
	return &j, nil
}

func (r *jabatanRepo) Update(ctx context.Context, id int, in domain.JabatanRequest) (*domain.Jabatan, error) {
	var j domain.Jabatan
	query := `UPDATE jabatan
		SET nama = $2, level = $3, is_inti = $4, is_active = $5, urutan = $6, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1
		RETURNING ` + jabatanColumns
	if err := r.db.GetContext(ctx, &j, query,
		id, strings.TrimSpace(in.Nama), in.Level, in.IsInti, in.IsActive, in.Urutan); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, mapDBError(err, "Jabatan dengan nama & level tersebut sudah ada")
	}
	return &j, nil
}

func (r *jabatanRepo) CountPengurus(ctx context.Context, jabatanID int) (int, error) {
	var n int
	if err := r.db.GetContext(ctx, &n, `SELECT COUNT(*) FROM pengurus WHERE jabatan_id = $1`, jabatanID); err != nil {
		return 0, err
	}
	return n, nil
}

// ============================================================
// SURAT KEPUTUSAN
// ============================================================

// SKFilter menyaring daftar SK. ProvinsiID/KabupatenID berasal dari scope
// server-side (bukan klien); nil = tanpa batas (Nasional/Super).
type SKFilter struct {
	Level          string
	ProvinsiID     *int
	KabupatenID    *int
	Status         string
	ApprovalStatus string
	Search         string
	WithTotal      bool
	Limit          int
	Offset         int
}

// SKRepository mengelola dokumen Surat Keputusan.
type SKRepository interface {
	Create(ctx context.Context, sk *domain.SuratKeputusan) (*domain.SuratKeputusan, error)
	GetByID(ctx context.Context, id int) (*domain.SuratKeputusan, error)
	List(ctx context.Context, f SKFilter) ([]domain.SKListItem, int, error)
	UpdateApproval(ctx context.Context, id int, from, to domain.SKApprovalStatus, catatan *string, approvedBy *string, approvedAt *time.Time) error
	// FinalizeSK mengesahkan SK ke DISETUJUI (compare-and-swap) + Single Active SK rule (atomik).
	FinalizeSK(ctx context.Context, id int, from domain.SKApprovalStatus, approvedBy *string, approvedAt *time.Time) error
	// SetStatusWithDemotion mengubah status SK; bila menjadi TidakAktif,
	// pengurus aktif SK tsb didemosi (Demisioner/Diberhentikan) + keterangan.
	SetStatusWithDemotion(ctx context.Context, id int, status domain.SKStatus, pengurusStatus domain.PengurusStatus, keterangan string) error
}

type skRepo struct{ db *sqlx.DB }

func NewSKRepository(db *sqlx.DB) SKRepository { return &skRepo{db: db} }

const skColumns = `id, nomor_sk, judul, level, provinsi_id, kabupaten_id,
	tanggal_terbit, tanggal_berakhir, file_sk_key, status, approval_status,
	catatan_penolakan, created_by, approved_by, approved_at, created_at, updated_at`

const skInsertQuery = `INSERT INTO surat_keputusan
	(nomor_sk, judul, level, provinsi_id, kabupaten_id, tanggal_terbit, tanggal_berakhir,
	 file_sk_key, status, approval_status, created_by)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	RETURNING ` + skColumns

func insertSK(ctx context.Context, q sqlx.ExtContext, sk *domain.SuratKeputusan) (*domain.SuratKeputusan, error) {
	var out domain.SuratKeputusan
	if err := sqlx.GetContext(ctx, q, &out, skInsertQuery,
		sk.NomorSK, sk.Judul, sk.Level, sk.ProvinsiID, sk.KabupatenID,
		sk.TanggalTerbit, sk.TanggalBerakhir, sk.FileSKKey, sk.Status, sk.ApprovalStatus, sk.CreatedBy); err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *skRepo) Create(ctx context.Context, sk *domain.SuratKeputusan) (*domain.SuratKeputusan, error) {
	return insertSK(ctx, r.db, sk)
}

func (r *skRepo) GetByID(ctx context.Context, id int) (*domain.SuratKeputusan, error) {
	var sk domain.SuratKeputusan
	query := `SELECT ` + skColumns + ` FROM surat_keputusan WHERE id = $1`
	if err := r.db.GetContext(ctx, &sk, query, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &sk, nil
}

const skListColumns = `sk.id, sk.nomor_sk, sk.judul, sk.level, sk.provinsi_id, sk.kabupaten_id,
	p.nama AS provinsi_nama, k.nama AS kabupaten_nama, sk.tanggal_terbit, sk.tanggal_berakhir,
	sk.status, sk.approval_status, sk.created_at,
	COALESCE(pg_lat.jumlah, 0) AS jumlah_pengurus`

const skListJoins = `FROM surat_keputusan sk
	LEFT JOIN wilayah_provinsi p ON p.id = sk.provinsi_id
	LEFT JOIN wilayah_kabupaten k ON k.id = sk.kabupaten_id`

// skLateralPengurus menghitung jumlah pengurus aktif per SK via LATERAL
// (satu pemindaian terindeks), menggantikan subquery korelatif per baris.
const skLateralPengurus = `LEFT JOIN LATERAL (
		SELECT COUNT(*) AS jumlah FROM pengurus pg
		WHERE pg.surat_keputusan_id = sk.id AND pg.status = 'Aktif'
	) pg_lat ON TRUE`

func skWhere(f SKFilter) (string, []interface{}) {
	where := []string{"1 = 1"}
	args := []interface{}{}
	if l := strings.TrimSpace(f.Level); l != "" {
		args = append(args, l)
		where = append(where, fmt.Sprintf("sk.level = $%d", len(args)))
	}
	if f.ProvinsiID != nil {
		args = append(args, *f.ProvinsiID)
		where = append(where, fmt.Sprintf("sk.provinsi_id = $%d", len(args)))
	}
	if f.KabupatenID != nil {
		args = append(args, *f.KabupatenID)
		where = append(where, fmt.Sprintf("sk.kabupaten_id = $%d", len(args)))
	}
	if s := strings.TrimSpace(f.Status); s != "" {
		args = append(args, s)
		where = append(where, fmt.Sprintf("sk.status = $%d", len(args)))
	}
	if a := strings.TrimSpace(f.ApprovalStatus); a != "" {
		args = append(args, a)
		where = append(where, fmt.Sprintf("sk.approval_status = $%d", len(args)))
	}
	if q := strings.TrimSpace(f.Search); q != "" {
		args = append(args, "%"+q+"%")
		where = append(where, fmt.Sprintf("(sk.nomor_sk ILIKE $%d OR sk.judul ILIKE $%d)", len(args), len(args)))
	}
	return strings.Join(where, " AND "), args
}

func (r *skRepo) List(ctx context.Context, f SKFilter) ([]domain.SKListItem, int, error) {
	if f.Limit <= 0 {
		f.Limit = 25
	}
	if f.Limit > 100 {
		f.Limit = 100
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
	where, args := skWhere(f)
	var total int
	if f.WithTotal {
		// COUNT tidak butuh join: skWhere hanya menyaring kolom sk.*.
		if err := r.db.GetContext(ctx, &total, `SELECT COUNT(*) FROM surat_keputusan sk WHERE `+where, args...); err != nil {
			return nil, 0, err
		}
	}
	items := make([]domain.SKListItem, 0)
	args = append(args, f.Limit, f.Offset)
	query := `SELECT ` + skListColumns + ` ` + skListJoins + ` ` + skLateralPengurus + ` WHERE ` + where +
		` ORDER BY sk.created_at DESC` + fmt.Sprintf(` LIMIT $%d OFFSET $%d`, len(args)-1, len(args))
	if err := r.db.SelectContext(ctx, &items, query, args...); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// UpdateApproval memakai compare-and-swap pada approval_status (anti balapan).
func (r *skRepo) UpdateApproval(ctx context.Context, id int, from, to domain.SKApprovalStatus, catatan *string, approvedBy *string, approvedAt *time.Time) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE surat_keputusan
		 SET approval_status = $3, catatan_penolakan = $4, approved_by = $5, approved_at = $6, updated_at = CURRENT_TIMESTAMP
		 WHERE id = $1 AND approval_status = $2`,
		id, from, to, catatan, approvedBy, approvedAt)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.NewConflictError("Status SK berubah. Muat ulang lalu coba lagi.")
	}
	return nil
}

// SetStatusWithDemotion mengubah status SK. Bila menjadi TidakAktif, seluruh
// pengurus aktif SK tsb didemosi (status dipilih + keterangan) dalam transaksi
// yang sama, selaras perilaku lama.
func (r *skRepo) SetStatusWithDemotion(ctx context.Context, id int, status domain.SKStatus, pengurusStatus domain.PengurusStatus, keterangan string) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx,
		`UPDATE surat_keputusan SET status = $2, updated_at = CURRENT_TIMESTAMP WHERE id = $1`, id, status)
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
	if status == domain.SKStatusTidakAktif {
		ps := pengurusStatus
		if ps == "" {
			ps = domain.PengurusStatusDemisioner
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE pengurus SET status = $2, keterangan_status = $3, updated_at = CURRENT_TIMESTAMP
			 WHERE surat_keputusan_id = $1 AND status = 'Aktif'`,
			id, string(ps), strings.TrimSpace(keterangan)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// FinalizeSK mengesahkan SK (compare-and-swap ke DISETUJUI) lalu menjalankan
// Single Active SK rule dalam satu transaksi: SK lain selevel+wilayah → TidakAktif
// dan pengurus aktifnya → Demisioner.
func (r *skRepo) FinalizeSK(ctx context.Context, id int, from domain.SKApprovalStatus, approvedBy *string, approvedAt *time.Time) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("gagal memulai transaksi: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx,
		`UPDATE surat_keputusan
		 SET approval_status = $3, approved_by = $4, approved_at = $5, updated_at = CURRENT_TIMESTAMP
		 WHERE id = $1 AND approval_status = $2`,
		id, from, domain.SKApprovalStatusDisetujui, approvedBy, approvedAt)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.NewConflictError("Status SK berubah. Muat ulang lalu coba lagi.")
	}

	var sk domain.SuratKeputusan
	if err := tx.GetContext(ctx, &sk, `SELECT `+skColumns+` FROM surat_keputusan WHERE id = $1`, id); err != nil {
		return err
	}
	if err := deactivatePriorActiveSKsTx(ctx, tx, &sk); err != nil {
		return err
	}
	return tx.Commit()
}

// priorActiveSKWhere membangun klausa WHERE untuk SK aktif lain selevel+wilayah
// (mengecualikan SK saat ini). start = indeks placeholder pertama.
func priorActiveSKWhere(sk *domain.SuratKeputusan, start int) (string, []interface{}, bool) {
	args := []interface{}{sk.ID, string(sk.Level)}
	where := fmt.Sprintf("id <> $%d AND status = 'Aktif' AND level = $%d", start, start+1)
	n := start + 2
	switch sk.Level {
	case domain.LevelProvinsi:
		if sk.ProvinsiID == nil {
			return "", nil, false
		}
		where += fmt.Sprintf(" AND provinsi_id = $%d", n)
		args = append(args, *sk.ProvinsiID)
	case domain.LevelKabupaten:
		if sk.ProvinsiID == nil || sk.KabupatenID == nil {
			return "", nil, false
		}
		where += fmt.Sprintf(" AND provinsi_id = $%d AND kabupaten_id = $%d", n, n+1)
		args = append(args, *sk.ProvinsiID, *sk.KabupatenID)
	case domain.LevelNasional:
		where += " AND provinsi_id IS NULL AND kabupaten_id IS NULL"
	default:
		return "", nil, false
	}
	return where, args, true
}

// deactivatePriorActiveSKsTx menonaktifkan SK aktif lama selevel+wilayah dan
// mendemisionerkan pengurus aktifnya (dipakai saat SK final).
func deactivatePriorActiveSKsTx(ctx context.Context, tx *sqlx.Tx, sk *domain.SuratKeputusan) error {
	where, args, ok := priorActiveSKWhere(sk, 1)
	if !ok {
		return nil
	}
	// Ambil ID dulu SEBELUM menonaktifkan, agar daftar tidak berubah akibat
	// status yang baru saja di-update.
	var ids []int
	if err := tx.SelectContext(ctx, &ids, `SELECT id FROM surat_keputusan WHERE `+where, args...); err != nil {
		return fmt.Errorf("gagal membaca SK lama: %w", err)
	}
	if len(ids) == 0 {
		return nil
	}
	ph1 := make([]string, len(ids))
	idArgs := make([]interface{}, len(ids))
	for i, id := range ids {
		ph1[i] = fmt.Sprintf("$%d", i+1)
		idArgs[i] = id
	}
	in1 := strings.Join(ph1, ",")
	if _, err := tx.ExecContext(ctx,
		`UPDATE surat_keputusan SET status = 'TidakAktif', updated_at = CURRENT_TIMESTAMP WHERE id IN (`+in1+`)`,
		idArgs...); err != nil {
		return fmt.Errorf("gagal menonaktifkan SK lama: %w", err)
	}

	ph2 := make([]string, len(ids))
	for i := range ids {
		ph2[i] = fmt.Sprintf("$%d", i+3) // $1=msg, $2=tanggal
	}
	in2 := strings.Join(ph2, ",")
	msg := "Otomatis demisioner karena SK baru " + sk.NomorSK + " telah disetujui"
	pArgs := append([]interface{}{msg, sk.TanggalTerbit}, idArgs...)
	if _, err := tx.ExecContext(ctx,
		`UPDATE pengurus
		 SET status = 'Demisioner', keterangan_status = $1, tanggal_selesai = $2, updated_at = CURRENT_TIMESTAMP
		 WHERE status = 'Aktif' AND surat_keputusan_id IN (`+in2+`)`,
		pArgs...); err != nil {
		return fmt.Errorf("gagal mendemisionerkan pengurus SK lama: %w", err)
	}
	return nil
}

// ============================================================
// PENGURUS
// ============================================================
// PengurusFilter menyaring daftar pengurus (scope server-side).
type PengurusFilter struct {
	ProvinsiID  *int
	KabupatenID *int
	Level       string
	Status      string
	MasaJabatan string
	Search      string
	WithTotal   bool
	Limit       int
	Offset      int
}

// PromoteInput data pengangkatan anggota menjadi pengurus dalam satu transaksi.
type PromoteInput struct {
	AnggotaID      int
	SKID           int
	UserID         string
	Level          string
	ProvinsiID     *int
	KabupatenID    *int
	JabatanID      int
	TanggalMulai   time.Time
	TanggalSelesai time.Time
	Keterangan     string
}

// PengurusRepository (agregat) = tulis + baca. Disusun dari interface kecil (ISP).
type PengurusRepository interface {
	PengurusWriteRepository
	PengurusQueryRepository
}

// PengurusWriteRepository — mutasi kepengurusan.
type PengurusWriteRepository interface {
	AddWithPromotion(ctx context.Context, in PromoteInput) (int, error)
	Remove(ctx context.Context, pengurusID int) error
	UpdateStatus(ctx context.Context, id int, status domain.PengurusStatus, keterangan string) error
	UpdateJabatan(ctx context.Context, id int, jabatanID int) error
}

// PengurusQueryRepository — pembacaan & validasi kepengurusan.
type PengurusQueryRepository interface {
	GetByID(ctx context.Context, id int) (*domain.PengurusDetail, error)
	ListBySK(ctx context.Context, skID int) ([]domain.PengurusDetail, error)
	List(ctx context.Context, f PengurusFilter) ([]domain.PengurusDetail, int, error)
	Stats(ctx context.Context, f PengurusFilter) (domain.PengurusStats, error)
	ExistsInSK(ctx context.Context, skID, anggotaID int) (bool, error)
	CountJabatanInSK(ctx context.Context, skID, jabatanID, excludeAnggotaID int) (int, error)
	// ListPromosi kandidat promosi: anggota AKTIF ber-riwayat pengurus yang
	// TIDAK sedang aktif menjabat (ter-scope). Untuk mode "Promosi Pengurus".
	ListPromosi(ctx context.Context, provinsiID, kabupatenID *int, search string, limit int) ([]domain.PromosiCandidate, error)
}

type pengurusRepo struct{ db *sqlx.DB }

func NewPengurusRepository(db *sqlx.DB) PengurusRepository { return &pengurusRepo{db: db} }

const pengurusDetailColumns = `p.id, p.anggota_id, a.nia, a.nama_lengkap,
	p.surat_keputusan_id, sk.nomor_sk, p.jabatan_id, j.nama AS jabatan, j.is_inti,
	p.level, p.provinsi_id, p.kabupaten_id, wp.nama AS provinsi_nama, wk.nama AS kabupaten_nama,
	p.status, p.keterangan_status,
	sk.tanggal_berakhir AS sk_tanggal_berakhir,
	p.tanggal_mulai, p.tanggal_selesai, p.created_at`

const pengurusDetailJoins = `FROM pengurus p
	JOIN anggota a ON a.id = p.anggota_id
	JOIN jabatan j ON j.id = p.jabatan_id
	JOIN surat_keputusan sk ON sk.id = p.surat_keputusan_id
	LEFT JOIN wilayah_provinsi wp ON wp.id = p.provinsi_id
	LEFT JOIN wilayah_kabupaten wk ON wk.id = p.kabupaten_id`

// AddWithPromotion mengeksekusi pengangkatan secara ATOMIK:
// demote pengurus Aktif lama, sisipkan pengurus baru, flip anggota.tipe +
// users.tipe_user ke PENGURUS, dan cabut seluruh refresh token anggota.
func (r *pengurusRepo) AddWithPromotion(ctx context.Context, in PromoteInput) (int, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("gagal memulai transaksi: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx,
		`UPDATE pengurus
		 SET status = $1, keterangan_status = $2, tanggal_selesai = $3, updated_at = CURRENT_TIMESTAMP
		 WHERE anggota_id = $4 AND status = 'Aktif'`,
		string(domain.PengurusStatusDemisioner), in.Keterangan, in.TanggalSelesai, in.AnggotaID); err != nil {
		return 0, fmt.Errorf("gagal menonaktifkan pengurus lama: %w", err)
	}

	var newID int
	if err := tx.QueryRowxContext(ctx,
		`INSERT INTO pengurus
			(anggota_id, surat_keputusan_id, level, provinsi_id, kabupaten_id, jabatan_id, status, tanggal_mulai)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id`,
		in.AnggotaID, in.SKID, in.Level, in.ProvinsiID, in.KabupatenID, in.JabatanID,
		string(domain.PengurusStatusAktif), in.TanggalMulai).Scan(&newID); err != nil {
		return 0, fmt.Errorf("gagal menyimpan pengurus: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE anggota SET tipe = $2, updated_at = CURRENT_TIMESTAMP WHERE id = $1`,
		in.AnggotaID, string(domain.TipePendaftaranPengurus)); err != nil {
		return 0, fmt.Errorf("gagal memperbarui tipe anggota: %w", err)
	}

	if strings.TrimSpace(in.UserID) != "" {
		if _, err := tx.ExecContext(ctx,
			`UPDATE users SET tipe_user = $2, updated_at = CURRENT_TIMESTAMP WHERE id = $1`,
			in.UserID, string(domain.UserTipePengurus)); err != nil {
			return 0, fmt.Errorf("gagal memperbarui tipe akun: %w", err)
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE user_refresh_tokens SET is_revoked = TRUE WHERE user_id = $1 AND is_revoked = FALSE`,
			in.UserID); err != nil {
			return 0, fmt.Errorf("gagal mencabut sesi anggota: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return newID, nil
}

func (r *pengurusRepo) Remove(ctx context.Context, pengurusID int) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM pengurus WHERE id = $1`, pengurusID)
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

func (r *pengurusRepo) GetByID(ctx context.Context, id int) (*domain.PengurusDetail, error) {
	var p domain.PengurusDetail
	query := `SELECT ` + pengurusDetailColumns + ` ` + pengurusDetailJoins + ` WHERE p.id = $1`
	if err := r.db.GetContext(ctx, &p, query, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (r *pengurusRepo) ListBySK(ctx context.Context, skID int) ([]domain.PengurusDetail, error) {
	items := make([]domain.PengurusDetail, 0)
	query := `SELECT ` + pengurusDetailColumns + ` ` + pengurusDetailJoins +
		` WHERE p.surat_keputusan_id = $1 ORDER BY j.urutan, a.nama_lengkap`
	if err := r.db.SelectContext(ctx, &items, query, skID); err != nil {
		return nil, err
	}
	return items, nil
}

func pengurusWhere(f PengurusFilter) (string, []interface{}) {
	where := []string{"1 = 1"}
	args := []interface{}{}
	if f.ProvinsiID != nil {
		args = append(args, *f.ProvinsiID)
		where = append(where, fmt.Sprintf("p.provinsi_id = $%d", len(args)))
	}
	if f.KabupatenID != nil {
		args = append(args, *f.KabupatenID)
		where = append(where, fmt.Sprintf("p.kabupaten_id = $%d", len(args)))
	}
	if s := strings.TrimSpace(f.Status); s != "" {
		args = append(args, s)
		where = append(where, fmt.Sprintf("p.status = $%d", len(args)))
	}
	if l := strings.TrimSpace(f.Level); l != "" {
		args = append(args, l)
		where = append(where, fmt.Sprintf("p.level = $%d", len(args)))
	}
	if m := strings.TrimSpace(f.MasaJabatan); m != "" {
		switch m {
		case "Aktif":
			where = append(where, "(sk.tanggal_berakhir IS NULL OR sk.tanggal_berakhir >= CURRENT_DATE)")
		case "AkanBerakhir":
			where = append(where, "(sk.tanggal_berakhir IS NOT NULL AND sk.tanggal_berakhir BETWEEN CURRENT_DATE AND CURRENT_DATE + INTERVAL '90 days')")
		case "Berakhir":
			where = append(where, "(sk.tanggal_berakhir IS NOT NULL AND sk.tanggal_berakhir < CURRENT_DATE)")
		}
	}
	if q := strings.TrimSpace(f.Search); q != "" {
		args = append(args, "%"+q+"%")
		where = append(where, fmt.Sprintf("(a.nama_lengkap ILIKE $%d OR a.nia ILIKE $%d)", len(args), len(args)))
	}
	return strings.Join(where, " AND "), args
}

func (r *pengurusRepo) List(ctx context.Context, f PengurusFilter) ([]domain.PengurusDetail, int, error) {
	if f.Limit <= 0 {
		f.Limit = 25
	}
	if f.Limit > 100 {
		f.Limit = 100
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
	where, args := pengurusWhere(f)
	var total int
	if f.WithTotal {
		if err := r.db.GetContext(ctx, &total, `SELECT COUNT(*) `+pengurusDetailJoins+` WHERE `+where, args...); err != nil {
			return nil, 0, err
		}
	}
	items := make([]domain.PengurusDetail, 0)
	args = append(args, f.Limit, f.Offset)
	query := `SELECT ` + pengurusDetailColumns + ` ` + pengurusDetailJoins + ` WHERE ` + where +
		` ORDER BY p.created_at DESC` + fmt.Sprintf(` LIMIT $%d OFFSET $%d`, len(args)-1, len(args))
	if err := r.db.SelectContext(ctx, &items, query, args...); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// Stats menghitung ringkasan pengurus AKTIF (ter-scope) untuk kartu dasbor.
func (r *pengurusRepo) Stats(ctx context.Context, f PengurusFilter) (domain.PengurusStats, error) {
	var out domain.PengurusStats
	args := []interface{}{}
	where := []string{"p.status = 'Aktif'"}
	if f.ProvinsiID != nil {
		args = append(args, *f.ProvinsiID)
		where = append(where, fmt.Sprintf("p.provinsi_id = $%d", len(args)))
	}
	if f.KabupatenID != nil {
		args = append(args, *f.KabupatenID)
		where = append(where, fmt.Sprintf("p.kabupaten_id = $%d", len(args)))
	}
	query := `SELECT
		COUNT(*) AS total,
		COUNT(*) FILTER (WHERE p.level = 'NASIONAL') AS nasional,
		COUNT(*) FILTER (WHERE p.level = 'PROVINSI') AS provinsi,
		COUNT(*) FILTER (WHERE p.level = 'KABUPATEN') AS kabupaten,
		COUNT(*) FILTER (WHERE sk.tanggal_berakhir IS NOT NULL AND sk.tanggal_berakhir BETWEEN CURRENT_DATE AND CURRENT_DATE + INTERVAL '90 days') AS akan_berakhir
		` + pengurusDetailJoins + ` WHERE ` + strings.Join(where, " AND ")
	if err := r.db.GetContext(ctx, &out, query, args...); err != nil {
		return domain.PengurusStats{}, err
	}
	return out, nil
}

func (r *pengurusRepo) UpdateStatus(ctx context.Context, id int, status domain.PengurusStatus, keterangan string) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE pengurus SET status = $2, keterangan_status = $3, updated_at = CURRENT_TIMESTAMP WHERE id = $1`,
		id, string(status), strings.TrimSpace(keterangan))
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

func (r *pengurusRepo) UpdateJabatan(ctx context.Context, id int, jabatanID int) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE pengurus SET jabatan_id = $2, updated_at = CURRENT_TIMESTAMP WHERE id = $1`,
		id, jabatanID)
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

func (r *pengurusRepo) ExistsInSK(ctx context.Context, skID, anggotaID int) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM pengurus WHERE surat_keputusan_id = $1 AND anggota_id = $2)`
	if err := r.db.GetContext(ctx, &exists, query, skID, anggotaID); err != nil {
		return false, err
	}
	return exists, nil
}

// CountJabatanInSK menghitung pemegang jabatan (selain anggota tertentu) pada SK.
// Dipakai menegakkan aturan jabatan inti tunggal.
func (r *pengurusRepo) CountJabatanInSK(ctx context.Context, skID, jabatanID, excludeAnggotaID int) (int, error) {
	var n int
	query := `SELECT COUNT(*) FROM pengurus
		WHERE surat_keputusan_id = $1 AND jabatan_id = $2 AND anggota_id <> $3`
	if err := r.db.GetContext(ctx, &n, query, skID, jabatanID, excludeAnggotaID); err != nil {
		return 0, err
	}
	return n, nil
}

// ListPromosi mengembalikan kandidat promosi (anggota AKTIF ber-riwayat
// pengurus, tidak sedang aktif menjabat) dengan jabatan/level terakhir.
func (r *pengurusRepo) ListPromosi(ctx context.Context, provinsiID, kabupatenID *int, search string, limit int) ([]domain.PromosiCandidate, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	where := []string{"a.status = 'Aktif'",
		"NOT EXISTS (SELECT 1 FROM pengurus p2 WHERE p2.anggota_id = a.id AND p2.status = 'Aktif')"}
	args := []interface{}{}
	if provinsiID != nil {
		args = append(args, *provinsiID)
		where = append(where, fmt.Sprintf("a.provinsi_id = $%d", len(args)))
	}
	if kabupatenID != nil {
		args = append(args, *kabupatenID)
		where = append(where, fmt.Sprintf("a.kabupaten_id = $%d", len(args)))
	}
	if q := strings.TrimSpace(search); q != "" {
		args = append(args, "%"+q+"%")
		where = append(where, fmt.Sprintf("(a.nama_lengkap ILIKE $%d OR a.nia ILIKE $%d)", len(args), len(args)))
	}
	args = append(args, limit)
	items := make([]domain.PromosiCandidate, 0)
	query := `SELECT a.id AS anggota_id, a.nia, a.nama_lengkap, a.provinsi_id, a.kabupaten_id,
		wp.nama AS provinsi_nama, wk.nama AS kabupaten_nama,
		last.jabatan, last.level, last.status
		FROM anggota a
		LEFT JOIN wilayah_provinsi wp ON wp.id = a.provinsi_id
		LEFT JOIN wilayah_kabupaten wk ON wk.id = a.kabupaten_id
		JOIN LATERAL (
			SELECT j.nama AS jabatan, p.level, p.status
			FROM pengurus p JOIN jabatan j ON j.id = p.jabatan_id
			WHERE p.anggota_id = a.id
			ORDER BY p.created_at DESC LIMIT 1
		) last ON TRUE
		WHERE ` + strings.Join(where, " AND ") + ` ORDER BY a.nama_lengkap` +
		fmt.Sprintf(` LIMIT $%d`, len(args))
	if err := r.db.SelectContext(ctx, &items, query, args...); err != nil {
		return nil, err
	}
	return items, nil
}
