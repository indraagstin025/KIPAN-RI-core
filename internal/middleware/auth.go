package middleware

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/response"
)

// JWTClaims adalah alias ke domain.JWTClaims (L-3: definisi tunggal di
// domain). Alias menjaga kompatibilitas pemanggil lama.
type JWTClaims = domain.JWTClaims

type AuthMiddleware struct {
	secret string
	rdb    *redis.Client
}

func NewAuthMiddleware(secret string, rdb *redis.Client) *AuthMiddleware {
	return &AuthMiddleware{
		secret: secret,
		rdb:    rdb,
	}
}

// Authenticate memverifikasi Bearer JWT token dan memeriksa blacklist di Redis.
//
// URUTAN:
//  1. Ekstrak token dari header
//  2. Parse + verifikasi signature (extract claims termasuk jti)
//  3. Cek blacklist berdasarkan jti (bukan raw token!)
//  4. Simpan claims ke context
//
// Perubahan dari versi sebelumnya:
//   - Blacklist pakai jti (UUID) → hemat memory Redis drastis (~800 byte → ~40 byte)
//   - Cek blacklist SETELAH parse → tidak perlu query Redis untuk token invalid
func (m *AuthMiddleware) Authenticate() fiber.Handler {
	return func(c *fiber.Ctx) error {
		tokenStr, err := ExtractBearerToken(c.Get("Authorization"))
		if err != nil {
			return response.Unauthorized(c, err.Error())
		}

		claims, err := m.validateToken(tokenStr)
		if err != nil {
			return response.Unauthorized(c, "Token tidak valid atau telah kedaluwarsa")
		}

		// Cek blacklist by jti (Claims.ID). T4: error Redis = fail-closed
		// (503), bukan meloloskan token yang mungkin sudah dicabut.
		revoked, err := m.isBlacklisted(c.Context(), claims.ID)
		if err != nil {
			return response.FromError(c, domain.NewUnavailableError(
				"Verifikasi sesi sedang tidak tersedia. Coba beberapa saat lagi"))
		}
		if revoked {
			return response.Unauthorized(c, "Token telah dicabut. Silakan login kembali")
		}

		storeClaims(c, claims)
		return c.Next()
	}
}

// ExtractBearerToken memvalidasi format header Authorization: Bearer <token>.
// Diekspor (L-3) agar handler memakai satu helper yang sama, bukan duplikat.
func ExtractBearerToken(authHeader string) (string, error) {
	if authHeader == "" {
		return "", errors.New("Header Authorization diperlukan")
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", errors.New("Format token tidak valid. Gunakan format 'Bearer <token>'")
	}

	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", errors.New("Token kosong")
	}
	return token, nil
}

// isBlacklisted memeriksa apakah jti token sudah dicabut via logout.
// Key: "blacklist:jti:<uuid>" — bukan token penuh.
//
// T4 (fail-closed): error selain "key tidak ada" dikembalikan sebagai error
// agar Authenticate menolak request (503) alih-alih meloloskan token yang
// mungkin sudah dicabut saat Redis sedang bermasalah. rdb == nil hanya
// mungkin di dev (produksi menolak start tanpa Redis).
func (m *AuthMiddleware) isBlacklisted(ctx context.Context, jti string) (bool, error) {
	if m.rdb == nil || jti == "" {
		return false, nil
	}

	reqCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	key := fmt.Sprintf("blacklist:jti:%s", jti)
	val, err := m.rdb.Get(reqCtx, key).Result()
	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("gagal memeriksa blacklist: %w", err)
	}
	return val != "", nil
}

// validateToken memverifikasi signature JWT dan mengekstrak claims.
// WAJIB cek signing method = HMAC, mencegah alg=none attack.
func (m *AuthMiddleware) validateToken(tokenStr string) (*JWTClaims, error) {
	claims := &JWTClaims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, m.keyFunc,
		jwt.WithValidMethods([]string{"HS256"}),
	)
	if err != nil || !token.Valid {
		return nil, errors.New("invalid token")
	}
	if claims.ID == "" {
		// Token tanpa jti = tidak bisa di-blacklist. Tolak untuk konsistensi.
		return nil, errors.New("token tidak memiliki jti")
	}
	return claims, nil
}

func (m *AuthMiddleware) keyFunc(t *jwt.Token) (interface{}, error) {
	if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
		return nil, fmt.Errorf("metode signing tidak valid: %v", t.Header["alg"])
	}
	return []byte(m.secret), nil
}

// ============================================================
// Context helpers
// ============================================================

func storeClaims(c *fiber.Ctx, claims *JWTClaims) {
	c.Locals("user", claims)
	c.Locals("user_id", claims.UserID)
	c.Locals("role", claims.Role)
}

// GetUser mengambil claims dari context. Return nil jika tidak ada.
// Gunakan ini di handler yang sudah dipasangi Authenticate().
func GetUser(c *fiber.Ctx) *JWTClaims {
	u, ok := c.Locals("user").(*JWTClaims)
	if !ok {
		return nil
	}
	return u
}