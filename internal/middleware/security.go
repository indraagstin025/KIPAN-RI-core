package middleware

import (
	"crypto/subtle"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	fiberredis "github.com/gofiber/storage/redis/v3"
	"github.com/redis/go-redis/v9"

	"github.com/kipan-indonesia/sim-kipan-core/pkg/response"
)

// RequireInternalToken membatasi endpoint internal (mis. /internal/health)
// dengan header rahasia X-Internal-Token. Perbandingan constant-time dan
// respons 404 (bukan 403) agar keberadaan endpoint tidak menjadi sinyal recon.
func RequireInternalToken(expected string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		got := c.Get("X-Internal-Token")
		if subtle.ConstantTimeCompare([]byte(got), []byte(expected)) != 1 {
			return response.NotFound(c, "Endpoint tidak ditemukan")
		}
		return c.Next()
	}
}

// SecurityHeaders menetapkan header keamanan standar untuk API.
//
// CATATAN HSTS:
// Di produksi, Caddy yang terminate TLS. Request ke Go Fiber adalah HTTP,
// sehingga c.Protocol() = "http". Karena itu, HSTS TIDAK di-set di sini —
// biarkan Caddy yang set (dia yang tahu dia serve HTTPS).
func SecurityHeaders() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Content-Type sniffing protection
		c.Set("X-Content-Type-Options", "nosniff")

		// Clickjacking protection
		c.Set("X-Frame-Options", "DENY")

		// Referrer policy
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Permissions policy
		c.Set("Permissions-Policy",
			"geolocation=(), microphone=(), camera=(), payment=(), usb=(), fullscreen=()")

		// CSP ketat: API tidak boleh render HTML/script sama sekali.
		c.Set("Content-Security-Policy",
			"default-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")

		// Cross-Origin isolation (defense in depth)
		c.Set("Cross-Origin-Opener-Policy", "same-origin")
		c.Set("Cross-Origin-Resource-Policy", "same-site")

		// Legacy Flash/PDF protection
		c.Set("X-Permitted-Cross-Domain-Policies", "none")

		// HSTS — set HANYA jika Fiber langsung serve HTTPS (dev/staging)
		// Di produksi, biarkan Caddy yang set.
		if c.Protocol() == "https" {
			c.Set("Strict-Transport-Security",
				"max-age=31536000; includeSubDomains")
		}

		return c.Next()
	}
}

// AuthRateLimiter membatasi frekuensi request per IP ke endpoint sensitif.
// Menggunakan Redis sebagai backing store agar konsisten di multi-instance.
//
// keyPrefix WAJIB unik per limiter: dua limiter dengan prefix sama berbagi
// counter Redis yang sama sehingga kuota tercampur dan 429 prematur.
// Catatan: OPTIONS preflight di-skip agar tidak menghabiskan kuota CORS.
func AuthRateLimiter(rdb *redis.Client, keyPrefix string, maxRequests int, window time.Duration) fiber.Handler {
	cfg := limiter.Config{
		Max:        maxRequests,
		Expiration: window,
		KeyGenerator: func(c *fiber.Ctx) string {
			// c.IP() menghormati TrustedProxies di main.go.
			return keyPrefix + c.IP()
		},
		Next: func(c *fiber.Ctx) bool {
			// Skip CORS preflight
			return c.Method() == fiber.MethodOptions
		},
		LimitReached: func(c *fiber.Ctx) error {
			return response.TooManyRequests(c,
				"Terlalu banyak percobaan autentikasi. Silakan coba kembali dalam 1 menit.")
		},
		SkipFailedRequests:     false,
		SkipSuccessfulRequests: false,
	}

	if rdb != nil {
		cfg.Storage = fiberredis.NewFromConnection(rdb)
	}

	return limiter.New(cfg)
}

