package handler

import (
	"context"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"github.com/kipan-indonesia/sim-kipan-core/config"
	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/middleware"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
	"github.com/kipan-indonesia/sim-kipan-core/internal/service"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/response"
	storagepkg "github.com/kipan-indonesia/sim-kipan-core/pkg/storage"
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
	auditRepo := repository.NewAuditLogRepository(db)
	pendaftaranRepo := repository.NewPendaftaranRepository(db)
	authService := service.NewAuthService(cfg, userRepo, rdb, auditRepo)
	anggotaRepo := repository.NewAnggotaRepository(db)
	storageService := wireStorageService(cfg, auditRepo)
	wilayahRepo := repository.NewWilayahRepository(db)
	pendaftaranService := service.NewPendaftaranService(cfg, pendaftaranRepo, anggotaRepo, auditRepo, storageService, wilayahRepo)
	// Flag Secure cookie diambil dari APP_ENV (fail-closed): hanya development
	// yang boleh tanpa Secure. Jangan diturunkan dari header request.
	secureCookie := cfg.App.Env != "development"
	authHandler := NewAuthHandler(authService, val, cfg.Auth.RefreshTokenTTL, secureCookie)
	pendaftaranHandler := NewPendaftaranHandler(pendaftaranService, val)
	storageHandler := NewStorageHandler(storageService, val)
	authMiddleware := middleware.NewAuthMiddleware(cfg.Auth.AccessTokenSecret, rdb)

	// ============================================================
	// Registrasi rute v1
	// ============================================================
	v1 := app.Group("/api/v1")

	registerAuthRoutes(v1, rdb, authHandler, authMiddleware)
	registerMembershipRoutes(v1, rdb, authMiddleware, pendaftaranHandler)
	registerStorageRoutes(v1, rdb, authMiddleware, storageHandler)
	registerAdminRoutes(v1, authMiddleware)
}

