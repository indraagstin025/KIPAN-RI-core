package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/crypto"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/nia"
)

// PendaftaranRepository menangani persistence data pendaftaran calon anggota.
type PendaftaranRepository interface {
	Create(ctx context.Context, p *domain.Pendaftaran) error
	// CreateWithHistory menyimpan pendaftaran + riwayat SUBMIT dalam satu
	// transaksi dan mengisi p.ID dari RETURNING id.
	CreateWithHistory(ctx context.Context, p *domain.Pendaftaran, aksi, catatan string) error
	// NextRegistrationSequence mengalokasikan nomor urut periode (tahun,
	// bulan) secara atomik via UPSERT ... RETURNING. Aman terhadap race.
	NextRegistrationSequence(ctx context.Context, year, month int) (int, error)
	GetByID(ctx context.Context, id int) (*domain.Pendaftaran, error)
	GetByNomorPendaftaran(ctx context.Context, nomor string) (*domain.Pendaftaran, error)
	GetByNikHash(ctx context.Context, nikHash string) (*domain.Pendaftaran, error)
	// ListHistory mengambil riwayat kronologis untuk timeline publik:
	// hanya aksi + catatan + waktu (tanpa identitas aktor).
	ListHistory(ctx context.Context, pendaftaranID int) ([]domain.PendaftaranRiwayat, error)
	// ListQueue mengambil antrean terfilter wilayah + status dengan
	// proyeksi kolom non-PII. limit dibatasi 1-100 oleh implementasi.
	ListQueue(ctx context.Context, provinsiID, kabupatenID *int, status string, limit, offset int) ([]domain.PendaftaranQueueItem, error)
	// CountQueue menghitung total baris filter yang sama untuk meta pagination.
	CountQueue(ctx context.Context, provinsiID, kabupatenID *int, status string) (int, error)
	// SetRevisiToken menyimpan hash token revisi + expiry (token mentah
	// tidak pernah disimpan).
	SetRevisiToken(ctx context.Context, id int, tokenHash string, expiresAt time.Time) error
	// SubmitRevisionTx mengganti dokumen + status DRAFT + hapus token
	// dalam satu transaksi. rows==0 berarti token salah/kedaluwarsa atau
	// state bukan PERBAIKAN (tanpa oracle: satu error generik).
	SubmitRevisionTx(ctx context.Context, id int, tokenHash string, keys map[string]string, catatan string) error
	UpdateStatus(ctx context.Context, id int, status domain.PendaftaranStatus, catatan string) error
	AppendHistory(ctx context.Context, pendaftaranID int, aksi string, actorID, actorName, actorRole *string, catatan string) error
	// UpdateStatusWithHistory mengubah status + mencatat riwayat beraktor
	// dalam satu transaksi (tidak ada riwayat yatim).
	UpdateStatusWithHistory(ctx context.Context, id int, status domain.PendaftaranStatus, aksi string, actorID, actorName, actorRole *string, catatan string) error
	// IssueMember menerbitkan anggota + NIA + signature KTA dalam satu
	// transaksi. ktaKey adalah KTA_SIGNING_KEY dari config (dipasok service).
	IssueMember(ctx context.Context, pendaftaranID int, year int, ktaKey string) (*domain.Anggota, error)
	// ExpireStaleDrafts menandai pendaftaran DRAFT yang lebih lama dari
	// olderThanDays hari menjadi KEDALUWARSA (lazy-on-access).
	ExpireStaleDrafts(ctx context.Context, olderThanDays int) error
}

// mapDBError memetakan error Postgres ke domain error yang tepat agar klien
// menerima 409/422, bukan 500 generik. Detail SQL mentah tidak diteruskan.
func mapDBError(err error, conflictMsg string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return domain.NewConflictError(conflictMsg)
	}
	return err
}

type pendaftaranRepo struct {
	db *sqlx.DB
}

func NewPendaftaranRepository(db *sqlx.DB) PendaftaranRepository {
	return &pendaftaranRepo{db: db}
}

