package router

// kepengurusan_routes.go mendaftarkan endpoint SK, jabatan, dan pengurus.
// Otorisasi rinci (yurisdiksi + rantai approval) ditegakkan di service.

import (
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/handler"
	"github.com/kipan-indonesia/sim-kipan-core/internal/middleware"
)

// registerKepengurusanRoutes mendaftarkan /admin/{jabatan,sk,pengurus} untuk
// seluruh admin; mutasi master jabatan dibatasi Super/Nasional.
func registerKepengurusanRoutes(
	v1 fiber.Router,
	rdb *redis.Client,
	authMiddleware *middleware.AuthMiddleware,
	h *handler.KepengurusanHandler,
) {
	adminRoles := middleware.RequireRoles(
		domain.RoleSuperAdmin,
		domain.RoleAdminNasional,
		domain.RoleAdminProvinsi,
		domain.RoleAdminKabupaten,
	)
	grp := v1.Group("/admin",
		authMiddleware.Authenticate(),
		adminRoles,
		middleware.MutatingRateLimit(rdb, "kep_mut"),
	)

	grp.Get("/jabatan", h.ListJabatan)
	grp.Get("/sk", h.ListSK)
	grp.Get("/sk/:id", h.GetSK)
	grp.Post("/sk", h.CreateSK)
	grp.Post("/sk/:id/approve", h.ApproveSK)
	grp.Post("/sk/:id/status", h.SetSKStatus)
	grp.Post("/sk/:id/pengurus", h.AddPengurus)
	grp.Delete("/sk/:id/pengurus/:pengurusId", h.RemovePengurus)
	grp.Get("/pengurus", h.ListPengurus)
	grp.Get("/pengurus/stats", h.PengurusStats)
	grp.Get("/pengurus/promosi", h.ListPromosi)
	grp.Patch("/pengurus/:id", h.UpdatePengurusStatus)
	grp.Patch("/pengurus/:id/jabatan", h.UpdatePengurusJabatan)

	// Master jabatan (mutasi) hanya Nasional/Super.
	master := grp.Group("/jabatan", middleware.RequireRoles(domain.RoleSuperAdmin, domain.RoleAdminNasional))
	master.Post("", h.CreateJabatan)
	master.Put("/:id", h.UpdateJabatan)
}
