package router

// backup_routes.go mendaftarkan manajemen backup database (Super Admin).

import (
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/handler"
	"github.com/kipan-indonesia/sim-kipan-core/internal/middleware"
)

// registerBackupRoutes mendaftarkan /admin/backups (list/buat/unduh/hapus).
func registerBackupRoutes(
	v1 fiber.Router,
	rdb *redis.Client,
	authMiddleware *middleware.AuthMiddleware,
	h *handler.BackupHandler,
) {
	grp := v1.Group("/admin/backups",
		authMiddleware.Authenticate(),
		middleware.RequireCapability(domain.CapManageBackup),
	)
	grp.Get("", h.List)
	grp.Get("/:id/download", h.Download)
	mut := middleware.MutatingRateLimit(rdb, "bak_mut")
	grp.Post("", mut, h.Create)
	grp.Delete("/:id", mut, h.Delete)
}
