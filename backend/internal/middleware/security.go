package middleware

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	fiberredis "github.com/gofiber/storage/redis/v3"
	"github.com/redis/go-redis/v9"

	"github.com/kipan-indonesia/sim-kipan-core/pkg/response"
)

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

// LoginAttemptLimiter membatasi percobaan login PER EMAIL (bukan hanya per IP).
// Layer kedua setelah AuthRateLimiter — melindungi dari botnet spray attack
// di mana penyerang pakai banyak IP untuk mencoba password satu akun target.
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
				return "login_attempt:email:" + strings.ToLower(body.Email)
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