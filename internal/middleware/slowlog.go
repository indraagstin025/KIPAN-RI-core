package middleware

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"
)

// SlowRequestLogger mencatat request yang durasinya melewati ambang ke log
// (monitoring produksi). Dilaporkan best-effort tanpa mengubah respons.
func SlowRequestLogger(threshold time.Duration) fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		if d := time.Since(start); d > threshold {
			log.Warn().
				Dur("duration_ms", d).
				Str("method", c.Method()).
				Str("path", c.Path()).
				Int("status", c.Response().StatusCode()).
				Msg("request lambat")
		}
		return err
	}
}
