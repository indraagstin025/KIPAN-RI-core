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

// PendaftaranRepository menangani persistence data pendaftaran calon anggota.
type PendaftaranRepository interface {
	Create(ctx context.Context, p *domain.Pendaftaran) error
	GetByID(ctx context.Context, id int) (*domain.Pendaftaran, error)
	GetByNomorPendaftaran(ctx context.Context, nomor string) (*domain.Pendaftaran, error)
	GetByNikHash(ctx context.Context, nikHash string) (*domain.Pendaftaran, error)
	ListByWilayah(ctx context.Context, provinsiID, kabupatenID *int, limit, offset int) ([]domain.Pendaftaran, error)
	UpdateStatus(ctx context.Context, id int, status domain.PendaftaranStatus, catatan string) error
	AppendHistory(ctx context.Context, pendaftaranID int, aksi string, actorID *string, actorName *string, catatan string) error
	IssueMember(ctx context.Context, pendaftaranID int, year int) (*domain.Anggota, error)
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
		return fmt.Errorf("gagal menyimpan pendaftaran: %w", err)
	}
	return nil
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

func (r *pendaftaranRepo) AppendHistory(ctx context.Context, pendaftaranID int, aksi string, actorID *string, actorName *string, catatan string) error {
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
	_, err := r.db.ExecContext(ctx, query, pendaftaranID, aksi, actorID, actorName, nil, catatan)
	if err != nil {
		return fmt.Errorf("gagal mencatat riwayat pendaftaran: %w", err)
	}
	return nil
}

func (r *pendaftaranRepo) IssueMember(ctx context.Context, pendaftaranID int, year int) (*domain.Anggota, error) {
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

	nia := fmt.Sprintf("KIPAN.%02d.%d.%d.%04d", p.ProvinsiID, p.KabupatenID, year, sequence)
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
	`, nia, p.NamaLengkap, p.NIKHash, p.NIKEncrypted, p.TempatLahir, p.TanggalLahir,
		p.JenisKelamin, p.Agama, p.Pendidikan, p.Pekerjaan, p.Alamat, p.ProvinsiID,
		p.KabupatenID, p.Kecamatan, p.Desa, p.KodePos, p.Email, p.Whatsapp, p.FotoKey,
		p.KTPKey, p.CVKey, p.SKKey, p.SuratPernyataanKey, p.SuratSehatKey,
		domain.AnggotaStatusAktif, fmt.Sprintf("%d", year), p.ID, p.CreatedAt, time.Now())
	if err != nil {
		return nil, fmt.Errorf("gagal membuat anggota: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `UPDATE pendaftaran SET anggota_id = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`, anggota.ID, pendaftaranID); err != nil {
		return nil, fmt.Errorf("gagal menghubungkan anggota dengan pendaftaran: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO pendaftaran_riwayat (pendaftaran_id, aksi, catatan) VALUES ($1, $2, $3)`, pendaftaranID, string(domain.PendaftaranActionSetujui), "Anggota resmi diterbitkan dengan NIA "+nia); err != nil {
		return nil, fmt.Errorf("gagal mencatat penerbitan anggota: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("gagal menyelesaikan penerbitan anggota: %w", err)
	}
	return &anggota, nil
}
