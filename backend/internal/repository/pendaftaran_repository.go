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
	ListByWilayah(ctx context.Context, provinsiID, kabupatenID *int, limit, offset int) ([]domain.Pendaftaran, error)
	UpdateStatus(ctx context.Context, id int, status domain.PendaftaranStatus, catatan string) error
	AppendHistory(ctx context.Context, pendaftaranID int, aksi string, actorID, actorName, actorRole *string, catatan string) error
	// UpdateStatusWithHistory mengubah status + mencatat riwayat beraktor
	// dalam satu transaksi (tidak ada riwayat yatim).
	UpdateStatusWithHistory(ctx context.Context, id int, status domain.PendaftaranStatus, aksi string, actorID, actorName, actorRole *string, catatan string) error
	// IssueMember menerbitkan anggota + NIA + signature KTA dalam satu
	// transaksi. ktaKey adalah KTA_SIGNING_KEY dari config (dipasok service).
	IssueMember(ctx context.Context, pendaftaranID int, year int, ktaKey string) (*domain.Anggota, error)
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
			foto_key,
			ktp_key,
			cv_key,
			sk_key,
			surat_pernyataan_key,
			surat_sehat_key,
			status,
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
			:foto_key,
			:ktp_key,
			:cv_key,
			:sk_key,
			:surat_pernyataan_key,
			:surat_sehat_key,
			:status,
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
			foto_key,
			ktp_key,
			cv_key,
			sk_key,
			surat_pernyataan_key,
			surat_sehat_key,
			status,
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
			:foto_key,
			:ktp_key,
			:cv_key,
			:sk_key,
			:surat_pernyataan_key,
			:surat_sehat_key,
			:status,
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

func (r *pendaftaranRepo) GetByID(ctx context.Context, id int) (*domain.Pendaftaran, error) {
	var p domain.Pendaftaran
	query := `SELECT * FROM pendaftaran WHERE id = $1`
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
	query := `SELECT * FROM pendaftaran WHERE nomor_pendaftaran = $1`
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
	query := `SELECT * FROM pendaftaran WHERE nik_hash = $1 LIMIT 1`
	if err := r.db.GetContext(ctx, &p, query, strings.TrimSpace(nikHash)); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (r *pendaftaranRepo) ListByWilayah(ctx context.Context, provinsiID, kabupatenID *int, limit, offset int) ([]domain.Pendaftaran, error) {
	params := []interface{}{}
	where := []string{"1 = 1"}

	if provinsiID != nil {
		where = append(where, "provinsi_id = ?")
		params = append(params, *provinsiID)
	}
	if kabupatenID != nil {
		where = append(where, "kabupaten_id = ?")
		params = append(params, *kabupatenID)
	}
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	query := fmt.Sprintf(`SELECT * FROM pendaftaran WHERE %s ORDER BY created_at DESC LIMIT %d OFFSET %d`,
		strings.Join(where, " AND "), limit, offset)
	query = strings.ReplaceAll(query, "?", "$?")
	_ = query

	rows, err := r.db.QueryxContext(ctx, `SELECT * FROM pendaftaran WHERE 1 = 1 ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.Pendaftaran, 0)
	for rows.Next() {
		var p domain.Pendaftaran
		if err := rows.StructScan(&p); err != nil {
			return nil, err
		}
		items = append(items, p)
	}
	return items, nil
}

func (r *pendaftaranRepo) UpdateStatus(ctx context.Context, id int, status domain.PendaftaranStatus, catatan string) error {
	query := `UPDATE pendaftaran SET status = $1, catatan_perbaikan = $2, updated_at = CURRENT_TIMESTAMP WHERE id = $3`
	result, err := r.db.ExecContext(ctx, query, status, catatan, id)
	if err != nil {
		return err
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
		return err
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
	if err := tx.GetContext(ctx, &p, `SELECT * FROM pendaftaran WHERE id = $1 FOR UPDATE`, pendaftaranID); err != nil {
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

	// Kode BPS resmi (bukan ID serial) agar NIA stabil dan terbaca.
	var provKode, kabKode string
	if err := tx.GetContext(ctx, &provKode, `SELECT kode FROM wilayah_provinsi WHERE id = $1`, p.ProvinsiID); err != nil {
		return nil, fmt.Errorf("gagal mengambil kode provinsi: %w", err)
	}
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

	niaCode, err := nia.GenerateNIA(provKode, kabKode, year, sequence)
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
			surat_pernyataan_key, surat_sehat_key, status, angkatan, pendaftaran_id,
			tanggal_daftar, tanggal_angkat, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16,
			$17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29,
			CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
		) RETURNING *
	`, niaCode, p.NamaLengkap, p.NIKHash, p.NIKEncrypted, p.TempatLahir, p.TanggalLahir,
		p.JenisKelamin, p.Agama, p.Pendidikan, p.Pekerjaan, p.Alamat, p.ProvinsiID,
		p.KabupatenID, p.Kecamatan, p.Desa, p.KodePos, p.Email, p.Whatsapp, p.FotoKey,
		p.KTPKey, p.CVKey, p.SKKey, p.SuratPernyataanKey, p.SuratSehatKey,
		domain.AnggotaStatusAktif, fmt.Sprintf("%d", year), p.ID, p.CreatedAt, time.Now())
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
