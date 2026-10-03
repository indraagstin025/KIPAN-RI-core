package router

// notification_routes.go mendaftarkan notifikasi milik pengguna (anti-IDOR:
// identitas diambil dari JWT terverifikasi, bukan parameter klien).

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kipan-indonesia/sim-kipan-core/internal/handler"
	"github.com/kipan-indonesia/sim-kipan-core/internal/middleware"
)

// registerNotificationRoutes mendaftarkan /notifications/me dan tandai dibaca.
func registerNotificationRoutes(
	v1 fiber.Router,
	authMiddleware *middleware.AuthMiddleware,
	h *handler.NotificationHandler,
) {
	mine := v1.Group("/notifications", authMiddleware.Authenticate())
	mine.Get("/me", h.Mine)
	mine.Post("/:id/read", h.MarkRead)
}
