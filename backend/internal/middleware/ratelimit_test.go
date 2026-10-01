package middleware

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

func TestRateLimitUnknownPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("nama tak dikenal harus panic saat wiring")
		}
	}()
	RateLimit(nil, "tidak-ada")
}

func TestRateLimitEnforcesQuota(t *testing.T) {
	// Tanpa Redis: memory store. Kebijakan "wil_pub" = 60/mnt; uji dengan
	// burst kecil melawan instance kustom via AuthRateLimiter langsung
	// agar test cepat (2 request, maks 1).
	app := fiber.New()
	app.Use(AuthRateLimiter(nil, "rl:test:ip:", 1, time.Minute))
	app.Get("/", func(c *fiber.Ctx) error { return c.SendString("ok") })

	for i := 0; i < 2; i++ {
		resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
		if err != nil {
			t.Fatalf("request gagal: %v", err)
		}
		want := fiber.StatusOK
		if i == 1 {
			want = fiber.StatusTooManyRequests
		}
		if resp.StatusCode != want {
			t.Fatalf("percobaan %d: harap %d, dapat %d", i+1, want, resp.StatusCode)
		}
	}
}

func TestRateLimitKnownNames(t *testing.T) {
	for _, name := range []string{"auth", "mem_pub", "mem_mut", "stor_up", "wil_pub", "agt_pub"} {
		if RateLimit(nil, name) == nil {
			t.Fatalf("limiter %q nil", name)
		}
	}
}
