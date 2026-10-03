package middleware

// Uji Batch 1 — T4: error Redis saat cek blacklist harus fail-closed (503),
// bukan meloloskan token yang mungkin sudah dicabut.

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
)

// unreachableRedis menunjuk port yang pasti menolak koneksi (port 1).
func unreachableRedis() *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:        "127.0.0.1:1",
		DialTimeout: 150 * time.Millisecond,
		MaxRetries:  -1,
	})
}

func TestIsBlacklistedMengembalikanErrorSaatRedisMati(t *testing.T) {
	rdb := unreachableRedis()
	defer rdb.Close()

	mw := NewAuthMiddleware("secret", rdb)
	if _, err := mw.isBlacklisted(context.Background(), "jti-1"); err == nil {
		t.Fatal("harap error saat Redis tidak terjangkau (fail-closed)")
	}
}

func TestAuthenticateFailClosedSaatRedisMati(t *testing.T) {
	const secret = "0123456789abcdef0123456789abcdef"

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, JWTClaims{
		UserID: "user-1",
		Email:  "a@b.co",
		Role:   domain.RoleSuperAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        "jti-test-1",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	})
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("gagal menandatangani token uji: %v", err)
	}

	rdb := unreachableRedis()
	defer rdb.Close()

	app := fiber.New()
	app.Get("/x", NewAuthMiddleware(secret, rdb).Authenticate(), func(c *fiber.Ctx) error {
		return c.SendString("lolos")
	})

	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Authorization", "Bearer "+signed)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request gagal: %v", err)
	}
	if resp.StatusCode != fiber.StatusServiceUnavailable {
		t.Fatalf("harap 503 saat Redis mati, dapat %d", resp.StatusCode)
	}
}
