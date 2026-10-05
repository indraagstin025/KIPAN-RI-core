package service

// PasswordResetService (Batch 3) — reset kata sandi mandiri via email.
//
// Alur: POST /auth/forgot-password (selalu sukses, anti-enumeration) →
// token 256-bit dikirim ke email terdaftar (hash disimpan di Redis,
// TTL 30 menit, sekali pakai) → POST /auth/reset-password menukar token
// dengan password baru + mencabut seluruh sesi.

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/alexedwards/argon2id"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"github.com/kipan-indonesia/sim-kipan-core/config"
	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/gateway"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
	"github.com/kipan-indonesia/sim-kipan-core/internal/service/svcutil"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/crypto"
)

const passwordResetTTL = 30 * time.Minute

type PasswordResetService interface {
	// ForgotPassword mengirim tautan reset bila email terdaftar & aktif.
	// Selalu mengembalikan nil untuk email apa pun (anti-enumeration).
	ForgotPassword(ctx context.Context, email string, audit domain.AuditContext) error
	// ResetPassword menukar token sekali pakai dengan password baru.
	ResetPassword(ctx context.Context, token, newPassword string, audit domain.AuditContext) error
	// SetPassword menukar token "buat kata sandi" (pwsetup:) dengan password baru.
	SetPassword(ctx context.Context, token, newPassword string, audit domain.AuditContext) error
}

// PasswordResetDeps adalah dependensi service reset password.
type PasswordResetDeps struct {
	UserRepo  repository.UserAccountRepository
	RDB       *redis.Client
	Mail      gateway.MailSender
	AuditRepo repository.AuditLogRepository
}

type passwordResetService struct {
	cfg       *config.Config
	userRepo  repository.UserAccountRepository
	rdb       *redis.Client
	mail      gateway.MailSender
	auditRepo repository.AuditLogRepository
}

func NewPasswordResetService(cfg *config.Config, deps PasswordResetDeps) PasswordResetService {
	return &passwordResetService{
		cfg: cfg, userRepo: deps.UserRepo, rdb: deps.RDB,
		mail: deps.Mail, auditRepo: deps.AuditRepo,
	}
}

func resetKey(rawToken string) string {
	return "pwreset:" + crypto.HashToken(rawToken)
}

func (s *passwordResetService) ready() error {
	if s.rdb == nil || s.mail == nil || s.userRepo == nil {
		return svcutil.Unavailable("reset kata sandi")
	}
	return nil
}

func (s *passwordResetService) ForgotPassword(ctx context.Context, email string, audit domain.AuditContext) error {
	if err := s.ready(); err != nil {
		return err
	}
	e := normalizeEmail(email)
	if e == "" {
		return nil // handler sudah menolak format; jaga-jaga tetap senyap
	}

	user, err := s.userRepo.GetByEmail(ctx, e)
	if err != nil || user == nil || user.Status != domain.UserStatusAktif {
		// Anti-enumeration: email tak dikenal / nonaktif → tetap sukses.
		if err != nil && !errors.Is(err, domain.ErrUserNotFound) {
			log.Warn().Err(err).Msg("forgot-password: gagal membaca user")
		}
		return nil
	}

	raw, err := crypto.GenerateSecureToken(32)
	if err != nil {
		return fmt.Errorf("gagal menerbitkan token reset: %w", err)
	}
	if err := s.rdb.Set(ctx, resetKey(raw), user.ID, passwordResetTTL).Err(); err != nil {
		return fmt.Errorf("gagal menyimpan token reset: %w", err)
	}

	link := svcutil.PublicURLFrom(s.cfg) + "/reset-password?token=" + raw
	content := PasswordResetEmail(user.Name, link)
	// Best-effort: kegagalan kirim tidak membocorkan keberadaan akun.
	if err := s.mail.Send(ctx, user.Email, content.Subject, content.TextBody, content.HTMLBody); err != nil {
		log.Warn().Err(err).Str("user_id", user.ID).Msg("forgot-password: gagal mengirim email reset")
	} else {
		log.Info().Str("user_id", user.ID).Msg("Email reset kata sandi terkirim")
	}

	meta := `{"event":"password_reset_request"}`
	svcutil.WriteAudit(ctx, s.auditRepo, audit, &user.ID, user.Name, string(user.Role),
		"users", user.ID, "PASSWORD_RESET_REQUEST", &meta)
	return nil
}

