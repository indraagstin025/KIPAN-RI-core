package router

// laporan_routes.go mendaftarkan laporan & statistik admin (ter-scope wilayah).

import (
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/handler"
	"github.com/kipan-indonesia/sim-kipan-core/internal/middleware"
)

// registerLaporanRoutes mendaftarkan /admin/laporan (data + ekspor CSV).
func registerLaporanRoutes(
	v1 fiber.Router,
	rdb *redis.Client,
	authMiddleware *middleware.AuthMiddleware,
	h *handler.LaporanHandler,
) {
	grp := v1.Group("/admin/laporan",
		authMiddleware.Authenticate(),
		middleware.RequireCapability(domain.CapViewLaporan),
		middleware.ScopeWilayah(),
	)
	grp.Get("", h.Get)
	grp.Get("/export.csv", h.Export)
}
