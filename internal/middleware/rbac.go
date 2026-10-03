package middleware

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/response"
)

// WilayahScope merepresentasikan batas yurisdiksi wilayah yang aktif.
// Nilai nil pada field berarti "tidak ada filter" (akses nasional/super admin).
//
// Disimpan di c.Locals("wilayah_scope") sebagai satu-satunya source of truth
// untuk filter wilayah — hindari penulisan Locals mentah di tempat lain.
type WilayahScope struct {
	ProvinsiID  *int // nil = tidak difilter
	KabupatenID *int // nil = tidak difilter
}

// IsNasional mengembalikan true jika scope memberikan akses seluruh Indonesia
func (w WilayahScope) IsNasional() bool {
	return w.ProvinsiID == nil && w.KabupatenID == nil
}

// GetWilayahScope mengambil scope wilayah dari context Fiber.
// Jika belum di-set (mis. route publik), kembalikan scope kosong (= nasional).
func GetWilayahScope(c *fiber.Ctx) WilayahScope {
	scope, ok := c.Locals("wilayah_scope").(WilayahScope)
	if !ok {
		return WilayahScope{}
	}
	return scope
}

// RequireRoles memastikan user yang login memiliki salah satu role yang diizinkan.
// Jika claims tidak ada, kembalikan 401. Jika role tidak cocok, kembalikan 403.
func RequireRoles(allowedRoles ...domain.Role) fiber.Handler {
	return func(c *fiber.Ctx) error {
		claims := GetUser(c)
		if claims == nil {
			return response.Unauthorized(c, "Akses ditolak: otentikasi diperlukan")
		}

		for _, role := range allowedRoles {
			if claims.Role == role {
				return c.Next()
			}
		}

		return response.Forbidden(c, "Akses ditolak: role Anda tidak memiliki izin untuk tindakan ini")
	}
}

// ScopeWilayah menerapkan filter wilayah otomatis berdasarkan role admin:
//
//   - SUPER_ADMIN / ADMIN_NASIONAL : scope nasional (tanpa filter)
//   - ADMIN_PROVINSI               : filter provinsi_id = miliknya
//   - ADMIN_KABUPATEN              : filter provinsi_id DAN kabupaten_id = miliknya
//
// Middleware ini WAJIB dipasang setelah Authenticate() agar claims tersedia.
// Jika claims tidak lengkap (mis. Admin Provinsi tanpa provinsi_id), kembalikan 403.
func ScopeWilayah() fiber.Handler {
	return func(c *fiber.Ctx) error {
		claims := GetUser(c)
		// L-2: tanpa claims = tolak. Versi lama memanggil Next() sehingga
		// route yang lupa dipasang setelah Authenticate() mendapat scope
		// nasional diam-diam (fail-open).
		if claims == nil {
			return response.Unauthorized(c, "Akses ditolak: otentikasi diperlukan")
		}

	var scope WilayahScope

	switch claims.Role {
		case domain.RoleSuperAdmin, domain.RoleAdminNasional:
			// Scope nasional — kedua field nil
			scope = WilayahScope{}

		case domain.RoleAdminProvinsi:
			if claims.ProvinsiID == nil {
				return response.Forbidden(c, "Akun Admin Provinsi belum terhubung ke wilayah provinsi")
			}
			scope = WilayahScope{ProvinsiID: claims.ProvinsiID}

		case domain.RoleAdminKabupaten:
			if claims.KabupatenID == nil {
				return response.Forbidden(c, "Akun Admin Kabupaten belum terhubung ke wilayah kabupaten")
			}
		// Admin Kabupaten juga wajib punya ProvinsiID untuk konsistensi filter hierarki.
		scope = WilayahScope{
			ProvinsiID:  claims.ProvinsiID, // boleh nil jika legacy data
			KabupatenID: claims.KabupatenID,
		}

	case domain.RoleUser:
		// Akun anggota tidak memiliki yurisdiksi admin. Tolak eksplisit
		// agar tidak jatuh ke scope nasional diam-diam (fail-closed).
		return response.Forbidden(c, "Akun user tidak memiliki akses wilayah admin")

	default:
			return response.Forbidden(c, "Role tidak dikenal untuk scope wilayah")
		}

		c.Locals("wilayah_scope", scope)
		return c.Next()
	}
}