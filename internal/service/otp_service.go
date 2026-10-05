package service

// OtpService menangani verifikasi kepemilikan nomor WhatsApp pendaftar
// (Batch 3, anti-bot untuk submit awal). Kode 6-digit dikirim via
// WAGateway (saat ini LogGateway = log-only dev), lalu ditukar menjadi
// token terverifikasi sekali pakai yang wajib disertakan saat submit.
//
// State 100% di Redis (tanpa migrasi DB). Kunci memakai hash nomor
// (bukan nomor mentah) agar keyspace tidak memuat PII. Semua kegagalan
// verifikasi mengembalikan pesan SERAGAM (tanpa oracle bedakan salah /
// kedaluwarsa / habis).

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/kipan-indonesia/sim-kipan-core/config"
	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/gateway"
	"github.com/kipan-indonesia/sim-kipan-core/internal/service/svcutil"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/crypto"
)

const (
	otpCodeTTL      = 5 * time.Minute
	otpCooldownTTL  = 60 * time.Second
	otpHourWindow   = time.Hour
	otpMaxPerHour   = 5
	otpMaxAttempts  = 5
	otpTokenTTL     = 15 * time.Minute
	otpCodeDigits   = 6
	otpUniformError = "Kode salah atau kedaluwarsa"
)

// OTPService adalah kontrak verifikasi OTP WhatsApp.
type OTPService interface {
	// RequestOTP menerbitkan kode baru (cooldown 60 dtk, maks 5/jam).
	RequestOTP(ctx context.Context, whatsapp string) (*OTPRequestResult, error)
	// VerifyOTP menukar kode benar menjadi token terverifikasi sekali pakai.
	VerifyOTP(ctx context.Context, whatsapp, code string) (*OTPVerifyResult, error)
	// VerifyAndConsume memvalidasi token milik nomor tsb lalu menghanguskannya.
	VerifyAndConsume(ctx context.Context, whatsapp, token string) error
}

// OTPRequestResult adalah respons permintaan kode.
type OTPRequestResult struct {
	ExpiresIn int64  `json:"expires_in"`
	ResendIn  int64  `json:"resend_in"`
	DebugCode string `json:"debug_code,omitempty"`
}

// OTPVerifyResult adalah respons verifikasi sukses.
type OTPVerifyResult struct {
	VerifiedToken string `json:"verified_token"`
	ExpiresIn     int64  `json:"expires_in"`
}

// OtpDeps adalah dependensi service OTP.
type OtpDeps struct {
	RDB     *redis.Client
	Gateway gateway.WAGateway
}

type otpService struct {
	cfg     *config.Config
	rdb     *redis.Client
	gateway gateway.WAGateway
}

func NewOTPService(cfg *config.Config, deps OtpDeps) OTPService {
	return &otpService{cfg: cfg, rdb: deps.RDB, gateway: deps.Gateway}
}

// otpKey membangun key Redis dari hash nomor ternormalisasi (tanpa PII mentah).
func otpKey(prefix, numberHash string) string {
	return "otp:wa:" + prefix + ":" + numberHash
}

func (s *otpService) ready() error {
	if s.rdb == nil || s.gateway == nil {
		return svcutil.Unavailable("verifikasi OTP")
	}
	return nil
}

// normalizeOTPTarget memvalidasi format WA lalu mengembalikan digit inti
// (08xx/62/+62 setara, reuse normalizeWA revision_service).
func normalizeOTPTarget(whatsapp string) (string, error) {
	wa := strings.TrimSpace(whatsapp)
	if !phonePattern.MatchString(wa) {
		return "", domain.NewValidationError("Nomor WhatsApp tidak valid (contoh: 081234567890)")
	}
	digits := normalizeWA(wa)
	if digits == "" {
		return "", domain.NewValidationError("Nomor WhatsApp tidak valid (contoh: 081234567890)")
	}
	return digits, nil
}

