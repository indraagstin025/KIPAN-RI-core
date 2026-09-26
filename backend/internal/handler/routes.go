package handler

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"

	"github.com/kipan-indonesia/sim-kipan-core/config"
	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/middleware"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
	"github.com/kipan-indonesia/sim-kipan-core/internal/service"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/response"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/validator"
)

// RegisterRoutes mendaftarkan seluruh endpoint aplikasi ke Fiber router
func RegisterRoutes(
	app *fiber.App,
	cfg *config.Config,
	db *sqlx.DB,
	rdb *redis.Client,
	val *validator.CustomValidator,
) {
	// Global Security Headers
	app.Use(middleware.SecurityHeaders())

	// Health check endpoint (tidak butuh DB)
	healthHandler := NewHealthHandler(cfg.App.Name, cfg.App.Env, db, rdb)
	app.Get("/health", healthHandler.Check)

	// Jika DB tidak aktif (mode dev tanpa DB), hentikan di sini
	if db == nil {
		return
	}

	// ============================================================
	// Dependency wiring (Simple DI manual)
	// ============================================================
	userRepo := repository.NewUserRepository(db)
	authService := service.NewAuthService(cfg, userRepo, rdb)
	// Flag Secure cookie diambil dari APP_ENV (fail-closed): hanya development
	// yang boleh tanpa Secure. Jangan diturunkan dari header request.
	secureCookie := cfg.App.Env != "development"
	authHandler := NewAuthHandler(authService, val, cfg.Auth.RefreshTokenTTL, secureCookie)
	authMiddleware := middleware.NewAuthMiddleware(cfg.Auth.AccessTokenSecret, rdb)

	// ============================================================
	// Registrasi rute v1
	// ============================================================
	v1 := app.Group("/api/v1")

	// PERBAIKAN: teruskan rdb ke registerAuthRoutes
	registerAuthRoutes(v1, rdb, authHandler, authMiddleware)
	registerAdminRoutes(v1, authMiddleware)
}

// registerAuthRoutes mendaftarkan endpoint autentikasi.
//
// PERBAIKAN: parameter rdb ditambahkan agar AuthRateLimiter bisa memakai Redis storage.
func registerAuthRoutes(
	v1 fiber.Router,
	rdb *redis.Client,
	authHandler *AuthHandler,
	authMiddleware *middleware.AuthMiddleware,
) {
	auth := v1.Group("/auth")

	// Layer 1: Rate limit per IP (semua endpoint auth)
	authLimiter := middleware.AuthRateLimiter(rdb, 20, 1*time.Minute)

	// Layer 2: Rate limit per email (khusus login)
	loginAttemptLimiter := middleware.LoginAttemptLimiter(rdb, 5, 15*time.Minute)

	auth.Post("/login", authLimiter, loginAttemptLimiter, authHandler.Login)
	auth.Post("/refresh", authLimiter, authHandler.RefreshToken)
	auth.Post("/logout", authMiddleware.Authenticate(), authHandler.Logout)
	auth.Get("/me", authMiddleware.Authenticate(), authHandler.Me)
	auth.Put("/password", authMiddleware.Authenticate(), authHandler.ChangePassword)
}

// registerAdminRoutes mendaftarkan endpoint khusus admin dengan RBAC & scope wilayah.
func registerAdminRoutes(v1 fiber.Router, authMiddleware *middleware.AuthMiddleware) {
	admin := v1.Group("/admin", authMiddleware.Authenticate())

	// Verifikasi Scoping Wilayah
	admin.Get("/me-scope", middleware.ScopeWilayah(), handleMeScope)

	// Role-Based Guards
	admin.Get("/super-only", middleware.RequireRoles(domain.RoleSuperAdmin), handleSuperOnly)
	admin.Get("/nasional-or-super",
		middleware.RequireRoles(domain.RoleSuperAdmin, domain.RoleAdminNasional),
		handleNasionalOrSuper,
	)
}

// ============================================================
// Test handlers (endpoint dummy untuk verifikasi RBAC/Scope)
// ============================================================

func handleMeScope(c *fiber.Ctx) error {
	claims := middleware.GetUser(c)
	if claims == nil {
		return response.Unauthorized(c, "Tidak terotentikasi")
	}

	// Baca WilayahScope yang sudah di-set middleware
	scope := middleware.GetWilayahScope(c)

	return response.Success(c, "Akses otorisasi wilayah terverifikasi", fiber.Map{
		"user_id":           claims.UserID,
		"email":             claims.Email,
		"role":              claims.Role,
		"provinsi_id_claim": claims.ProvinsiID,
		"kabupaten_id_claim": claims.KabupatenID,
		"filter_provinsi_id": scope.ProvinsiID,
		"filter_kabupaten_id": scope.KabupatenID,
		"is_nasional_scope":  scope.IsNasional(),
	})
}

func handleSuperOnly(c *fiber.Ctx) error {
	return response.Success(c, "Akses khusus SUPER_ADMIN berhasil dibuka", nil)
}

func handleNasionalOrSuper(c *fiber.Ctx) error {
	return response.Success(c, "Akses level nasional/super admin berhasil dibuka", nil)
}