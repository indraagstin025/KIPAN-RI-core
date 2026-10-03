// Package router adalah composition root untuk registrasi rute HTTP SIM-KIPAN.
// Ia membangun dependensi (repo → service → handler) lalu mendaftarkan seluruh
// endpoint ke Fiber. Handler tetap berada di paket `handler` (transport-only).
package router

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"github.com/kipan-indonesia/sim-kipan-core/config"
	"github.com/kipan-indonesia/sim-kipan-core/internal/handler"
	"github.com/kipan-indonesia/sim-kipan-core/internal/middleware"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/validator"
)

// Register mendaftarkan seluruh endpoint aplikasi ke Fiber router.
// Bila DB tidak tersedia (mode dev tanpa DB), hanya health-check yang didaftarkan.
func Register(
	app *fiber.App,
	cfg *config.Config,
	db *sqlx.DB,
	rdb *redis.Client,
	val *validator.CustomValidator,
) {
	// Middleware global.
	app.Use(middleware.SecurityHeaders())
	app.Use(middleware.SlowRequestLogger(200 * time.Millisecond))

	// Health-check (tidak butuh DB).
	healthHandler := handler.NewHealthHandler(cfg.App.Name, cfg.App.Env, db, rdb)
	app.Get("/health", healthHandler.Check)
	registerInternalHealth(app, cfg, healthHandler)

	if db == nil {
		return
	}

	// Composition root + registrasi rute v1.
	d := newDeps(cfg, db, rdb, val)
	v1 := app.Group("/api/v1")

	registerAuthRoutes(v1, rdb, d.authHandler, d.pwResetHandler, d.authMiddleware)
	registerMembershipRoutes(v1, rdb, d.authMiddleware, d.pendaftaranHandler, d.otpHandler)
	registerStorageRoutes(v1, rdb, d.authMiddleware, d.storageHandler)
	registerWilayahRoutes(v1, rdb, d.wilayahHandler)
	registerWilayahAdminRoutes(v1, rdb, d.authMiddleware, d.wilayahAdminHandler)
	registerOutboxRoutes(v1, rdb, d.authMiddleware, d.outboxHandler)
	registerAnggotaRoutes(v1, rdb, d.authMiddleware, d.ktaHandler, d.anggotaHandler)
	registerNotificationRoutes(v1, d.authMiddleware, d.notifHandler)
	registerUserRoutes(v1, rdb, d.authMiddleware, d.ktaHandler)
	registerUserAdminRoutes(v1, rdb, d.authMiddleware, d.userAdminHandler)
	registerKepengurusanRoutes(v1, rdb, d.authMiddleware, d.kepengurusanHandler)
	registerDashboardRoutes(v1, rdb, d.authMiddleware, d.dashboardHandler)
	registerLaporanRoutes(v1, rdb, d.authMiddleware, d.laporanHandler)
	registerAuditRoutes(v1, d.authMiddleware, d.auditHandler)
	registerAdminRoutes(v1, cfg.App.Env, d.authMiddleware)
}

// registerInternalHealth mendaftarkan /internal/health secara fail-closed:
// bila INTERNAL_HEALTH_TOKEN diisi → wajib header; bila kosong hanya di
// non-production; di production tanpa token route TIDAK didaftarkan (404).
func registerInternalHealth(app *fiber.App, cfg *config.Config, h *handler.HealthHandler) {
	internal := app.Group("/internal")
	switch {
	case strings.TrimSpace(cfg.App.InternalHealthToken) != "":
		internal.Use(middleware.RequireInternalToken(cfg.App.InternalHealthToken))
		internal.Get("/health", h.Detail)
	case cfg.App.Env != "production":
		internal.Get("/health", h.Detail)
	default:
		log.Warn().Msg("/internal/health TIDAK didaftarkan: production tanpa INTERNAL_HEALTH_TOKEN")
	}
}
