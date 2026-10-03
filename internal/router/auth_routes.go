package router

// auth_routes.go mendaftarkan endpoint autentikasi (login/refresh/logout,
// reset & set kata sandi, profil, ganti kata sandi).

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"

	"github.com/kipan-indonesia/sim-kipan-core/internal/handler"
	"github.com/kipan-indonesia/sim-kipan-core/internal/middleware"
)

// registerAuthRoutes mendaftarkan endpoint di bawah /auth beserta rate-limit
// berlapis (per-IP, per-email login, dan per-email reset kata sandi).
func registerAuthRoutes(
	v1 fiber.Router,
	rdb *redis.Client,
	authHandler *handler.AuthHandler,
	pwResetHandler *handler.PasswordResetHandler,
	authMiddleware *middleware.AuthMiddleware,
) {
	auth := v1.Group("/auth")

	// Layer 1: rate limit per IP untuk semua endpoint auth.
	authLimiter := middleware.RateLimit(rdb, "auth")
	// Layer 2: rate limit per email (khusus login, anti brute force akun).
	loginAttemptLimiter := middleware.LoginAttemptLimiter(rdb, 20, 15*time.Minute)
	// Layer 3: rate limit per email reset (anti email bombing).
	pwResetLimiter := middleware.PasswordResetLimiter(rdb, 3, 15*time.Minute)

	auth.Post("/login", authLimiter, loginAttemptLimiter, authHandler.Login)
	auth.Post("/refresh", authLimiter, authHandler.RefreshToken)
	// Logout sengaja tanpa Authenticate(): sesi harus bisa dicabut via cookie
	// refresh walau access token sudah kedaluwarsa; handler selalu sukses.
	auth.Post("/logout", authLimiter, authHandler.Logout)
	auth.Post("/forgot-password", authLimiter, pwResetLimiter, pwResetHandler.Forgot)
	auth.Post("/reset-password", authLimiter, pwResetHandler.Reset)
	// Set-password: tukar token "buat kata sandi" (dari antrian email) dengan sandi baru.
	auth.Post("/set-password", authLimiter, pwResetHandler.Set)
	auth.Get("/me", authMiddleware.Authenticate(), authHandler.Me)
	auth.Put("/password", authMiddleware.Authenticate(), authHandler.ChangePassword)
}
