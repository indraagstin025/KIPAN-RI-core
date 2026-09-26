package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/alexedwards/argon2id"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"github.com/kipan-indonesia/sim-kipan-core/config"
	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/middleware"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/crypto"
)

// ============================================================
// DTO (Data Transfer Object)
// ============================================================

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,password_strength"`
}

// AuthResponse — response publik untuk klien.
// Refresh token SENGAJA TIDAK disertakan di sini.
// Refresh token hanya dikirim via HttpOnly cookie (set oleh handler).
type AuthResponse struct {
	AccessToken string       `json:"access_token"`
	ExpiresIn   int64        `json:"expires_in"`
	User        UserResponse `json:"user"`
}

type UserResponse struct {
	ID            string      `json:"id"`
	Email         string      `json:"email"`
	Name          string      `json:"name"`
	Role          domain.Role `json:"role"`
	Status        string      `json:"status"`
	ProvinsiID    *int        `json:"provinsi_id,omitempty"`
	ProvinsiNama  *string     `json:"provinsi_nama,omitempty"`
	KabupatenID   *int        `json:"kabupaten_id,omitempty"`
	KabupatenNama *string     `json:"kabupaten_nama,omitempty"`
	Wilayah       string      `json:"wilayah"`
	AvatarURL     *string     `json:"avatar_url,omitempty"`
}

// ============================================================
// Service Interface
// ============================================================

// AuthService — perhatikan Login dan RefreshToken mengembalikan refresh token
// sebagai return value terpisah. Handler WAJIB menaruhnya di HttpOnly cookie,
// BUKAN di JSON response.
type AuthService interface {
	Login(ctx context.Context, req LoginRequest) (refreshToken string, resp *AuthResponse, err error)
	RefreshToken(ctx context.Context, rawRefreshToken string) (newRefreshToken string, resp *AuthResponse, err error)
	Logout(ctx context.Context, rawAccessToken, rawRefreshToken string) error
	GetProfile(ctx context.Context, userID string) (*UserResponse, error)
	ChangePassword(ctx context.Context, userID string, req ChangePasswordRequest) error
}

// ============================================================
// Implementation
// ============================================================

type authService struct {
	cfg      *config.Config
	userRepo repository.UserRepository
	rdb      *redis.Client
}

// Parameter Argon2id sesuai OWASP 2025 & spesifikasi proyek (Rule 9).
var argon2Params = &argon2id.Params{
	Memory:      64 * 1024, // 64 MB
	Iterations:  3,
	Parallelism: 2,
	SaltLength:  16,
	KeyLength:   32,
}

func NewAuthService(cfg *config.Config, userRepo repository.UserRepository, rdb *redis.Client) AuthService {
	return &authService{
		cfg:      cfg,
		userRepo: userRepo,
		rdb:      rdb,
	}
}

// ============================================================
// LOGIN
// ============================================================

func (s *authService) Login(ctx context.Context, req LoginRequest) (string, *AuthResponse, error) {
	user, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		// Timing equalization (fix C-9): jalankan operasi argon2id setara agar
		// waktu respons tidak membedakan "email tidak ada" vs "password salah".
		_, _ = argon2id.CreateHash(req.Password, argon2Params)
		return "", nil, domain.ErrInvalidCredentials
	}

	match, err := argon2id.ComparePasswordAndHash(req.Password, user.PasswordHash)
	if err != nil || !match {
		return "", nil, domain.ErrInvalidCredentials
	}

	// Cek status SETELAH password terverifikasi.
	// Pesan error disamarkan dengan ErrInvalidCredentials untuk mencegah
	// enumerasi akun (attacker tidak bisa membedakan akun nonaktif dari akun tidak ada).
	if user.Status != domain.UserStatusAktif {
		return "", nil, domain.ErrInvalidCredentials
	}

	if err := s.userRepo.UpdateLastLogin(ctx, user.ID); err != nil {
		// Jangan gagalkan login — hanya catat.
		log.Warn().Err(err).Str("user_id", user.ID).Msg("Gagal memperbarui last_login_at")
	}

	familyID := uuid.NewString()
	return s.issueNewSession(ctx, user, familyID)
}

// ============================================================
// REFRESH TOKEN — Token Rotation (Atomic & Safe)
// ============================================================

