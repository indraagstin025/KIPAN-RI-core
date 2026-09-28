package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/kipan-indonesia/sim-kipan-core/config"
	"github.com/kipan-indonesia/sim-kipan-core/internal/database"
	"github.com/kipan-indonesia/sim-kipan-core/internal/handler"
	"github.com/kipan-indonesia/sim-kipan-core/internal/middleware"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/validator"
)

func main() {
	setupLogger()

	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("Gagal memuat konfigurasi. Server tidak dapat berjalan.")
	}

	log.Info().
		Str("env", cfg.App.Env).
		Str("port", cfg.App.Port).
		Msg("SIM-KIPAN Core API starting...")

	// ============================================================
	// Database
	// ============================================================
	db, err := database.ConnectPostgres(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Gagal konfigurasi database PostgreSQL")
	}
	if db != nil {
		defer func() {
			if err := db.Close(); err != nil {
				log.Warn().Err(err).Msg("Gagal menutup koneksi database")
			}
		}()
	}

	// ============================================================
	// Redis
	// ============================================================
	rdb, err := database.ConnectRedis(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Gagal konfigurasi Redis")
	}
	if rdb != nil {
		defer func() {
			if err := rdb.Close(); err != nil {
				log.Warn().Err(err).Msg("Gagal menutup koneksi Redis")
			}
		}()
	}

	// ============================================================
	// Fiber app
	// ============================================================
	app := setupFiberApp(cfg)
	handler.RegisterRoutes(app, cfg, db, rdb, validator.New())

	startServer(app, cfg.App.Port)
}

func setupLogger() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	// Development: console writer berwarna
	// Production: JSON output untuk aggregator (Loki, ELK, Datadog)
	if os.Getenv("APP_ENV") == "production" {
		log.Logger = zerolog.New(os.Stdout).
			With().
			Timestamp().
			Logger()
	} else {
		log.Logger = log.Output(zerolog.ConsoleWriter{
			Out:        os.Stderr,
			TimeFormat: time.RFC3339,
		})
	}
}

func setupFiberApp(cfg *config.Config) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:           cfg.App.Name,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       30 * time.Second,
		BodyLimit:         2 * 1024 * 1024, // 2 MB
		EnablePrintRoutes: cfg.App.Debug,
		ErrorHandler:      errorHandler,

		// ============================================================
		// Fix C-8: Trusted Proxy configuration
		//
		// Di produksi, request masuk lewat Caddy/Nginx (TLS termination)
		// lalu di-proxy ke Fiber dalam bentuk HTTP. Tanpa konfigurasi ini,
		// c.IP() akan mengembalikan IP proxy (127.0.0.1) — bukan IP klien.
		//
		// KONSEKUENSI:
		//   - Rate limiter akan lumpuh (semua request dari 1 IP)
		//   - Log forensik tidak berguna (IP palsu)
		//
		// Fiber hanya mempercayai header X-Forwarded-For dari IP di TrustedProxies.
		// Request dari IP lain yang mengirim header ini akan diabaikan.
		// ============================================================
		EnableTrustedProxyCheck: true,
		TrustedProxies:          parseTrustedProxies(cfg),
		ProxyHeader:             fiber.HeaderXForwardedFor,
	})

	// Middleware chain (urutan PENTING):
	// 1. Recover harus paling luar agar panic di middleware lain ditangkap
	// 2. RequestID untuk tracing
	// 3. Logger mencatat semua (sukses & gagal)
	// 4. CORS
	app.Use(recover.New(recover.Config{EnableStackTrace: cfg.App.Debug}))
	app.Use(requestid.New())
	app.Use(middleware.RequestLogger())
	app.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.App.AllowOrigin,
		AllowMethods:     "GET,POST,PUT,PATCH,DELETE,OPTIONS",
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization, X-Request-ID",
		AllowCredentials: true,
		MaxAge:           86400,
	}))

	return app
}