// consumeResetScript mengambil + menghapus token secara atomik (sekali pakai).
// Lua agar kompatibel dengan Redis <6.2 (tanpa GETDEL).
var consumeResetScript = redis.NewScript(`
local v = redis.call('GET', KEYS[1])
if v then
	redis.call('DEL', KEYS[1])
	return v
end
return 0
`)

func (s *passwordResetService) ResetPassword(ctx context.Context, token, newPassword string, audit domain.AuditContext) error {
	if s.rdb == nil || s.userRepo == nil {
		return svcutil.Unavailable("reset kata sandi")
	}
	t := strings.TrimSpace(token)
	if t == "" || len(t) > 256 {
		return domain.NewValidationError("Token reset tidak valid atau kedaluwarsa")
	}

	val, err := consumeResetScript.Run(ctx, s.rdb, []string{resetKey(t)}).Result()
	if err != nil {
		return fmt.Errorf("gagal memverifikasi token reset: %w", err)
	}
	userID, ok := val.(string)
	if !ok || strings.TrimSpace(userID) == "" {
		return domain.NewValidationError("Token reset tidak valid atau kedaluwarsa")
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return domain.NewValidationError("Token reset tidak valid atau kedaluwarsa")
	}
	if user.Status != domain.UserStatusAktif {
		return domain.ErrUserInactive
	}
	if match, err := argon2id.ComparePasswordAndHash(newPassword, user.PasswordHash); err == nil && match {
		return domain.NewValidationError("Kata sandi baru tidak boleh sama dengan kata sandi lama")
	}

	hash, err := argon2id.CreateHash(newPassword, argon2Params)
	if err != nil {
		return fmt.Errorf("gagal hash password baru: %w", err)
	}
	if err := s.userRepo.UpdatePassword(ctx, user.ID, hash); err != nil {
		return fmt.Errorf("gagal menyimpan password baru: %w", err)
	}
	if err := s.userRepo.RevokeAllUserTokens(ctx, user.ID); err != nil {
		log.Warn().Err(err).Str("user_id", user.ID).
			Msg("Gagal mencabut sesi setelah reset password")
	}

	meta := `{"event":"password_reset","sessions_revoked":true}`
	svcutil.WriteAudit(ctx, s.auditRepo, audit, &user.ID, user.Name, string(user.Role),
		"users", user.ID, "PASSWORD_RESET", &meta)
	return nil
}

// SetPassword menukar token set-password (prefix `pwsetup:`, diterbitkan
// worker outbox) dengan kata sandi baru untuk anggota.
func (s *passwordResetService) SetPassword(ctx context.Context, token, newPassword string, audit domain.AuditContext) error {
	if s.rdb == nil || s.userRepo == nil {
		return svcutil.Unavailable("set kata sandi")
	}
	t := strings.TrimSpace(token)
	if t == "" || len(t) > 256 {
		return domain.NewValidationError("Tautan tidak valid atau kedaluwarsa")
	}
	val, err := consumeResetScript.Run(ctx, s.rdb, []string{setupKey(t)}).Result()
	if err != nil {
		return fmt.Errorf("gagal memverifikasi tautan: %w", err)
	}
	userID, ok := val.(string)
	if !ok || strings.TrimSpace(userID) == "" {
		return domain.NewValidationError("Tautan tidak valid atau kedaluwarsa")
	}
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return domain.NewValidationError("Tautan tidak valid atau kedaluwarsa")
	}
	if user.Status != domain.UserStatusAktif {
		return domain.ErrUserInactive
	}
	hash, err := argon2id.CreateHash(newPassword, argon2Params)
	if err != nil {
		return fmt.Errorf("gagal hash password baru: %w", err)
	}
	if err := s.userRepo.UpdatePassword(ctx, user.ID, hash); err != nil {
		return fmt.Errorf("gagal menyimpan password baru: %w", err)
	}
	if err := s.userRepo.RevokeAllUserTokens(ctx, user.ID); err != nil {
		log.Warn().Err(err).Str("user_id", user.ID).Msg("Gagal mencabut sesi setelah set password")
	}
	meta := `{"event":"set_password","sessions_revoked":true}`
	svcutil.WriteAudit(ctx, s.auditRepo, audit, &user.ID, user.Name, string(user.Role),
		"users", user.ID, "PASSWORD_SET", &meta)
	return nil
}
