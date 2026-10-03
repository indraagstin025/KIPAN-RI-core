package middleware

// Uji Batch 6 (#2): proteksi token endpoint internal.

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestRequireInternalToken(t *testing.T) {
	app := fiber.New()
	app.Get("/internal/health", RequireInternalToken("rahasia-internal"), func(c *fiber.Ctx) error {
		return c.SendString("detail")
	})

	cases := []struct {
		name   string
		header string
		want   int
	}{
		{"tanpa header", "", fiber.StatusNotFound},
		{"token salah", "salah", fiber.StatusNotFound},
		{"token benar", "rahasia-internal", fiber.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/internal/health", nil)
			if tc.header != "" {
				req.Header.Set("X-Internal-Token", tc.header)
			}
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("request gagal: %v", err)
			}
			if resp.StatusCode != tc.want {
				t.Fatalf("harap %d, dapat %d", tc.want, resp.StatusCode)
			}
		})
	}
}
