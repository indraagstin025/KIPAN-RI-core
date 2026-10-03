package router

// user_admin_routes.go mendaftarkan manajemen akun admin (khusus Super Admin).

import (
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/handler"
	"github.com/kipan-indonesia/sim-kipan-core/internal/middleware"
)

// registerUserAdminRoutes mendaftarkan /admin/users (list, counts, CRUD) —
// hanya SUPER_ADMIN.
func registerUserAdminRoutes(
	v1 fiber.Router,
	rdb *redis.Client,
	authMiddleware *middleware.AuthMiddleware,
	h *handler.UserAdminHandler,
) {
	grp := v1.Group("/admin/users",
		authMiddleware.Authenticate(),
		middleware.RequireCapability(domain.CapManageUsers),
		middleware.MutatingRateLimit(rdb, "usr_mut"),
	)
	grp.Get("", h.List)
	grp.Get("/counts", h.Counts)
	grp.Post("", h.Create)
	grp.Put("/:id", h.Update)
	grp.Delete("/:id", h.Delete)
}