// RevisionTokenLimiter membatasi permintaan token revisi PER NOMOR
// pendaftaran (BE-001): limiter per-IP saja tidak menghentikan enumerasi
// terarah ke satu target (botnet/ganti IP). Kunci memakai nomor ternormal
// (trim + upper) agar varian penulisan tidak mereset kuota.
func RevisionTokenLimiter(rdb *redis.Client, maxAttempts int, window time.Duration) fiber.Handler {
	cfg := limiter.Config{
		Max:        maxAttempts,
		Expiration: window,
		KeyGenerator: func(c *fiber.Ctx) string {
			var body struct {
				Nomor string `json:"nomor"`
			}
			_ = c.BodyParser(&body)
			nr := strings.ToUpper(strings.TrimSpace(body.Nomor))
			if nr == "" {
				return "rev_tok:ip:" + c.IP()
			}
			return "rev_tok:nomor:" + nr
		},
		LimitReached: func(c *fiber.Ctx) error {
			return response.TooManyRequests(c,
				"Terlalu banyak permintaan token revisi. Coba lagi nanti.")
		},
	}

	if rdb != nil {
		cfg.Storage = fiberredis.NewFromConnection(rdb)
	}

	return limiter.New(cfg)
}

// LoginAttemptLimiter membatasi percobaan login PER PASANGAN email+IP
// (BE-003): kunci email saja memungkinkan lockout akun oleh pihak ketiga
// (kirim 5 gagal memakai email korban). Kunci gabungan menutup lockout;
// proteksi spray botnet ditopang limiter per-IP (authLimiter 20/mnt).
// Backoff progresif + CAPTCHA + notifikasi pemilik ditunda ke Fase 5/6
// (butuh frontend), dengan pesan LimitReached yang tetap tidak membocorkan
// keterdaftaran email.
func LoginAttemptLimiter(rdb *redis.Client, maxAttempts int, window time.Duration) fiber.Handler {
	cfg := limiter.Config{
		Max:        maxAttempts,
		Expiration: window,
		KeyGenerator: func(c *fiber.Ctx) string {
			var body struct {
				Email string `json:"email"`
			}
			_ = c.BodyParser(&body)
			if body.Email != "" {
				return "login_attempt:" + strings.ToLower(strings.TrimSpace(body.Email)) + ":" + c.IP()
			}
			return "login_attempt:ip:" + c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return response.TooManyRequests(c,
				"Terlalu banyak percobaan login untuk akun ini. Coba lagi dalam beberapa menit.")
		},
	}

	if rdb != nil {
		cfg.Storage = fiberredis.NewFromConnection(rdb)
	}

	return limiter.New(cfg)
}

// PasswordResetLimiter membatasi permintaan reset password PER PASANGAN
// email+IP (Batch 3): mencegah spam email ke satu target (email bombing)
// sekaligus pembatasan per-IP dari RateLimit("pw_reset"). Pesan 429 tidak
// membocorkan keterdaftaran email.
func PasswordResetLimiter(rdb *redis.Client, maxAttempts int, window time.Duration) fiber.Handler {
	cfg := limiter.Config{
		Max:        maxAttempts,
		Expiration: window,
		KeyGenerator: func(c *fiber.Ctx) string {
			var body struct {
				Email string `json:"email"`
			}
			_ = c.BodyParser(&body)
			if body.Email != "" {
				return "pwreset:" + strings.ToLower(strings.TrimSpace(body.Email)) + ":" + c.IP()
			}
			return "pwreset:ip:" + c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return response.TooManyRequests(c,
				"Terlalu banyak permintaan reset kata sandi. Coba lagi beberapa menit lagi.")
		},
	}

	if rdb != nil {
		cfg.Storage = fiberredis.NewFromConnection(rdb)
	}

	return limiter.New(cfg)
}

// TrackLimiter membatasi pelacakan publik PER NOMOR pendaftaran (SEC-TRACK-PII):
// nomor REG sekuensial mudah dienumerasi, sehingga limiter per-IP saja tidak
// menghentikan scraping satu-target dari banyak IP. Kunci memakai nomor pada
// path (trim + upper) agar varian penulisan tidak mereset kuota; fallback ke
// IP bila nomor kosong.
func TrackLimiter(rdb *redis.Client, maxRequests int, window time.Duration) fiber.Handler {
	cfg := limiter.Config{
		Max:        maxRequests,
		Expiration: window,
		KeyGenerator: func(c *fiber.Ctx) string {
			nr := strings.ToUpper(strings.TrimSpace(c.Params("nomor")))
			if nr == "" {
				return "track:ip:" + c.IP()
			}
			return "track:nomor:" + nr
		},
		LimitReached: func(c *fiber.Ctx) error {
			return response.TooManyRequests(c,
				"Terlalu banyak pelacakan untuk nomor ini. Coba lagi nanti.")
		},
	}

	if rdb != nil {
		cfg.Storage = fiberredis.NewFromConnection(rdb)
	}

	return limiter.New(cfg)
}