// ExpireStaleDrafts menandai DRAFT kedaluwarsa (lazy-on-access).
func (r *pendaftaranRepo) ExpireStaleDrafts(ctx context.Context, olderThanDays int) error {
	if olderThanDays <= 0 {
		olderThanDays = 30
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE pendaftaran
		 SET status = $1, updated_at = CURRENT_TIMESTAMP
		 WHERE status = $2 AND created_at < CURRENT_TIMESTAMP - ($3 * INTERVAL '1 day')`,
		domain.PendaftaranStatusKedaluwarsa, domain.PendaftaranStatusDraft, olderThanDays)
	return err
}

func (r *pendaftaranRepo) Create(ctx context.Context, p *domain.Pendaftaran) error {
	query := `
		INSERT INTO pendaftaran (
			nomor_pendaftaran,
			nama_lengkap,
			nik_hash,
			nik_encrypted,
			tempat_lahir,
			tanggal_lahir,
			jenis_kelamin,
			agama,
			pendidikan,
			pekerjaan,
			status_pribadi,
			alamat,
			provinsi_id,
			kabupaten_id,
			kecamatan,
			desa,
			kode_pos,
			email,
			whatsapp,
			motivasi,
			persyaratan_checklist,
			foto_key,
			ktp_key,
			cv_key,
			sk_key,
			surat_pernyataan_key,
			surat_sehat_key,
			status,
			tipe_pendaftaran,
			catatan_perbaikan,
			revisi_token_hash,
			revisi_token_expires_at,
			created_at,
			updated_at
		) VALUES (
			:nomor_pendaftaran,
			:nama_lengkap,
			:nik_hash,
			:nik_encrypted,
			:tempat_lahir,
			:tanggal_lahir,
			:jenis_kelamin,
			:agama,
			:pendidikan,
			:pekerjaan,
			:status_pribadi,
			:alamat,
			:provinsi_id,
			:kabupaten_id,
			:kecamatan,
			:desa,
			:kode_pos,
			:email,
			:whatsapp,
			:motivasi,
			:persyaratan_checklist,
			:foto_key,
			:ktp_key,
			:cv_key,
			:sk_key,
			:surat_pernyataan_key,
			:surat_sehat_key,
			:status,
			:tipe_pendaftaran,
			:catatan_perbaikan,
			:revisi_token_hash,
			:revisi_token_expires_at,
			CURRENT_TIMESTAMP,
			CURRENT_TIMESTAMP
		)`
	_, err := r.db.NamedExecContext(ctx, query, p)
	if err != nil {
		return mapDBError(
			fmt.Errorf("gagal menyimpan pendaftaran: %w", err),
			"Data pendaftaran sudah terdaftar (NIK/nomor duplikat)")
	}
	return nil
}

// CreateWithHistory menyimpan pendaftaran baru beserta riwayat pertamanya
// (aksi SUBMIT sistem) dalam satu transaksi, dan mengisi p.ID.
func (r *pendaftaranRepo) CreateWithHistory(ctx context.Context, p *domain.Pendaftaran, aksi, catatan string) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("gagal memulai transaksi pendaftaran: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	insertQuery := `
		INSERT INTO pendaftaran (
			nomor_pendaftaran,
			nama_lengkap,
			nik_hash,
			nik_encrypted,
			tempat_lahir,
			tanggal_lahir,
			jenis_kelamin,
			agama,
			pendidikan,
			pekerjaan,
			status_pribadi,
			alamat,
			provinsi_id,
			kabupaten_id,
			kecamatan,
			desa,
			kode_pos,
			email,
			whatsapp,
			motivasi,
			persyaratan_checklist,
			foto_key,
			ktp_key,
			cv_key,
			sk_key,
			surat_pernyataan_key,
			surat_sehat_key,
			status,
			tipe_pendaftaran,
			catatan_perbaikan,
			revisi_token_hash,
			revisi_token_expires_at,
			created_at,
			updated_at
		) VALUES (
			:nomor_pendaftaran,
			:nama_lengkap,
			:nik_hash,
			:nik_encrypted,
			:tempat_lahir,
			:tanggal_lahir,
			:jenis_kelamin,
			:agama,
			:pendidikan,
			:pekerjaan,
			:status_pribadi,
			:alamat,
			:provinsi_id,
			:kabupaten_id,
			:kecamatan,
			:desa,
			:kode_pos,
			:email,
			:whatsapp,
			:motivasi,
			:persyaratan_checklist,
			:foto_key,
			:ktp_key,
			:cv_key,
			:sk_key,
			:surat_pernyataan_key,
			:surat_sehat_key,
			:status,
			:tipe_pendaftaran,
			:catatan_perbaikan,
			:revisi_token_hash,
			:revisi_token_expires_at,
			CURRENT_TIMESTAMP,
			CURRENT_TIMESTAMP
		) RETURNING id`

	stmt, err := tx.PrepareNamedContext(ctx, insertQuery)
	if err != nil {
		return fmt.Errorf("gagal menyiapkan insert pendaftaran: %w", err)
	}
	defer stmt.Close()

	if err := stmt.GetContext(ctx, &p.ID, p); err != nil {
		return mapDBError(
			fmt.Errorf("gagal menyimpan pendaftaran: %w", err),
			"Data pendaftaran sudah terdaftar (NIK/nomor duplikat)")
	}

	historyQuery := `
		INSERT INTO pendaftaran_riwayat (
			pendaftaran_id, aksi, catatan, created_at
		) VALUES ($1, $2, $3, CURRENT_TIMESTAMP)`
	if _, err := tx.ExecContext(ctx, historyQuery, p.ID, aksi, catatan); err != nil {
		return fmt.Errorf("gagal mencatat riwayat pendaftaran: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("gagal menyelesaikan transaksi pendaftaran: %w", err)
	}
	return nil
}

// NextRegistrationSequence mengalokasikan nomor urut periode secara atomik.
// Baris sequence dibuat saat pertama dipakai (next_value=2, kembalikan 1);
// perebutan konkuren diselesaikan Postgres via UPSERT dalam satu statement.
func (r *pendaftaranRepo) NextRegistrationSequence(ctx context.Context, year, month int) (int, error) {
	if year <= 0 || month < 1 || month > 12 {
		return 0, domain.NewValidationError("Periode sequence pendaftaran tidak valid")
	}
	var seq int
	query := `
		INSERT INTO pendaftaran_nomor_sequence (tahun, bulan, next_value)
		VALUES ($1, $2, 2)
		ON CONFLICT (tahun, bulan)
		DO UPDATE SET next_value = pendaftaran_nomor_sequence.next_value + 1
		RETURNING next_value - 1`
	if err := r.db.GetContext(ctx, &seq, query, year, month); err != nil {
		return 0, fmt.Errorf("gagal mengalokasikan nomor urut pendaftaran: %w", err)
	}
	return seq, nil
}

// pendaftaranColumns adalah proyeksi eksplisit (BE-005): hindari SELECT *
// agar kolom sensitif baru tidak ikut transit memori jalur publik.
// Catatan: NIK tetap aman di JSON via tag `json:"-"` (defense in depth).
const pendaftaranColumns = `id, nomor_pendaftaran, nama_lengkap, nik_hash,
	nik_encrypted, tempat_lahir, tanggal_lahir, jenis_kelamin, agama,
	pendidikan, pekerjaan, status_pribadi, alamat, provinsi_id, kabupaten_id,
	kecamatan, desa, kode_pos, email, whatsapp, motivasi, persyaratan_checklist,
	foto_key, ktp_key,
	cv_key, sk_key, surat_pernyataan_key, surat_sehat_key, status, tipe_pendaftaran,
	catatan_perbaikan, revisi_token_hash, revisi_token_expires_at,
	anggota_id, created_at, updated_at`

func (r *pendaftaranRepo) GetByID(ctx context.Context, id int) (*domain.Pendaftaran, error) {
	var p domain.Pendaftaran
	query := `SELECT ` + pendaftaranColumns + ` FROM pendaftaran WHERE id = $1`
	if err := r.db.GetContext(ctx, &p, query, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (r *pendaftaranRepo) GetByNomorPendaftaran(ctx context.Context, nomor string) (*domain.Pendaftaran, error) {
	var p domain.Pendaftaran
	query := `SELECT ` + pendaftaranColumns + ` FROM pendaftaran WHERE nomor_pendaftaran = $1`
	if err := r.db.GetContext(ctx, &p, query, strings.TrimSpace(nomor)); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (r *pendaftaranRepo) GetByNikHash(ctx context.Context, nikHash string) (*domain.Pendaftaran, error) {
	var p domain.Pendaftaran
	query := `SELECT ` + pendaftaranColumns + ` FROM pendaftaran WHERE nik_hash = $1 LIMIT 1`
	if err := r.db.GetContext(ctx, &p, query, strings.TrimSpace(nikHash)); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (r *pendaftaranRepo) ListHistory(ctx context.Context, pendaftaranID int) ([]domain.PendaftaranRiwayat, error) {
	items := make([]domain.PendaftaranRiwayat, 0)
	query := `SELECT id, pendaftaran_id, aksi, catatan, created_at
		FROM pendaftaran_riwayat WHERE pendaftaran_id = $1 ORDER BY created_at ASC`
	if err := r.db.SelectContext(ctx, &items, query, pendaftaranID); err != nil {
		return nil, err
	}
	return items, nil
}

// queueWhere membangun klausa WHERE + args dengan placeholder $n terindeks.
// Filter wilayah berasal dari ActorContext server-side (bukan client).
func queueWhere(provinsiID, kabupatenID *int, status string) (string, []interface{}) {
	where := []string{"1 = 1"}
	args := []interface{}{}
	if provinsiID != nil {
		args = append(args, *provinsiID)
		where = append(where, fmt.Sprintf("provinsi_id = $%d", len(args)))
	}
	if kabupatenID != nil {
		args = append(args, *kabupatenID)
		where = append(where, fmt.Sprintf("kabupaten_id = $%d", len(args)))
	}
	if s := strings.TrimSpace(status); s != "" {
		args = append(args, s)
		where = append(where, fmt.Sprintf("status = $%d", len(args)))
	}
	return strings.Join(where, " AND "), args
}

const queueColumns = `id, nomor_pendaftaran, nama_lengkap, status,
	provinsi_id, kabupaten_id, created_at, updated_at`

func boundLimit(limit int) int {
	if limit <= 0 {
		return 25
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func (r *pendaftaranRepo) ListQueue(ctx context.Context, provinsiID, kabupatenID *int, status string, limit, offset int) ([]domain.PendaftaranQueueItem, error) {
	limit = boundLimit(limit)
	if offset < 0 {
		offset = 0
	}
	where, args := queueWhere(provinsiID, kabupatenID, status)
	args = append(args, limit, offset)
	query := fmt.Sprintf(`SELECT %s FROM pendaftaran WHERE %s
		ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		queueColumns, where, len(args)-1, len(args))

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
	return items, nil
}

