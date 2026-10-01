package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
)

func TestRequireRoles(t *testing.T) {
	app := fiber.New()
 
	// Dummy endpoint with role guard
	app.Get("/test-super", func(c *fiber.Ctx) error {
		// Mock injected claims
		c.Locals("user", &JWTClaims{
			UserID: "user-1",
			Role:   domain.RoleAdminProvinsi,
		})
		return c.Next()
	}, RequireRoles(domain.RoleSuperAdmin), func(c *fiber.Ctx) error {
		return c.SendString("SUCCESS")
	})

	app.Get("/test-provinsi", func(c *fiber.Ctx) error {
		c.Locals("user", &JWTClaims{
			UserID: "user-1",
			Role:   domain.RoleAdminProvinsi,
		})
		return c.Next()
	}, RequireRoles(domain.RoleAdminProvinsi, domain.RoleSuperAdmin), func(c *fiber.Ctx) error {
		return c.SendString("SUCCESS")
	})

	// Test 1: Admin Provinsi mencoba akses endpoint Super Admin (Harus 403 Forbidden)
	req1 := httptest.NewRequest("GET", "/test-super", nil)
	resp1, err := app.Test(req1)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	if resp1.StatusCode != fiber.StatusForbidden {
		t.Errorf("Expected 403 Forbidden, got %d", resp1.StatusCode)
	}

	// Test 2: Admin Provinsi akses endpoint Provinsi (Harus 200 OK)
	req2 := httptest.NewRequest("GET", "/test-provinsi", nil)
	resp2, err := app.Test(req2)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	if resp2.StatusCode != fiber.StatusOK {
		t.Errorf("Expected 200 OK, got %d", resp2.StatusCode)
	}
}

func TestScopeWilayahTanpaClaimsDitolak(t *testing.T) {
	// L-2: ScopeWilayah tanpa claims (lupa Authenticate) wajib 401,
	// bukan Next() dengan scope nasional diam-diam.
	app := fiber.New()
	app.Get("/scope", ScopeWilayah(), func(c *fiber.Ctx) error {
		return c.SendString("SUCCESS")
	})

	req := httptest.NewRequest("GET", "/scope", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized, got %d", resp.StatusCode)
	}
}

func TestScopeWilayah(t *testing.T) {
	app := fiber.New()

	provID := 32
	kabID := 3273

	app.Get("/test-scope", func(c *fiber.Ctx) error {
		c.Locals("user", &JWTClaims{
			UserID:     "user-prov",
			Role:       domain.RoleAdminProvinsi,
			ProvinsiID: &provID,
		})
		return c.Next()
	}, ScopeWilayah(), func(c *fiber.Ctx) error {
		scope := GetWilayahScope(c)
		if scope.ProvinsiID == nil || *scope.ProvinsiID != 32 {
			return c.Status(500).SendString("Filter provinsi salah")
		}
		if scope.KabupatenID != nil {
			return c.Status(500).SendString("Filter kabupaten harus nil")
		}
		return c.SendString("OK")
	})

	app.Get("/test-scope-kab", func(c *fiber.Ctx) error {
		c.Locals("user", &JWTClaims{
			UserID:      "user-kab",
			Role:        domain.RoleAdminKabupaten,
			ProvinsiID:  &provID,
			KabupatenID: &kabID,
		})
		return c.Next()
	}, ScopeWilayah(), func(c *fiber.Ctx) error {
		scope := GetWilayahScope(c)
		if scope.KabupatenID == nil || *scope.KabupatenID != 3273 {
			return c.Status(500).SendString("Filter kabupaten salah")
		}
		if scope.ProvinsiID == nil || *scope.ProvinsiID != 32 {
			return c.Status(500).SendString("Filter provinsi salah")
		}
		return c.SendString("OK")
	})

	app.Get("/test-scope-nasional", func(c *fiber.Ctx) error {
		c.Locals("user", &JWTClaims{
			UserID: "user-nas",
			Role:   domain.RoleAdminNasional,
		})
		return c.Next()
	}, ScopeWilayah(), func(c *fiber.Ctx) error {
		scope := GetWilayahScope(c)
		if !scope.IsNasional() {
			return c.Status(500).SendString("Scope nasional harus kosong")
		}
		return c.SendString("OK")
	})

	// ... lanjutkan test runner
}