func (s *authService) RefreshToken(ctx context.Context, rawRefreshToken string) (string, *AuthResponse, error) {
	tokenHash := crypto.HashToken(rawRefreshToken)
	tokenRecord, err := s.userRepo.FindRefreshToken(ctx, tokenHash)
	if err != nil {
		return "", nil, domain.ErrInvalidToken
	}

	// Deteksi token reuse: token sudah pernah di-revoke sebelumnya.
	// Ini indikasi pencurian token — cabut SEMUA token dalam family.
	if tokenRecord.IsRevoked {
		_ = s.userRepo.RevokeFamilyTokens(ctx, tokenRecord.FamilyID)
		log.Warn().
			Str("user_id", tokenRecord.UserID).
			Str("family_id", tokenRecord.FamilyID).
			Msg("Refresh token reuse terdeteksi — seluruh sesi family dicabut")
		return "", nil, domain.NewForbiddenError(
			"Token refresh terindikasi digunakan ulang. Sesi Anda dihentikan demi keamanan.")
	}

	if time.Now().After(tokenRecord.ExpiresAt) {
		return "", nil, domain.ErrInvalidToken
	}

	user, err := s.userRepo.GetByID(ctx, tokenRecord.UserID)
	if err != nil {
		return "", nil, domain.ErrUserNotFound
	}

	// Fix C-3: cek status user — user suspended/nonaktif tidak boleh refresh.
	if user.Status != domain.UserStatusAktif {
		_ = s.userRepo.RevokeFamilyTokens(ctx, tokenRecord.FamilyID)
		return "", nil, domain.ErrUserInactive
	}

	// Fix C-6: rotasi atomik — revoke token lama + insert token baru
	// dalam satu transaksi DB. Jika salah satu gagal, rollback total.
	return s.rotateSession(ctx, user, tokenRecord)
}

// ============================================================
// LOGOUT
// ============================================================

func (s *authService) Logout(ctx context.Context, rawAccessToken, rawRefreshToken string) error {
	s.revokeRefreshFamily(ctx, rawRefreshToken)
	s.blacklistAccessToken(ctx, rawAccessToken)
	return nil
}

func (s *authService) revokeRefreshFamily(ctx context.Context, rawRefreshToken string) {
	if rawRefreshToken == "" {
		return
	}

	tokenHash := crypto.HashToken(rawRefreshToken)
	rec, err := s.userRepo.FindRefreshToken(ctx, tokenHash)
	if err != nil || rec == nil {
		return
	}

	_ = s.userRepo.RevokeFamilyTokens(ctx, rec.FamilyID)
}

func (s *authService) blacklistAccessToken(ctx context.Context, rawAccessToken string) {
	if s.rdb == nil || rawAccessToken == "" {
		return
	}

	claims := &middleware.JWTClaims{}
	_, err := jwt.ParseWithClaims(rawAccessToken, claims,
		func(t *jwt.Token) (interface{}, error) {
			return []byte(s.cfg.Auth.AccessTokenSecret), nil
		},
		jwt.WithValidMethods([]string{"HS256"}),
	)
	if err != nil || claims.ID == "" || claims.ExpiresAt == nil {
		return
	}

	ttl := time.Until(claims.ExpiresAt.Time)
	if ttl <= 0 {
		// Token sudah kedaluwarsa secara alami — tidak perlu di-blacklist.
		return
	}

	key := fmt.Sprintf("blacklist:jti:%s", claims.ID)
	if err := s.rdb.Set(ctx, key, "revoked", ttl).Err(); err != nil {
		log.Warn().
			Err(err).
			Str("jti", claims.ID).
			Msg("Gagal menyimpan blacklist jti di Redis")
	}
}

// ============================================================
// GET PROFILE
// ============================================================

func (s *authService) GetProfile(ctx context.Context, userID string) (*UserResponse, error) {
	u, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	resp := s.mapUserResponse(u)
	return &resp, nil
}

// ============================================================
// CHANGE PASSWORD
// ============================================================

func (s *authService) ChangePassword(ctx context.Context, userID string, req ChangePasswordRequest) error {
	u, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return domain.ErrUserNotFound
	}

	match, err := argon2id.ComparePasswordAndHash(req.OldPassword, u.PasswordHash)
	if err != nil || !match {
		return domain.NewValidationError("Kata sandi saat ini tidak sesuai")
	}

	if req.NewPassword == req.OldPassword {
		return domain.NewValidationError(
			"Kata sandi baru tidak boleh sama dengan kata sandi saat ini")
	}

	newHash, err := argon2id.CreateHash(req.NewPassword, argon2Params)
	if err != nil {
		return fmt.Errorf("gagal hash password baru: %w", err)
	}

	if err := s.userRepo.UpdatePassword(ctx, userID, newHash); err != nil {
		return fmt.Errorf("gagal menyimpan password baru: %w", err)
	}

	// Fix C-5: cabut SEMUA sesi (semua family) setelah password berubah.
	// Ini mencegah attacker yang sudah mencuri refresh token tetap punya akses.
	if err := s.userRepo.RevokeAllUserTokens(ctx, userID); err != nil {
		log.Warn().Err(err).Str("user_id", userID).
			Msg("Gagal mencabut seluruh sesi setelah ganti password")
	}

	return nil
}

// ============================================================
// INTERNAL HELPERS
// ============================================================