func (r *pendaftaranRepo) CountQueue(ctx context.Context, provinsiID, kabupatenID *int, status string) (int, error) {
	where, args := queueWhere(provinsiID, kabupatenID, status)
	var total int
	query := fmt.Sprintf(`SELECT COUNT(*) FROM pendaftaran WHERE %s`, where)
	if err := r.db.GetContext(ctx, &total, query, args...); err != nil {
		return 0, err
	}
	return total, nil
}

func (r *pendaftaranRepo) SetRevisiToken(ctx context.Context, id int, tokenHash string, expiresAt time.Time) error {
	query := `UPDATE pendaftaran
		SET revisi_token_hash = $1, revisi_token_expires_at = $2, updated_at = CURRENT_TIMESTAMP
		WHERE id = $3`
	res, err := r.db.ExecContext(ctx, query, tokenHash, expiresAt, id)
	if err != nil {
		return mapDBError(fmt.Errorf("gagal menyimpan token revisi: %w", err), "Konflik data pendaftaran")
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

func (r *pendaftaranRepo) SubmitRevisionTx(ctx context.Context, id int, tokenHash string, keys map[string]string, catatan string) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("gagal memulai transaksi revisi: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Token cocok + belum kedaluwarsa + state PERBAIKAN dalam satu UPDATE
	// atomik. Gagal cocok = 0 rows = satu error generik (tanpa oracle
	// bedakan token salah vs kedaluwarsa vs state salah).
	res, err := tx.ExecContext(ctx, `
		UPDATE pendaftaran SET
			foto_key = $1, ktp_key = $2, cv_key = $3, sk_key = $4,
			surat_pernyataan_key = $5, surat_sehat_key = $6,
			status = $7, catatan_perbaikan = $8,
			revisi_token_hash = NULL, revisi_token_expires_at = NULL,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = $9
		  AND revisi_token_hash = $10
		  AND revisi_token_expires_at > CURRENT_TIMESTAMP
		  AND status = $11`,
		keys["foto_key"], keys["ktp_key"], keys["cv_key"], keys["sk_key"],
		keys["surat_pernyataan_key"], keys["surat_sehat_key"],
		domain.PendaftaranStatusDraft, strings.TrimSpace(catatan),
		id, tokenHash, domain.PendaftaranStatusPerbaikan)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.NewValidationError("Token revisi tidak valid, kedaluwarsa, atau status bukan PERBAIKAN")
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO pendaftaran_riwayat (pendaftaran_id, aksi, catatan, created_at)
		 VALUES ($1, $2, $3, CURRENT_TIMESTAMP)`,
		id, "REVISI", "Dokumen revisi diunggah ulang oleh pendaftar"); err != nil {
		return fmt.Errorf("gagal mencatat riwayat revisi: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("gagal menyelesaikan transaksi revisi: %w", err)
	}
	return nil
}

func (r *pendaftaranRepo) UpdateStatus(ctx context.Context, id int, status domain.PendaftaranStatus, catatan string) error {
	query := `UPDATE pendaftaran SET status = $1, catatan_perbaikan = $2, updated_at = CURRENT_TIMESTAMP WHERE id = $3`
	result, err := r.db.ExecContext(ctx, query, status, catatan, id)
	if err != nil {
		return mapDBError(fmt.Errorf("gagal mengubah status pendaftaran: %w", err), "Konflik data pendaftaran")
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *pendaftaranRepo) AppendHistory(ctx context.Context, pendaftaranID int, aksi string, actorID, actorName, actorRole *string, catatan string) error {
	query := `
		INSERT INTO pendaftaran_riwayat (
			pendaftaran_id,
			aksi,
			actor_id,
			actor_name,
			actor_role,
			catatan,
			created_at
		) VALUES ($1, $2, $3, $4, $5, $6, CURRENT_TIMESTAMP)
	`
	_, err := r.db.ExecContext(ctx, query, pendaftaranID, aksi, actorID, actorName, actorRole, catatan)
	if err != nil {
		return fmt.Errorf("gagal mencatat riwayat pendaftaran: %w", err)
	}
	return nil
}

func (r *pendaftaranRepo) UpdateStatusWithHistory(ctx context.Context, id int, status domain.PendaftaranStatus, aksi string, actorID, actorName, actorRole *string, catatan string) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("gagal memulai transaksi status pendaftaran: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx,
		`UPDATE pendaftaran SET status = $1, catatan_perbaikan = $2, updated_at = CURRENT_TIMESTAMP WHERE id = $3`,
		status, catatan, id)
	if err != nil {
		return mapDBError(fmt.Errorf("gagal mengubah status pendaftaran: %w", err), "Konflik data pendaftaran")
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrNotFound
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO pendaftaran_riwayat (pendaftaran_id, aksi, actor_id, actor_name, actor_role, catatan, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, CURRENT_TIMESTAMP)`,
		id, aksi, actorID, actorName, actorRole, catatan); err != nil {
		return fmt.Errorf("gagal mencatat riwayat pendaftaran: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("gagal menyelesaikan transaksi status pendaftaran: %w", err)
	}
	return nil
}

func (r *pendaftaranRepo) IssueMember(ctx context.Context, pendaftaranID int, year int, ktaKey string) (*domain.Anggota, error) {
	if strings.TrimSpace(ktaKey) == "" {
		return nil, domain.NewValidationError("KTA_SIGNING_KEY belum dikonfigurasi")
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("gagal memulai transaksi penerbitan anggota: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var p domain.Pendaftaran
	if err := tx.GetContext(ctx, &p, `SELECT `+pendaftaranColumns+` FROM pendaftaran WHERE id = $1 FOR UPDATE`, pendaftaranID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("gagal mengambil pendaftaran: %w", err)
	}
	if p.Status != domain.PendaftaranStatusDiverifikasi {
		return nil, domain.NewValidationError("Pendaftaran belum berstatus DIVERIFIKASI")
	}
	if p.AnggotaID != nil {
		return nil, domain.NewConflictError("Pendaftaran sudah memiliki anggota")
	}

	// Kode BPS resmi kabupaten (bukan ID serial) agar NIA stabil dan terbaca.
	// Kode kab 4 digit sudah memuat kode provinsi (mis. "3204").
	var kabKode string
	if err := tx.GetContext(ctx, &kabKode, `SELECT kode FROM wilayah_kabupaten WHERE id = $1`, p.KabupatenID); err != nil {
		return nil, fmt.Errorf("gagal mengambil kode kabupaten: %w", err)
	}

	var sequence int
	err = tx.GetContext(ctx, &sequence, `
		INSERT INTO anggota_nia_sequence (provinsi_id, kabupaten_id, tahun, next_value)
		VALUES ($1, $2, $3, 2)
		ON CONFLICT (provinsi_id, kabupaten_id, tahun)
		DO UPDATE SET next_value = anggota_nia_sequence.next_value + 1
		RETURNING next_value - 1
	`, p.ProvinsiID, p.KabupatenID, year)
	if err != nil {
		return nil, fmt.Errorf("gagal menghasilkan sequence NIA: %w", err)
	}

	niaCode, err := nia.GenerateNIA(kabKode, year, sequence)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE pendaftaran SET status = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`, domain.PendaftaranStatusDisetujui, pendaftaranID); err != nil {
		return nil, fmt.Errorf("gagal mengubah status pendaftaran: %w", err)
	}

	var anggota domain.Anggota
	err = tx.GetContext(ctx, &anggota, `
		INSERT INTO anggota (
			nia, nama_lengkap, nik_hash, nik_encrypted, tempat_lahir, tanggal_lahir,
			jenis_kelamin, agama, pendidikan, pekerjaan, alamat, provinsi_id, kabupaten_id,
			kecamatan, desa, kode_pos, email, whatsapp, foto_key, ktp_key, cv_key, sk_key,
			surat_pernyataan_key, surat_sehat_key, status, tipe, angkatan, pendaftaran_id,
			tanggal_daftar, tanggal_angkat, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16,
			$17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29, $30,
			CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
		) RETURNING *
	`, niaCode, p.NamaLengkap, p.NIKHash, p.NIKEncrypted, p.TempatLahir, p.TanggalLahir,
		p.JenisKelamin, p.Agama, p.Pendidikan, p.Pekerjaan, p.Alamat, p.ProvinsiID,
		p.KabupatenID, p.Kecamatan, p.Desa, p.KodePos, p.Email, p.Whatsapp, p.FotoKey,
		p.KTPKey, p.CVKey, p.SKKey, p.SuratPernyataanKey, p.SuratSehatKey,
		domain.AnggotaStatusAktif, p.Tipe, fmt.Sprintf("%d", year), p.ID, p.CreatedAt, time.Now())
	if err != nil {
		return nil, mapDBError(
			fmt.Errorf("gagal membuat anggota: %w", err),
			"Data anggota sudah terdaftar (NIK/NIA duplikat)")
	}

	// Signature anti-pemalsuan QR KTA (RULES 20): HMAC atas data kanonis
	// NIA + tanggal angkat + ID anggota, dalam transaksi yang sama.
	tanggalAngkat := anggota.TanggalAngkat.Format("2006-01-02")
	ktaSig, err := crypto.KTASignature(niaCode, tanggalAngkat, anggota.ID, ktaKey)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE anggota SET kta_qr_hash = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`,
		ktaSig, anggota.ID); err != nil {
		return nil, fmt.Errorf("gagal menyimpan signature KTA: %w", err)
	}
	anggota.KTAQRHash = &ktaSig

	if _, err := tx.ExecContext(ctx, `UPDATE pendaftaran SET anggota_id = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`, anggota.ID, pendaftaranID); err != nil {
		return nil, fmt.Errorf("gagal menghubungkan anggota dengan pendaftaran: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO pendaftaran_riwayat (pendaftaran_id, aksi, catatan) VALUES ($1, $2, $3)`, pendaftaranID, string(domain.PendaftaranActionSetujui), "Anggota resmi diterbitkan dengan NIA "+niaCode); err != nil {
		return nil, fmt.Errorf("gagal mencatat penerbitan anggota: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("gagal menyelesaikan penerbitan anggota: %w", err)
	}
	return &anggota, nil
}