// RequestOTP menerbitkan kode 6-digit baru untuk nomor yang valid.
func (s *otpService) RequestOTP(ctx context.Context, whatsapp string) (*OTPRequestResult, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	digits, err := normalizeOTPTarget(whatsapp)
	if err != nil {
		return nil, err
	}
	nh := crypto.HashToken(digits)

	// Batas per jam (anti pemborosan gateway / spam target).
	hourKey := otpKey("hour", nh)
	hourly, err := s.rdb.Incr(ctx, hourKey).Result()
	if err != nil {
		return nil, fmt.Errorf("gagal meminta OTP: %w", err)
	}
	if hourly == 1 {
		_ = s.rdb.Expire(ctx, hourKey, otpHourWindow).Err()
	}
	if hourly > otpMaxPerHour {
		return nil, domain.NewValidationError("Terlalu banyak permintaan kode. Coba lagi nanti")
	}

	// Cooldown kirim ulang (SET NX 60 detik).
	cdKey := otpKey("cd", nh)
	if ok, err := s.rdb.SetNX(ctx, cdKey, "1", otpCooldownTTL).Result(); err != nil {
		return nil, fmt.Errorf("gagal meminta OTP: %w", err)
	} else if !ok {
		ttl, _ := s.rdb.TTL(ctx, cdKey).Result()
		if ttl < 0 {
			ttl = otpCooldownTTL
		}
		return nil, tooManyOTP(int64(ttl.Seconds()))
	}

	code, err := generateOTPCode(otpCodeDigits)
	if err != nil {
		return nil, fmt.Errorf("gagal membuat kode OTP: %w", err)
	}
	codeKey := otpKey("code", nh)
	if err := s.rdb.Set(ctx, codeKey, crypto.HashToken(code), otpCodeTTL).Err(); err != nil {
		return nil, fmt.Errorf("gagal menyimpan OTP: %w", err)
	}
	// Reset counter percobaan untuk kode baru.
	_ = s.rdb.Del(ctx, otpKey("att", nh)).Err()

	if err := s.gateway.SendMessage(ctx, digits, otpMessage(code)); err != nil {
		return nil, fmt.Errorf("gagal mengirim OTP: %w", err)
	}

	res := &OTPRequestResult{ExpiresIn: int64(otpCodeTTL.Seconds()), ResendIn: int64(otpCooldownTTL.Seconds())}
	// DEV-ONLY: kembalikan kode agar pentest/dev tanpa gateway tetap bisa
	// jalan. Production TIDAK PERNAH (fail-closed: kode hanya via WA).
	if s.cfg == nil || s.cfg.App.Env != "production" {
		res.DebugCode = code
	}
	return res, nil
}

// VerifyOTP memeriksa kode lalu menerbitkan token terverifikasi sekali pakai.
func (s *otpService) VerifyOTP(ctx context.Context, whatsapp, code string) (*OTPVerifyResult, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	digits, err := normalizeOTPTarget(whatsapp)
	if err != nil {
		return nil, err
	}
	c := strings.TrimSpace(code)
	if c == "" || len(c) > 16 {
		return nil, domain.NewValidationError(otpUniformError)
	}
	nh := crypto.HashToken(digits)
	codeKey := otpKey("code", nh)
	attKey := otpKey("att", nh)

	stored, err := s.rdb.Get(ctx, codeKey).Result()
	if errors.Is(err, redis.Nil) {
		// Kunci hilang = tidak pernah minta / kedaluwarsa / sudah dipakai.
		// Pesan seragam: tanpa oracle.
		return nil, domain.NewValidationError(otpUniformError)
	}
	if err != nil {
		return nil, fmt.Errorf("gagal memverifikasi OTP: %w", err)
	}
	if subtle.ConstantTimeCompare([]byte(stored), []byte(crypto.HashToken(c))) != 1 {
		att, err := s.rdb.Incr(ctx, attKey).Result()
		if err != nil {
			return nil, fmt.Errorf("gagal memverifikasi OTP: %w", err)
		}
		if att == 1 {
			_ = s.rdb.Expire(ctx, attKey, otpCodeTTL).Err()
		}
		if att >= otpMaxAttempts {
			// Kunci kode dihanguskan — minta kode baru.
			_ = s.rdb.Del(ctx, codeKey, attKey).Err()
		}
		return nil, domain.NewValidationError(otpUniformError)
	}

	// Sukses: hanguskan kode + counter, terbitkan token sekali pakai.
	_ = s.rdb.Del(ctx, codeKey, attKey).Err()
	raw, err := crypto.GenerateSecureToken(32)
	if err != nil {
		return nil, fmt.Errorf("gagal menerbitkan token verifikasi: %w", err)
	}
	if err := s.rdb.Set(ctx, otpKey("ok", crypto.HashToken(raw)), digits, otpTokenTTL).Err(); err != nil {
		return nil, fmt.Errorf("gagal menyimpan token verifikasi: %w", err)
	}
	return &OTPVerifyResult{VerifiedToken: raw, ExpiresIn: int64(otpTokenTTL.Seconds())}, nil
}

