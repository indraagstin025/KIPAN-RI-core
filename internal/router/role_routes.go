package router

// role_routes.go mendaftarkan katalog Role & Wewenang (Super Admin).

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/handler"
	"github.com/kipan-indonesia/sim-kipan-core/internal/middleware"
)

// registerRoleRoutes mendaftarkan /admin/roles (katalog read-only).
func registerRoleRoutes(
	v1 fiber.Router,
	authMiddleware *middleware.AuthMiddleware,
	h *handler.RoleHandler,
) {
	grp := v1.Group("/admin/roles",
		authMiddleware.Authenticate(),
		middleware.RequireRoles(domain.RoleSuperAdmin),
	)
	grp.Get("", h.Catalog)
}
