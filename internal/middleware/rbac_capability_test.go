package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
)

// capApp membangun app dengan role tetap + guard capability.
func capApp(role domain.Role, caps ...domain.Capability) *fiber.App {
	app := fiber.New()
	app.Get("/x", func(c *fiber.Ctx) error {
		c.Locals("user", &JWTClaims{UserID: "u1", Role: role})
		return c.Next()
	}, RequireCapability(caps...), func(c *fiber.Ctx) error {
		return c.SendString("SUCCESS")
	})
	return app
}

// capGet menjalankan request dan mengembalikan status.
func capGet(t *testing.T, app *fiber.App) int {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest("GET", "/x", nil))
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	return resp.StatusCode
}

// TestRequireCapability memverifikasi guard berbasis capability.
func TestRequireCapability(t *testing.T) {
	if code := capGet(t, capApp(domain.RoleSuperAdmin, domain.CapManageUsers)); code != fiber.StatusOK {
		t.Errorf("Super di manage_users harap 200, dapat %d", code)
	}
	if code := capGet(t, capApp(domain.RoleAdminNasional, domain.CapManageUsers)); code != fiber.StatusForbidden {
		t.Errorf("Nasional di manage_users harap 403, dapat %d", code)
	}
	if code := capGet(t, capApp(domain.RoleAdminKabupaten, domain.CapVerifyPendaftaran)); code != fiber.StatusOK {
		t.Errorf("Kabupaten di verify_pendaftaran harap 200, dapat %d", code)
	}
	if code := capGet(t, capApp(domain.RoleAdminProvinsi, domain.CapVerifyPendaftaran)); code != fiber.StatusForbidden {
		t.Errorf("Provinsi di verify_pendaftaran harap 403, dapat %d", code)
	}
}

// TestRequireCapabilityTanpaClaims memastikan 401 bila belum terotentikasi.
func TestRequireCapabilityTanpaClaims(t *testing.T) {
	app := fiber.New()
	app.Get("/x", RequireCapability(domain.CapManageUsers), func(c *fiber.Ctx) error {
		return c.SendString("SUCCESS")
	})
	if code := capGet(t, app); code != fiber.StatusUnauthorized {
		t.Errorf("tanpa claims harap 401, dapat %d", code)
	}
}
