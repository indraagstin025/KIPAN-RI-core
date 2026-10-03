package router

// audit_routes.go mendaftarkan penelusur jejak audit (Admin Nasional/Super).

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/handler"
	"github.com/kipan-indonesia/sim-kipan-core/internal/middleware"
)

// registerAuditRoutes mendaftarkan /admin/audit (data + ekspor CSV).
func registerAuditRoutes(
	v1 fiber.Router,
	authMiddleware *middleware.AuthMiddleware,
	h *handler.AuditHandler,
) {
	grp := v1.Group("/admin/audit",
		authMiddleware.Authenticate(),
		middleware.RequireCapability(domain.CapViewAudit),
	)
	grp.Get("", h.List)
	grp.Get("/export.csv", h.Export)
}