// consumeOTPScript menghanguskan token sekali pakai secara ATOMIK
// (GET + bandingkan + DEL dalam satu script). Lua dipakai alih-alih GETDEL
// agar kompatibel dengan Redis lama (<6.2) yang masih dipakai di dev.
var consumeOTPScript = redis.NewScript(`
local v = redis.call('GET', KEYS[1])
if v == ARGV[1] then
	return redis.call('DEL', KEYS[1])
else
	return 0
end
`)

// VerifyAndConsume memastikan token milik nomor tsb lalu menghanguskannya
// (sekali pakai — dipakai di CreateRegistration).
func (s *otpService) VerifyAndConsume(ctx context.Context, whatsapp, token string) error {
	if err := s.ready(); err != nil {
		return err
	}
	digits, err := normalizeOTPTarget(whatsapp)
	if err != nil {
		return err
	}
	t := strings.TrimSpace(token)
	if t == "" || len(t) > 256 {
		return domain.NewValidationError("Token verifikasi WhatsApp wajib diisi")
	}
	key := otpKey("ok", crypto.HashToken(t))
	n, err := consumeOTPScript.Run(ctx, s.rdb, []string{key}, digits).Int()
	if err != nil {
		return fmt.Errorf("gagal memverifikasi OTP: %w", err)
	}
	if n != 1 {
		// Token salah / kedaluwarsa / milik nomor lain / sudah dipakai:
		// satu pesan generik (tanpa oracle).
		return domain.NewValidationError("Verifikasi WhatsApp belum valid untuk nomor ini")
	}
	return nil
}

// generateOTPCode membuat kode numerik N digit dari crypto/rand
// (digit pertama tak-nol agar selalu tepat N digit).
func generateOTPCode(digits int) (string, error) {
	if digits <= 0 {
		digits = 6
	}
	min := 1
	for i := 1; i < digits; i++ {
		min *= 10
	}
	n, err := svcutil.RandIntN(9 * min)
	if err != nil {
		return "", err
	}
	code := min + n
	s := fmt.Sprintf("%d", code)
	if len(s) != digits {
		return "", fmt.Errorf("gagal menyusun kode OTP")
	}
	return s, nil
}

// otpMessage menyusun teks WhatsApp berisi kode verifikasi.
func otpMessage(code string) string {
	return fmt.Sprintf(
		"Kode verifikasi KIPAN Anda: %s\nBerlaku %d menit. Jangan bagikan kode ini kepada siapa pun.",
		code, int(otpCodeTTL.Minutes()))
}

// tooManyOTP membangun error 429 dengan pesan tunggu yang aman
// (tanpa membocorkan apakah nomor pernah meminta).
func tooManyOTP(waitSecs int64) error {
	return &domain.AppError{
		Code:   429,
		Message: "Terlalu banyak permintaan",
		Detail: fmt.Sprintf("Tunggu %d detik sebelum meminta kode lagi", waitSecs),
	}
}
