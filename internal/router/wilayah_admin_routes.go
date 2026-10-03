package router

// wilayah_admin_routes.go mendaftarkan master wilayah admin (Super/Nasional).

import (
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/handler"
	"github.com/kipan-indonesia/sim-kipan-core/internal/middleware"
)

// registerWilayahAdminRoutes mendaftarkan /admin/wilayah (daftar, detail,
// pengurus, ubah status, tambah dari master) — hanya Super/Nasional.
func registerWilayahAdminRoutes(
	v1 fiber.Router,
	rdb *redis.Client,
	authMiddleware *middleware.AuthMiddleware,
	h *handler.WilayahAdminHandler,
) {
	grp := v1.Group("/admin/wilayah",
		authMiddleware.Authenticate(),
		middleware.RequireCapability(domain.CapManageWilayah),
		middleware.MutatingRateLimit(rdb, "wil_mut"),
	)
	grp.Get("/cards", h.Cards)
	grp.Get("", h.List)
	grp.Post("", h.Add)
	grp.Get("/:type/:id/detail", h.Detail)
	grp.Get("/:type/:id/pengurus", h.Pengurus)
	grp.Patch("/:type/:id", h.SetStatus)
}
