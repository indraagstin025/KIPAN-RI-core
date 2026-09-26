package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
)

type RefreshTokenRecord struct {
	ID        string    `db:"id"`
	UserID    string    `db:"user_id"`
	TokenHash string    `db:"token_hash"`
	FamilyID  string    `db:"family_id"`
	IsRevoked bool      `db:"is_revoked"`
	ExpiresAt time.Time `db:"expires_at"`
	CreatedAt time.Time `db:"created_at"`
}

type UserRepository interface {
	GetByID(ctx context.Context, id string) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	Create(ctx context.Context, u *domain.User) error
	UpdateLastLogin(ctx context.Context, id string) error
	UpdatePassword(ctx context.Context, id string, newPasswordHash string) error

	// Refresh token management
	SaveRefreshToken(ctx context.Context, userID, tokenHash, familyID string, expiresAt time.Time) error
	FindRefreshToken(ctx context.Context, tokenHash string) (*RefreshTokenRecord, error)
	RevokeFamilyTokens(ctx context.Context, familyID string) error
	RevokeAllUserTokens(ctx context.Context, userID string) error
	RotateRefreshToken(ctx context.Context, oldTokenID, userID, newTokenHash, familyID string, expiresAt time.Time) error
}

type userRepo struct {
	db *sqlx.DB
}

func NewUserRepository(db *sqlx.DB) UserRepository {
	return &userRepo{db: db}
}

const selectUserSQL = `
	SELECT 
		u.id, u.email, u.password_hash, u.name, u.role, u.status, u.avatar_url, 
		u.provinsi_id, p.nama AS provinsi_nama,
		u.kabupaten_id, k.nama AS kabupaten_nama,
		u.last_login_at, u.created_at, u.updated_at, u.deleted_at 
	FROM users u
	LEFT JOIN wilayah_provinsi p ON u.provinsi_id = p.id
	LEFT JOIN wilayah_kabupaten k ON u.kabupaten_id = k.id
`

func (r *userRepo) GetByID(ctx context.Context, id string) (*domain.User, error) {
	var u domain.User
	query := selectUserSQL + ` WHERE u.id = $1 AND u.deleted_at IS NULL`
	err := r.db.GetContext(ctx, &u, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, err
	}
	return &u, nil
}

func (r *userRepo) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	var u domain.User
	query := selectUserSQL + ` WHERE LOWER(u.email) = LOWER($1) AND u.deleted_at IS NULL`
	err := r.db.GetContext(ctx, &u, query, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, err
	}
	return &u, nil
}

func (r *userRepo) Create(ctx context.Context, u *domain.User) error {
	if u.ID == "" {
		u.ID = uuid.NewString()
	}
	query := `INSERT INTO users (id, email, password_hash, name, role, status, avatar_url, provinsi_id, kabupaten_id, created_at, updated_at)
	          VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`
	_, err := r.db.ExecContext(ctx, query, u.ID, u.Email, u.PasswordHash, u.Name, u.Role, u.Status, u.AvatarURL, u.ProvinsiID, u.KabupatenID)
	return err
}

func (r *userRepo) UpdateLastLogin(ctx context.Context, id string) error {
	query := `UPDATE users SET last_login_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

func (r *userRepo) UpdatePassword(ctx context.Context, id string, newPasswordHash string) error {
	query := `UPDATE users SET password_hash = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, newPasswordHash, id)
	return err
}

// SaveRefreshToken menyimpan refresh token baru (login path).
func (r *userRepo) SaveRefreshToken(ctx context.Context, userID, tokenHash, familyID string, expiresAt time.Time) error {
	query := `INSERT INTO user_refresh_tokens (id, user_id, token_hash, family_id, is_revoked, expires_at)
	          VALUES ($1, $2, $3, $4, FALSE, $5)`
	_, err := r.db.ExecContext(ctx, query, uuid.NewString(), userID, tokenHash, familyID, expiresAt)
	return err
}

func (r *userRepo) FindRefreshToken(ctx context.Context, tokenHash string) (*RefreshTokenRecord, error) {
	var rec RefreshTokenRecord
	query := `SELECT id, user_id, token_hash, family_id, is_revoked, expires_at, created_at
	          FROM user_refresh_tokens WHERE token_hash = $1`
	err := r.db.GetContext(ctx, &rec, query, tokenHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrInvalidToken
		}
		return nil, err
	}
	return &rec, nil
}

// RevokeFamilyTokens mencabut SEMUA token dalam family yang sama.
// Dipakai saat deteksi token reuse (indikasi pencurian) atau logout.
func (r *userRepo) RevokeFamilyTokens(ctx context.Context, familyID string) error {
	query := `UPDATE user_refresh_tokens SET is_revoked = TRUE WHERE family_id = $1`
	_, err := r.db.ExecContext(ctx, query, familyID)
	return err
}

// RevokeAllUserTokens mencabut SEMUA refresh token milik user (semua family).
// Dipakai setelah change password — mencegah sesi lama tetap aktif.
func (r *userRepo) RevokeAllUserTokens(ctx context.Context, userID string) error {
	query := `UPDATE user_refresh_tokens 
	          SET is_revoked = TRUE 
	          WHERE user_id = $1 AND is_revoked = FALSE`
	_, err := r.db.ExecContext(ctx, query, userID)
	return err
}

// RotateRefreshToken melakukan rotasi token secara ATOMIK:
//  1. Revoke token lama dengan COMPARE-AND-SWAP (hanya jika belum di-revoke)
//  2. Insert token baru dalam family yang sama
//
// Jika salah satu gagal, seluruh transaksi di-rollback. Ini mencegah
// skenario user kehilangan semua token karena kegagalan parsial.
//
// KEAMANAN (race condition / session cloning):
// Langkah 1 memakai kondisi `AND is_revoked = FALSE` dan memeriksa
// RowsAffected. Dua request refresh PARALEL dengan token yang sama akan
// menghasilkan tepat SATU pemenang; yang kalah mendapat
// domain.ErrTokenAlreadyRotated sehingga service bisa mencabut seluruh family.
// Tanpa kondisi itu, keduanya lolos pemeriksaan IsRevoked (yang dibaca sebelum
// transaksi) dan sama-sama menerbitkan token baru dari satu token = session
// cloning. Lihat RULES #8.
func (r *userRepo) RotateRefreshToken(
	ctx context.Context,
	oldTokenID, userID, newTokenHash, familyID string,
	expiresAt time.Time,
) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("gagal memulai transaksi: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Step 1: revoke token lama — COMPARE-AND-SWAP, hanya berhasil sekali.
	res, err := tx.ExecContext(ctx,
		`UPDATE user_refresh_tokens
		 SET is_revoked = TRUE
		 WHERE id = $1 AND is_revoked = FALSE`,
		oldTokenID,
	)
	if err != nil {
		return fmt.Errorf("gagal revoke token lama: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("gagal membaca hasil revoke token lama: %w", err)
	}
	if affected == 0 {
		// Token sudah dirotasi/dicabut oleh request lain. Transaksi dibatalkan
		// oleh deferred Rollback, sehingga TIDAK ada token baru yang terbit.
		return domain.ErrTokenAlreadyRotated
	}

	// Step 2: insert token baru
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO user_refresh_tokens (id, user_id, token_hash, family_id, is_revoked, expires_at)
		 VALUES ($1, $2, $3, $4, FALSE, $5)`,
		uuid.NewString(), userID, newTokenHash, familyID, expiresAt,
	); err != nil {
		return fmt.Errorf("gagal insert token baru: %w", err)
	}

	return tx.Commit()
}
