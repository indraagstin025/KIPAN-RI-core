package router

// anggota_routes.go mendaftarkan endpoint anggota: cek publik minimal
// (anti scraping NIA) dan daftar/detail admin (ter-scope wilayah).

import (
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/handler"
	"github.com/kipan-indonesia/sim-kipan-core/internal/middleware"
)

// registerAnggotaRoutes mendaftarkan /anggota/cek (publik) dan /admin/anggota
// (list, detail, unduh KTA, reset kata sandi) untuk semua peran admin.
func registerAnggotaRoutes(
	v1 fiber.Router,
	rdb *redis.Client,
	authMiddleware *middleware.AuthMiddleware,
	ktaHandler *handler.KTAHandler,
	h *handler.AnggotaHandler,
) {
	public := v1.Group("/anggota", middleware.RateLimit(rdb, "agt_pub"))
	public.Get("/cek", h.CheckPublic)

	adminAnggota := v1.Group("/admin/anggota",
		authMiddleware.Authenticate(),
		middleware.RequireRoles(
			domain.RoleSuperAdmin,
			domain.RoleAdminNasional,
			domain.RoleAdminProvinsi,
			domain.RoleAdminKabupaten,
		),
	)
	adminAnggota.Get("", middleware.ScopeWilayah(), h.List)
	adminAnggota.Get("/export.csv", middleware.ScopeWilayah(), h.Export)
	adminAnggota.Get("/:id", middleware.ScopeWilayah(), h.Detail)
	adminAnggota.Get("/:id/riwayat", middleware.ScopeWilayah(), h.Riwayat)
	adminAnggota.Get("/:id/activity", middleware.ScopeWilayah(), h.Activity)
	adminAnggota.Get("/:id/kta", middleware.ScopeWilayah(), ktaHandler.DownloadKTA)
	// Reset kata sandi akun USER anggota (tautan set-password via antrian email).
	adminAnggota.Post("/:id/reset-password", middleware.ScopeWilayah(), h.ResetPassword)

	// Mutasi anggota langsung (tambah/sunting/status/nonaktif) — semua admin,
	// yurisdiksi wilayah ditegakkan di service.
	mut := middleware.MutatingRateLimit(rdb, "agt_mut")
	adminAnggota.Post("", middleware.ScopeWilayah(), mut, h.Create)
	adminAnggota.Put("/:id", middleware.ScopeWilayah(), mut, h.Update)
	adminAnggota.Patch("/:id/status", middleware.ScopeWilayah(), mut, h.SetStatus)
	adminAnggota.Delete("/:id", middleware.ScopeWilayah(), mut, h.Delete)
}
