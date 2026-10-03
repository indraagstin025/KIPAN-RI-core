package router

// user_routes.go mendaftarkan layanan mandiri akun USER (unduh KTA sendiri).

import (
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/handler"
	"github.com/kipan-indonesia/sim-kipan-core/internal/middleware"
)

// registerUserRoutes mendaftarkan /user/* (khusus role USER). Otorisasi
// kepemilikan ditegakkan di service via anggota.user_id.
func registerUserRoutes(
	v1 fiber.Router,
	rdb *redis.Client,
	authMiddleware *middleware.AuthMiddleware,
	ktaHandler *handler.KTAHandler,
) {
	user := v1.Group("/user",
		authMiddleware.Authenticate(),
		middleware.RequireRoles(domain.RoleUser),
		middleware.RateLimit(rdb, "agt_pub"),
	)
	user.Get("/kta", ktaHandler.DownloadMyKTA)
}
