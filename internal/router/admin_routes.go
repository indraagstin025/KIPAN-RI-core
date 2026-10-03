package router

// admin_routes.go mendaftarkan endpoint admin umum: verifikasi scoping
// wilayah (/admin/me-scope) dan endpoint uji RBAC (hanya non-production).

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/middleware"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/response"
)

// registerAdminRoutes mendaftarkan /admin/me-scope (dipakai dasbor) dan
// endpoint uji super-only / nasional-or-super (non-production saja).
func registerAdminRoutes(v1 fiber.Router, env string, authMiddleware *middleware.AuthMiddleware) {
	admin := v1.Group("/admin", authMiddleware.Authenticate())

	// Verifikasi scoping wilayah (dipakai dasbor).
	admin.Get("/me-scope", middleware.ScopeWilayah(), handleMeScope)

	// Guard RBAC — endpoint uji, non-production saja.
	if env != "production" {
		admin.Get("/super-only", middleware.RequireRoles(domain.RoleSuperAdmin), handleSuperOnly)
		admin.Get("/nasional-or-super",
			middleware.RequireRoles(domain.RoleSuperAdmin, domain.RoleAdminNasional),
			handleNasionalOrSuper,
		)
	}
}

// handleMeScope mengembalikan hasil evaluasi middleware ScopeWilayah untuk
// pemanggil (klaim JWT + filter wilayah). Dipakai uji & dasbor.
func handleMeScope(c *fiber.Ctx) error {
	claims := middleware.GetUser(c)
	if claims == nil {
		return response.Unauthorized(c, "Tidak terotentikasi")
	}
	scope := middleware.GetWilayahScope(c)

	return response.Success(c, "Akses otorisasi wilayah terverifikasi", fiber.Map{
		"user_id":             claims.UserID,
		"email":               claims.Email,
		"role":                claims.Role,
		"provinsi_id_claim":   claims.ProvinsiID,
		"kabupaten_id_claim":  claims.KabupatenID,
		"filter_provinsi_id":  scope.ProvinsiID,
		"filter_kabupaten_id": scope.KabupatenID,
		"is_nasional_scope":   scope.IsNasional(),
	})
}

// handleSuperOnly adalah endpoint dummy untuk menguji guard Super-Admin.
func handleSuperOnly(c *fiber.Ctx) error {
	return response.Success(c, "Akses khusus SUPER_ADMIN berhasil dibuka", nil)
}

// handleNasionalOrSuper adalah endpoint dummy untuk menguji guard Nasional/Super.
func handleNasionalOrSuper(c *fiber.Ctx) error {
	return response.Success(c, "Akses level nasional/super admin berhasil dibuka", nil)
}
