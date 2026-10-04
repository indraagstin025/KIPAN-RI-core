package router

// outbox_routes.go mendaftarkan pemantauan antrian email (outbox) admin,
// termasuk kirim ulang manual (retry) dan kirim semua pending.

import (
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/handler"
	"github.com/kipan-indonesia/sim-kipan-core/internal/middleware"
)

// registerOutboxRoutes mendaftarkan /admin/email-outbox (list + retry) untuk
// seluruh admin (ter-scope wilayah di service).
func registerOutboxRoutes(
	v1 fiber.Router,
	rdb *redis.Client,
	authMiddleware *middleware.AuthMiddleware,
	h *handler.OutboxHandler,
) {
	grp := v1.Group("/admin/email-outbox",
		authMiddleware.Authenticate(),
		middleware.RequireRoles(
			domain.RoleSuperAdmin, domain.RoleAdminNasional,
			domain.RoleAdminProvinsi, domain.RoleAdminKabupaten,
		),
		middleware.MutatingRateLimit(rdb, "out_mut"),
	)
	grp.Get("", h.List)
	grp.Post("/retry-pending", h.RetryPending)
	grp.Post("/retry", h.RetryMany)
	grp.Post("/:id/send", h.Send)
}
