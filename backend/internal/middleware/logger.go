package middleware

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"
)

// RequestLogger mencatat access log terstruktur untuk setiap request.
//
// CATATAN PENTING:
//   - c.IP() akan mengembalikan IP client asli HANYA JIKA main.go mengonfigurasi
//     EnableTrustedProxyCheck + TrustedProxies. Jika tidak, c.IP() return IP proxy.
//   - Log ini berada DI LUAR chain Authenticate(), jadi user_id tersedia SETELAH
//     c.Next() karena middleware auth sudah menyimpan claims ke context.
func RequestLogger() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		path := c.Path()
		method := c.Method()
		reqID := c.GetRespHeader("X-Request-ID")

		err := c.Next()

		latency := time.Since(start)
		statusCode := c.Response().StatusCode()

		event := log.Info()
		switch {
		case statusCode >= 500:
			event = log.Error()
		case statusCode >= 400:
			event = log.Warn()
		}

		event = event.
			Int("status", statusCode).
			Str("method", method).
			Str("path", path).
			Str("ip", c.IP()).
			Str("request_id", reqID).
			Dur("latency", latency)

		// Tambahkan user_id & role jika user sudah terautentikasi.
		// Jika request ditolak di auth middleware, GetUser() = nil → skip.
		if claims := GetUser(c); claims != nil {
			event = event.
				Str("user_id", claims.UserID).
				Str("role", string(claims.Role))
		}

		event.Msg("HTTP Request")

		return err
	}
}