// issueNewSession membuat sesi baru untuk login path.
// access token + refresh token + record di DB (familyID baru).
func (s *authService) issueNewSession(
	ctx context.Context,
	user *domain.User,
	familyID string,
) (string, *AuthResponse, error) {
	accessToken, expiry, err := s.generateAccessToken(user)
	if err != nil {
		return "", nil, err
	}

	rawRefresh, tokenHash := generateRefreshTokenValue()
	refreshExpiry := s.refreshTTL()

	if err := s.userRepo.SaveRefreshToken(
		ctx, user.ID, tokenHash, familyID, time.Now().Add(refreshExpiry),
	); err != nil {
		return "", nil, fmt.Errorf("gagal menyimpan refresh token: %w", err)
	}

	return rawRefresh, &AuthResponse{
		AccessToken: accessToken,
		ExpiresIn:   int64(expiry.Seconds()),
		User:        s.mapUserResponse(user),
	}, nil
}

// rotateSession melakukan rotasi atomik: revoke token lama + terbitkan baru.
// Fix C-6: seluruh operasi DB dijalankan dalam 1 transaksi.
func (s *authService) rotateSession(
	ctx context.Context,
	user *domain.User,
	oldRecord *repository.RefreshTokenRecord,
) (string, *AuthResponse, error) {
	accessToken, expiry, err := s.generateAccessToken(user)
	if err != nil {
		return "", nil, err
	}

	rawRefresh, tokenHash := generateRefreshTokenValue()
	refreshExpiry := s.refreshTTL()

	if err := s.userRepo.RotateRefreshToken(
		ctx,
		oldRecord.ID,
		user.ID,
		tokenHash,
		oldRecord.FamilyID,
		time.Now().Add(refreshExpiry),
	); err != nil {
		// Race terdeteksi: token yang sama ternyata sudah dirotasi oleh request
		// paralel. Ini pola khas session cloning (satu token dipakai dua pihak),
		// sehingga seluruh family dicabut — sama seperti penanganan reuse di
		// RefreshToken. Tanpa cabang ini, kegagalan CAS akan tampak seperti
		// error internal biasa dan sesi penyerang tetap hidup.
		if errors.Is(err, domain.ErrTokenAlreadyRotated) {
			_ = s.userRepo.RevokeFamilyTokens(ctx, oldRecord.FamilyID)
			log.Warn().
				Str("user_id", user.ID).
				Str("family_id", oldRecord.FamilyID).
				Msg("Rotasi refresh token paralel terdeteksi — seluruh sesi family dicabut")
			return "", nil, domain.NewForbiddenError(
				"Token refresh terindikasi digunakan ulang. Sesi Anda dihentikan demi keamanan.")
		}

		return "", nil, fmt.Errorf("gagal rotasi refresh token: %w", err)
	}

	return rawRefresh, &AuthResponse{
		AccessToken: accessToken,
		ExpiresIn:   int64(expiry.Seconds()),
		User:        s.mapUserResponse(user),
	}, nil
}

// generateAccessToken membuat JWT access token dengan claims lengkap + jti.
func (s *authService) generateAccessToken(user *domain.User) (string, time.Duration, error) {
	expiry := s.cfg.Auth.AccessTokenTTL
	if expiry <= 0 {
		expiry = 15 * time.Minute
	}

	jti := uuid.NewString()

	claims := middleware.JWTClaims{
		UserID:      user.ID,
		Email:       user.Email,
		Role:        user.Role,
		ProvinsiID:  user.ProvinsiID,
		KabupatenID: user.KabupatenID,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti, // WAJIB: middleware menolak token tanpa jti
			Subject:   user.ID,
			Issuer:    s.cfg.App.Name,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(s.cfg.Auth.AccessTokenSecret))
	if err != nil {
		return "", 0, fmt.Errorf("gagal menandatangani jwt: %w", err)
	}
	return signed, expiry, nil
}

// refreshTTL mengembalikan durasi refresh token dari config dengan fallback 7 hari.
func (s *authService) refreshTTL() time.Duration {
	if s.cfg.Auth.RefreshTokenTTL > 0 {
		return s.cfg.Auth.RefreshTokenTTL
	}
	return 7 * 24 * time.Hour
}

// generateRefreshTokenValue menghasilkan raw refresh token + hash SHA-256-nya.
// Raw dikirim ke klien via HttpOnly cookie; hash disimpan di DB.
func generateRefreshTokenValue() (raw string, hash string) {
	raw = uuid.NewString() + "." + uuid.NewString()
	hash = crypto.HashToken(raw)
	return
}

// mapUserResponse memetakan domain.User ke DTO publik.
func (s *authService) mapUserResponse(u *domain.User) UserResponse {
	wilayah := "Nasional"
	if u.Role == domain.RoleAdminProvinsi && u.ProvinsiNama != nil {
		wilayah = *u.ProvinsiNama
	} else if u.Role == domain.RoleAdminKabupaten && u.KabupatenNama != nil {
		wilayah = *u.KabupatenNama
	}

	return UserResponse{
		ID:            u.ID,
		Email:         u.Email,
		Name:          u.Name,
		Role:          u.Role,
		Status:        string(u.Status),
		ProvinsiID:    u.ProvinsiID,
		ProvinsiNama:  u.ProvinsiNama,
		KabupatenID:   u.KabupatenID,
		KabupatenNama: u.KabupatenNama,
		Wilayah:       wilayah,
		AvatarURL:     u.AvatarURL,
	}
}