// wireStorageService membangun S3 client + StorageService.
//
// Fail-closed: bila endpoint/kredensial tidak dikonfigurasi, service tetap
// dibangun dengan client nil sehingga RequestUploadPresign/RequestViewPresign
// mengembalikan 503 (bukan bypass diam-diam). Bucket disiapkan otomatis
// hanya di non-production.
func wireStorageService(cfg *config.Config, auditRepo repository.AuditLogRepository) *service.StorageService {
	var client *storagepkg.Client
	if strings.TrimSpace(cfg.Storage.Endpoint) != "" {
		c, err := storagepkg.NewClient(
			cfg.Storage.Endpoint,
			cfg.Storage.Region,
			cfg.Storage.AccessKeyID,
			cfg.Storage.SecretAccessKey,
		)
		if err != nil {
			log.Warn().Err(err).Msg("Storage tidak aktif (konfigurasi tidak lengkap) — presign fail-closed 503")
		} else {
			client = c
		}
	} else {
		log.Warn().Msg("STORAGE_ENDPOINT kosong — presign upload/view nonaktif (fail-closed 503)")
	}

	svc := service.NewStorageService(cfg, client, auditRepo)
	if client != nil && cfg.App.Env != "production" {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		svc.EnsureBuckets(ctx)
	}
	return svc
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
	authLimiter := middleware.AuthRateLimiter(rdb, "auth_rate:ip:", 20, 1*time.Minute)

	// Layer 2: Rate limit per email (khusus login)
	// 20/15 mnt per pasangan email+IP (BE-003): longgar per akun agar
	// lockout pihak ketiga tidak mungkin, ketat per IP via authLimiter.
	loginAttemptLimiter := middleware.LoginAttemptLimiter(rdb, 20, 15*time.Minute)

	auth.Post("/login", authLimiter, loginAttemptLimiter, authHandler.Login)
	auth.Post("/refresh", authLimiter, authHandler.RefreshToken)
	// Logout SENGAJA tanpa Authenticate(): sesi harus bisa dicabut via refresh
	// cookie bahkan saat access token sudah kedaluwarsa/invalid. Service
	// mencabut refresh family + blacklist access secara best-effort dan handler
	// selalu mengembalikan sukses (idempoten, anti-oracle).
	auth.Post("/logout", authLimiter, authHandler.Logout)
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

func registerMembershipRoutes(
	v1 fiber.Router,
	rdb *redis.Client,
	authMiddleware *middleware.AuthMiddleware,
	handler *PendaftaranHandler,
) {
	// Limiter publik (anti enumerasi/spam) + limiter mutasi sensitif.
	// Redis-backed agar konsisten multi-instance (seperti auth).
	// Prefix berbeda: berbagi prefix = berbagi kuota (429 prematur).
	publicLimiter := middleware.AuthRateLimiter(rdb, "mem_pub:ip:", 30, 1*time.Minute)
	adminMutasiLimiter := middleware.AuthRateLimiter(rdb, "mem_mut:ip:", 20, 1*time.Minute)

	// Limiter token revisi per-nomor (BE-001): 3 permintaan / 24 jam per
	// nomor pendaftaran, di atas limiter per-IP grup ini.
	revTokenLimiter := middleware.RevisionTokenLimiter(rdb, 3, 24*time.Hour)

	public := v1.Group("/pendaftaran")
	public.Use(publicLimiter)
	public.Post("", handler.Submit)
	public.Get("/track/:nomor", handler.TrackStatus)
	public.Post("/revisi/request-token", revTokenLimiter, handler.RequestRevisionToken)
	public.Put("/revisi/:nomor", handler.SubmitRevision)
	public.Get("/kta/:nia", handler.VerifyKTA)

	adminPendaftaran := v1.Group("/admin/pendaftaran",
		authMiddleware.Authenticate(),
		middleware.RequireRoles(
			domain.RoleSuperAdmin,
			domain.RoleAdminNasional,
			domain.RoleAdminProvinsi,
			domain.RoleAdminKabupaten,
		),
		adminMutasiLimiter,
	)
	adminPendaftaran.Get("", middleware.ScopeWilayah(), handler.ListQueue)
	adminPendaftaran.Get("/:id", middleware.ScopeWilayah(), handler.Detail)
	adminPendaftaran.Post("/:id/verifikasi", middleware.ScopeWilayah(), handler.Verify)
	adminPendaftaran.Post("/:id/perbaikan", middleware.ScopeWilayah(), handler.RequestRevision)
	adminPendaftaran.Post("/:id/tolak", middleware.ScopeWilayah(), handler.Reject)
	adminPendaftaran.Post("/:id/setujui", middleware.ScopeWilayah(), handler.Approve)
}

// registerStorageRoutes mendaftarkan endpoint tiket presigned S3.
//
// POST /storage/presign-upload publik (pendaftar belum punya akun) tetapi
// di-rate-limit ketat; GET /storage/presign-view wajib auth + tercatat audit.
func registerStorageRoutes(
	v1 fiber.Router,
	rdb *redis.Client,
	authMiddleware *middleware.AuthMiddleware,
	handler *StorageHandler,
) {
	uploadLimiter := middleware.AuthRateLimiter(rdb, "stor_up:ip:", 30, 1*time.Minute)

	public := v1.Group("/storage")
	public.Post("/presign-upload", uploadLimiter, handler.PresignUpload)
	public.Get("/presign-view", authMiddleware.Authenticate(), handler.PresignView)
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
		"user_id":             claims.UserID,
		"email":               claims.Email,
		"role":                claims.Role,
		"provinsi_id_claim":   claims.ProvinsiID,
		"kabupaten_id_claim":  claims.KabupatenID,
		"filter_provinsi_id":  scope.ProvinsiID,
		"filter_kabupaten_id": scope.KabupatenID,
		"is_nasional_scope":   scope.IsNasional(),
	})
}

func handleSuperOnly(c *fiber.Ctx) error {
	return response.Success(c, "Akses khusus SUPER_ADMIN berhasil dibuka", nil)
}

func handleNasionalOrSuper(c *fiber.Ctx) error {
	return response.Success(c, "Akses level nasional/super admin berhasil dibuka", nil)
}
