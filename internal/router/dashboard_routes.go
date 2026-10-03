package router

// dashboard_routes.go mendaftarkan ringkasan analitik admin (ter-scope wilayah).

import (
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/handler"
	"github.com/kipan-indonesia/sim-kipan-core/internal/middleware"
)

// registerDashboardRoutes mendaftarkan /admin/dashboard untuk semua peran admin.
func registerDashboardRoutes(
	v1 fiber.Router,
	rdb *redis.Client,
	authMiddleware *middleware.AuthMiddleware,
	h *handler.DashboardHandler,
) {
	grp := v1.Group("/admin",
		authMiddleware.Authenticate(),
		middleware.RequireRoles(
			domain.RoleSuperAdmin,
			domain.RoleAdminNasional,
			domain.RoleAdminProvinsi,
			domain.RoleAdminKabupaten,
		),
	)
	grp.Get("/dashboard", h.Get)
}
