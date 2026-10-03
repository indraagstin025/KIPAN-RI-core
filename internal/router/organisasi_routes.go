package router

// organisasi_routes.go mendaftarkan profil organisasi: baca publik + sunting
// Super Admin.

import (
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/handler"
	"github.com/kipan-indonesia/sim-kipan-core/internal/middleware"
)

// registerOrganisasiRoutes mendaftarkan /organisasi (publik) & /admin/organisasi.
func registerOrganisasiRoutes(
	v1 fiber.Router,
	rdb *redis.Client,
	authMiddleware *middleware.AuthMiddleware,
	h *handler.OrganisasiHandler,
) {
	public := v1.Group("/organisasi", middleware.RateLimit(rdb, "org_pub"))
	public.Get("", h.Get)

	admin := v1.Group("/admin/organisasi",
		authMiddleware.Authenticate(),
		middleware.RequireCapability(domain.CapManageOrganisasi),
		middleware.MutatingRateLimit(rdb, "org_mut"),
	)
	admin.Put("", h.Update)
}
