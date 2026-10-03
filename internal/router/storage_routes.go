package router

// storage_routes.go mendaftarkan endpoint tiket presigned S3 (upload publik
// ber-rate-limit ketat, dan view privat yang wajib auth + teraudit).

import (
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"

	"github.com/kipan-indonesia/sim-kipan-core/internal/handler"
	"github.com/kipan-indonesia/sim-kipan-core/internal/middleware"
)

// registerStorageRoutes mendaftarkan /storage/presign-upload (publik) dan
// /storage/presign-view (auth).
func registerStorageRoutes(
	v1 fiber.Router,
	rdb *redis.Client,
	authMiddleware *middleware.AuthMiddleware,
	h *handler.StorageHandler,
) {
	uploadLimiter := middleware.RateLimit(rdb, "stor_up")

	public := v1.Group("/storage")
	public.Post("/presign-upload", uploadLimiter, h.PresignUpload)
	public.Get("/presign-view", authMiddleware.Authenticate(), h.PresignView)
}