// parseTrustedProxies mengembalikan daftar IP/CIDR proxy yang dipercaya.
//
// L-7: HANYA proxy yang didaftarkan eksplisit yang dipercaya. Versi lama
// otomatis memercayai seluruh RFC1918 di production — bila Go terekspos
// langsung atau ada host lain di VPC, X-Forwarded-For palsu dianggap sah
// (bypass limiter per-IP + forensik IP rusak).
//
// Default: localhost saja. Tambahan via APP_TRUSTED_PROXIES (koma, mis.
// "10.0.0.5, 172.16.0.0/12") — isi dengan IP Caddy yang sebenarnya.
func parseTrustedProxies(cfg *config.Config) []string {
	base := []string{"127.0.0.1", "::1"}

	for _, p := range strings.Split(cfg.App.TrustedProxies, ",") {
		if p = strings.TrimSpace(p); p != "" {
			base = append(base, p)
		}
	}

	return base
}

func startServer(app *fiber.App, port string) {
	// Channel untuk menangkap error dari goroutine listen
	serverErr := make(chan error, 1)

	go func() {
		addr := fmt.Sprintf(":%s", port)
		log.Info().Str("addr", addr).Msg("Server listening")

		if err := app.Listen(addr); err != nil {
			// Jangan log.Fatal di goroutine — biarkan main() yang handle
			// agar defer db.Close() dan graceful shutdown tetap jalan.
			serverErr <- fmt.Errorf("gagal start server: %w", err)
		}
	}()

	// Tunggu sinyal OS atau error server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	select {
	case <-quit:
		log.Info().Msg("Menerima sinyal shutdown, menutup server...")
	case err := <-serverErr:
		log.Error().Err(err).Msg("Server berhenti karena error")
	}

	// Graceful shutdown dengan timeout 10 detik
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := app.ShutdownWithContext(ctx); err != nil {
		log.Error().Err(err).Msg("Gagal graceful shutdown")
		return
	}

	log.Info().Msg("Server berhenti dengan bersih. Selesai.")
}

// errorHandler adalah fallback terakhir untuk error yang tidak tertangani
// oleh response.FromError(). Format response dibuat konsisten dengan
// pkg/response agar frontend bisa parsing dengan satu skema.
func errorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	msg := "Terjadi kesalahan pada server"

	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
		// Sanitasi: jangan bocorkan pesan internal Fiber ke klien
		switch code {
		case fiber.StatusBadRequest:
			msg = "Format permintaan tidak valid"
		case fiber.StatusUnauthorized:
			msg = "Autentikasi diperlukan"
		case fiber.StatusForbidden:
			msg = "Akses ditolak"
		case fiber.StatusNotFound:
			msg = "Endpoint tidak ditemukan"
		case fiber.StatusRequestEntityTooLarge:
			msg = "Ukuran payload melebihi batas"
		default:
			msg = "Terjadi kesalahan pada server"
		}
	}

	// Log lengkap untuk forensik; pesan ke klien tetap generik
	log.Error().
		Int("status", code).
		Str("method", c.Method()).
		Str("path", c.Path()).
		Str("request_id", c.GetRespHeader("X-Request-ID")).
		Err(err).
		Msg("Unhandled request error")

	// Response body menggunakan format ErrorResponse yang sama dengan
	// pkg/response agar konsisten di mata frontend.
	traceID := c.GetRespHeader("X-Request-ID")
	return c.Status(code).JSON(fiber.Map{
		"success":  false,
		"code":     errorCodeForStatus(code),
		"message":  msg,
		"trace_id": traceID,
	})
}

// errorCodeForStatus memetakan HTTP status ke error code string
// (konsisten dengan pkg/response).
func errorCodeForStatus(status int) string {
	switch status {
	case fiber.StatusBadRequest:
		return "BAD_REQUEST"
	case fiber.StatusUnauthorized:
		return "UNAUTHORIZED"
	case fiber.StatusForbidden:
		return "FORBIDDEN"
	case fiber.StatusNotFound:
		return "NOT_FOUND"
	case fiber.StatusMethodNotAllowed:
		return "METHOD_NOT_ALLOWED"
	case fiber.StatusRequestEntityTooLarge:
		return "PAYLOAD_TOO_LARGE"
	case fiber.StatusTooManyRequests:
		return "TOO_MANY_REQUESTS"
	case fiber.StatusInternalServerError:
		return "INTERNAL_ERROR"
	default:
		return "ERROR"
	}
}

// ensure strings import tidak dihapus oleh gofmt
var _ = strings.TrimSpace