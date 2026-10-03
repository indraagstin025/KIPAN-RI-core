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
	"github.com/kipan-indonesia/sim-kipan-core/internal/gateway"
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
	// Monitoring: catat request lambat (>200ms) untuk tuning skala (§10.2).
	app.Use(middleware.SlowRequestLogger(200 * time.Millisecond))

	// Health check endpoint (tidak butuh DB)
	healthHandler := NewHealthHandler(cfg.App.Name, cfg.App.Env, db, rdb)
	app.Get("/health", healthHandler.Check)
	// BE-004 / #2: detail internal (env + status dependensi) TIDAK boleh
	// terbuka. Bila INTERNAL_HEALTH_TOKEN diisi → wajib header X-Internal-Token.
	// Bila kosong: hanya didaftarkan di non-production (dev). Di production
	// tanpa token, route TIDAK didaftarkan sama sekali (fail-closed 404).
	internal := app.Group("/internal")
	switch {
	case strings.TrimSpace(cfg.App.InternalHealthToken) != "":
		internal.Use(middleware.RequireInternalToken(cfg.App.InternalHealthToken))
		internal.Get("/health", healthHandler.Detail)
	case cfg.App.Env != "production":
		internal.Get("/health", healthHandler.Detail)
	default:
		log.Warn().Msg("/internal/health TIDAK didaftarkan: production tanpa INTERNAL_HEALTH_TOKEN")
	}

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
	notifRepo := repository.NewNotificationRepository(db)
	authService := service.NewAuthService(cfg, service.AuthDeps{
		UserRepo: userRepo, RDB: rdb, AuditRepo: auditRepo,
	})
	anggotaRepo := repository.NewAnggotaRepository(db)
	storageService := wireStorageService(cfg, auditRepo, repository.NewDocumentRepository(db))
	wilayahRepo := repository.NewWilayahRepository(db)
	ktaSvc := service.NewKTAService(cfg, service.KTADeps{
		AnggotaRepo: anggotaRepo, DocStore: storageService, AuditRepo: auditRepo,
	})
	// Batch 2: gateway WA nyata (Fonnte) di produksi; LogGateway hanya
	// dev/test. Produksi dijamin provider 'fonnte' oleh validasi config.
	waGateway := wireWAGateway(cfg)
	otpSvc := service.NewOTPService(cfg, service.OtpDeps{
		RDB: rdb, Gateway: waGateway,
	})
	pendaftaranService := service.NewPendaftaranService(cfg, service.PendaftaranDeps{
		Repo: pendaftaranRepo, AnggotaRepo: anggotaRepo, AuditRepo: auditRepo,
		StorageSvc: storageService, WilayahRepo: wilayahRepo, NotifRepo: notifRepo,
		OTPSvc: otpSvc, WAGateway: waGateway, ListRepo: repository.NewListKeysetRepository(db),
	})
	// Flag Secure cookie diambil dari APP_ENV (fail-closed): hanya development
	// yang boleh tanpa Secure. Jangan diturunkan dari header request.
	secureCookie := cfg.App.Env != "development"
	authHandler := NewAuthHandler(authService, val, cfg.Auth.RefreshTokenTTL, secureCookie,
		cfg.Auth.CookieSameSite, cfg.Auth.CookiePath, cfg.Auth.CookieDomain)
	// Batch 3: email SMTP (Mailpit dev / Mailtrap sandbox / produksi).
	// Host kosong = LogMailSender (HANYA dev; produksi fail-fast).
	mailSender := wireMailSender(cfg)
	revisionSvc := service.NewRevisionService(cfg, service.RevisionDeps{
		Repo: pendaftaranRepo, StorageSvc: storageService, AuditRepo: auditRepo,
		Mail: mailSender,
	})
	verificationSvc := service.NewVerificationService(cfg, service.VerificationDeps{
		Repo: pendaftaranRepo, AnggotaRepo: anggotaRepo, UserRepo: userRepo,
		AuditRepo: auditRepo, KTASvc: ktaSvc,
		NotifRepo: notifRepo, Mail: mailSender,
	})
	pendaftaranHandler := NewPendaftaranHandler(pendaftaranService, revisionSvc, verificationSvc, val)
	otpHandler := NewOTPHandler(otpSvc)
	pwResetSvc := service.NewPasswordResetService(cfg, service.PasswordResetDeps{
		UserRepo: userRepo, RDB: rdb, Mail: mailSender, AuditRepo: auditRepo,
	})
	pwResetHandler := NewPasswordResetHandler(pwResetSvc, val)
	storageHandler := NewStorageHandler(storageService, val)
	wilayahService := service.NewWilayahService(wilayahRepo)
	wilayahHandler := NewWilayahHandler(wilayahService)
	wilayahAdminHandler := NewWilayahAdminHandler(service.NewWilayahAdminService(
		repository.NewWilayahAdminRepository(db), auditRepo,
	))
	userAdminHandler := NewUserAdminHandler(service.NewUserAdminService(service.UserAdminDeps{
		UserRepo: userRepo, AdminRepo: repository.NewUserAdminRepository(db),
		WilayahRepo: wilayahRepo, AuditRepo: auditRepo,
	}))
	ktaHandler := NewKTAHandler(ktaSvc)
	anggotaService := service.NewAnggotaService(cfg, service.AnggotaDeps{
		AnggotaRepo: anggotaRepo, WilayahRepo: wilayahRepo,
		UserRepo: userRepo, AuditRepo: auditRepo,
		ListRepo: repository.NewListKeysetRepository(db),
	})
	anggotaHandler := NewAnggotaHandler(anggotaService)
	notifHandler := NewNotificationHandler(service.NewNotificationService(cfg, notifRepo))
	dashboardHandler := NewDashboardHandler(service.NewDashboardService(repository.NewDashboardRepository(db)))
	kepengurusanHandler := NewKepengurusanHandler(service.NewKepengurusanService(cfg, service.KepengurusanDeps{
		JabatanRepo:  repository.NewJabatanRepository(db),
		SKRepo:       repository.NewSKRepository(db),
		PengurusRepo: repository.NewPengurusRepository(db),
		AnggotaRepo:  anggotaRepo, UserRepo: userRepo, AuditRepo: auditRepo,
		WilayahRepo: wilayahRepo, Mail: mailSender,
	}))
	authMiddleware := middleware.NewAuthMiddleware(cfg.Auth.AccessTokenSecret, rdb)

	// ============================================================
	// Registrasi rute v1
	// ============================================================
	v1 := app.Group("/api/v1")

	registerAuthRoutes(v1, rdb, authHandler, pwResetHandler, authMiddleware)
	registerMembershipRoutes(v1, rdb, authMiddleware, pendaftaranHandler, otpHandler)
	registerStorageRoutes(v1, rdb, authMiddleware, storageHandler)
	registerWilayahRoutes(v1, rdb, wilayahHandler)
	registerWilayahAdminRoutes(v1, rdb, authMiddleware, wilayahAdminHandler)
	registerAnggotaRoutes(v1, rdb, authMiddleware, ktaHandler, anggotaHandler)
	registerNotificationRoutes(v1, authMiddleware, notifHandler)
	registerUserRoutes(v1, rdb, authMiddleware, ktaHandler)
	registerUserAdminRoutes(v1, rdb, authMiddleware, userAdminHandler)
	registerKepengurusanRoutes(v1, rdb, authMiddleware, kepengurusanHandler)
	registerDashboardRoutes(v1, rdb, authMiddleware, dashboardHandler)
	registerAdminRoutes(v1, cfg.App.Env, authMiddleware)
}

// wireWAGateway memilih implementasi WAGateway dari config: Fonnte untuk
// produksi, LogGateway untuk dev/test (fail-fast produksi dijamin config).
func wireWAGateway(cfg *config.Config) gateway.WAGateway {
	if cfg.WA.Provider == "fonnte" {
		log.Info().Msg("WA gateway aktif: Fonnte")
		return gateway.NewFonnteGateway(cfg.WA.FonnteToken)
	}
	log.Warn().Msg("WA gateway: log-only (HANYA dev/test — jangan dipakai di produksi)")
	return gateway.NewLogGateway()
}

// wireMailSender memilih implementasi MailSender: SMTP bila MAIL_HOST diisi
// (Mailpit dev / Mailtrap sandbox / produksi), selain itu LogMailSender
// (dev/test; produksi dijamin oleh validasi config).
func wireMailSender(cfg *config.Config) gateway.MailSender {
	if cfg.Mail.Enabled() {
		log.Info().
			Str("host", cfg.Mail.Host).
			Int("port", cfg.Mail.Port).
			Msg("Email gateway aktif: SMTP")
		return gateway.NewSMTPSender(gateway.MailConfig{
			Host:      cfg.Mail.Host,
			Port:      cfg.Mail.Port,
			Username:  cfg.Mail.Username,
			Password:  cfg.Mail.Password,
			FromEmail: cfg.Mail.FromEmail,
			FromName:  cfg.Mail.FromName,
		})
	}
	log.Warn().Msg("Email gateway: log-only (set MAIL_HOST untuk SMTP; HANYA dev/test)")
	return gateway.NewLogMailSender()
}

// registerAnggotaRoutes mendaftarkan endpoint kader: cek publik minimal
// (pengganti cek-anggota lama, rate-limit ketat) + daftar admin teraudit.
func registerAnggotaRoutes(
	v1 fiber.Router,
	rdb *redis.Client,
	authMiddleware *middleware.AuthMiddleware,
	handler *KTAHandler,
	anggotaHandler *AnggotaHandler,
) {
	public := v1.Group("/anggota", middleware.RateLimit(rdb, "agt_pub"))
	public.Get("/cek", anggotaHandler.CheckPublic)

	adminAnggota := v1.Group("/admin/anggota",
		authMiddleware.Authenticate(),
		middleware.RequireRoles(
			domain.RoleSuperAdmin,
			domain.RoleAdminNasional,
			domain.RoleAdminProvinsi,
			domain.RoleAdminKabupaten,
		),
	)
	adminAnggota.Get("", middleware.ScopeWilayah(), anggotaHandler.List)
	adminAnggota.Get("/:id", middleware.ScopeWilayah(), anggotaHandler.Detail)
	adminAnggota.Get("/:id/kta", middleware.ScopeWilayah(), handler.DownloadKTA)
	// T1: reset password akun USER anggota (password tampil sekali).
	adminAnggota.Post("/:id/reset-password", middleware.ScopeWilayah(), anggotaHandler.ResetPassword)
}

// registerUserRoutes mendaftarkan endpoint layanan mandiri akun USER
// (Batch 2): hanya role USER, otorisasi kepemilikan di service via
// anggota.user_id. Rate-limit agt_pub seperti cek anggota publik.
func registerUserRoutes(
	v1 fiber.Router,
	rdb *redis.Client,
	authMiddleware *middleware.AuthMiddleware,
	handler *KTAHandler,
) {
	user := v1.Group("/user",
		authMiddleware.Authenticate(),
		middleware.RequireRoles(domain.RoleUser),
		middleware.RateLimit(rdb, "agt_pub"),
	)
	user.Get("/kta", handler.DownloadMyKTA)
}

// wireStorageService membangun S3 client + StorageService.
//
// Fail-closed: bila endpoint/kredensial tidak dikonfigurasi, service tetap
// dibangun dengan client nil sehingga RequestUploadPresign/RequestViewPresign
// mengembalikan 503 (bukan bypass diam-diam). Bucket disiapkan otomatis
// hanya di non-production.
func wireStorageService(cfg *config.Config, auditRepo repository.AuditLogRepository, docRepo repository.DocumentRepository) *service.StorageService {
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

	svc := service.NewStorageService(cfg, client, auditRepo, docRepo)
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
	pwResetHandler *PasswordResetHandler,
	authMiddleware *middleware.AuthMiddleware,
) {
	auth := v1.Group("/auth")

	// Layer 1: Rate limit per IP (semua endpoint auth)
	authLimiter := middleware.RateLimit(rdb, "auth")

	// Layer 2: Rate limit per email (khusus login)
	// 20/15 mnt per pasangan email+IP (BE-003): longgar per akun agar
	// lockout pihak ketiga tidak mungkin, ketat per IP via authLimiter.
	loginAttemptLimiter := middleware.LoginAttemptLimiter(rdb, 20, 15*time.Minute)

	// Batch 3: reset password — limiter per-email+IP (anti email bombing).
	pwResetLimiter := middleware.PasswordResetLimiter(rdb, 3, 15*time.Minute)

	auth.Post("/login", authLimiter, loginAttemptLimiter, authHandler.Login)
	auth.Post("/refresh", authLimiter, authHandler.RefreshToken)
	// Logout SENGAJA tanpa Authenticate(): sesi harus bisa dicabut via refresh
	// cookie bahkan saat access token sudah kedaluwarsa/invalid. Service
	// mencabut refresh family + blacklist access secara best-effort dan handler
	// selalu mengembalikan sukses (idempoten, anti-oracle).
	auth.Post("/logout", authLimiter, authHandler.Logout)
	// Batch 3: reset kata sandi mandiri via email (anti-enumeration).
	auth.Post("/forgot-password", authLimiter, pwResetLimiter, pwResetHandler.Forgot)
	auth.Post("/reset-password", authLimiter, pwResetHandler.Reset)
	auth.Get("/me", authMiddleware.Authenticate(), authHandler.Me)
	auth.Put("/password", authMiddleware.Authenticate(), authHandler.ChangePassword)
}

// registerAdminRoutes mendaftarkan endpoint khusus admin dengan RBAC & scope wilayah.
// registerAdminRoutes mendaftarkan endpoint admin. #6: endpoint UJI
// (super-only, nasional-or-super) hanya di non-production; me-scope tetap
// ada karena dipakai dasbor (hanya mengembalikan klaim pemanggil sendiri).
func registerAdminRoutes(v1 fiber.Router, env string, authMiddleware *middleware.AuthMiddleware) {
	admin := v1.Group("/admin", authMiddleware.Authenticate())

	// Verifikasi Scoping Wilayah (dipakai dasbor).
	admin.Get("/me-scope", middleware.ScopeWilayah(), handleMeScope)

	// Role-Based Guards — endpoint uji, non-production saja.
	if env != "production" {
		admin.Get("/super-only", middleware.RequireRoles(domain.RoleSuperAdmin), handleSuperOnly)
		admin.Get("/nasional-or-super",
			middleware.RequireRoles(domain.RoleSuperAdmin, domain.RoleAdminNasional),
			handleNasionalOrSuper,
		)
	}
}

// registerDashboardRoutes mendaftarkan ringkasan analitik admin (ter-scope).
func registerDashboardRoutes(
	v1 fiber.Router,
	rdb *redis.Client,
	authMiddleware *middleware.AuthMiddleware,
	handler *DashboardHandler,
) {
	grp := v1.Group("/admin",
		authMiddleware.Authenticate(),
		middleware.RequireRoles(
			domain.RoleSuperAdmin,
			domain.RoleAdminNasional,
			domain.RoleAdminProvinsi,
			domain.RoleAdminKabupaten,
		),
	)
	grp.Get("/dashboard", handler.Get)
}

// registerKepengurusanRoutes mendaftarkan endpoint SK, jabatan, dan pengurus.
// Otorisasi rinci (yurisdiksi + rantai approval) ditegakkan di service.
func registerKepengurusanRoutes(
	v1 fiber.Router,
	rdb *redis.Client,
	authMiddleware *middleware.AuthMiddleware,
	handler *KepengurusanHandler,
) {
	adminRoles := middleware.RequireRoles(
		domain.RoleSuperAdmin,
		domain.RoleAdminNasional,
		domain.RoleAdminProvinsi,
		domain.RoleAdminKabupaten,
	)
	grp := v1.Group("/admin",
		authMiddleware.Authenticate(),
		adminRoles,
		middleware.MutatingRateLimit(rdb, "kep_mut"),
	)

	grp.Get("/jabatan", handler.ListJabatan)
	grp.Get("/sk", handler.ListSK)
	grp.Get("/sk/:id", handler.GetSK)
	grp.Post("/sk", handler.CreateSK)
	grp.Post("/sk/:id/approve", handler.ApproveSK)
	grp.Post("/sk/:id/status", handler.SetSKStatus)
	grp.Post("/sk/:id/pengurus", handler.AddPengurus)
	grp.Delete("/sk/:id/pengurus/:pengurusId", handler.RemovePengurus)
	grp.Get("/pengurus", handler.ListPengurus)
	grp.Get("/pengurus/stats", handler.PengurusStats)
	grp.Get("/pengurus/promosi", handler.ListPromosi)
	grp.Patch("/pengurus/:id", handler.UpdatePengurusStatus)
	grp.Patch("/pengurus/:id/jabatan", handler.UpdatePengurusJabatan)

	// Master jabatan (mutasi) hanya Nasional/Super.
	master := grp.Group("/jabatan", middleware.RequireRoles(domain.RoleSuperAdmin, domain.RoleAdminNasional))
	master.Post("", handler.CreateJabatan)
	master.Put("/:id", handler.UpdateJabatan)
}

func registerMembershipRoutes(
	v1 fiber.Router,
	rdb *redis.Client,
	authMiddleware *middleware.AuthMiddleware,
	handler *PendaftaranHandler,
	otpHandler *OTPHandler,
) {
	// Limiter publik (anti enumerasi/spam) + limiter mutasi sensitif.
	// Redis-backed agar konsisten multi-instance (seperti auth).
	// Prefix berbeda: berbagi prefix = berbagi kuota (429 prematur).
	publicLimiter := middleware.RateLimit(rdb, "mem_pub")
	adminMutasiLimiter := middleware.MutatingRateLimit(rdb, "mem_mut")

	// Limiter token revisi per-nomor (BE-001): 3 permintaan / 24 jam per
	// nomor pendaftaran, di atas limiter per-IP grup ini.
	revTokenLimiter := middleware.RevisionTokenLimiter(rdb, 3, 24*time.Hour)

	// Limiter pelacakan per-nomor (SEC-TRACK-PII): nomor REG sekuensial mudah
	// dienumerasi, jadi limiter per-IP (mem_pub) saja tidak cukup.
	trackLimiter := middleware.TrackLimiter(rdb, 15, time.Minute)

	public := v1.Group("/pendaftaran")
	public.Use(publicLimiter)
	public.Post("", handler.Submit)
	public.Get("/track/:nomor", trackLimiter, handler.TrackStatus)
	public.Post("/revisi/request-token", revTokenLimiter, handler.RequestRevisionToken)
	public.Put("/revisi/:nomor", handler.SubmitRevision)
	public.Get("/kta/:nia", handler.VerifyKTA)

	// Batch 3: OTP WhatsApp untuk submit awal (publik, limiter sendiri
	// 5/mnt per IP di atas limiter grup — anti spam gateway/bot).
	otpLimiter := middleware.RateLimit(rdb, "otp_wa")
	public.Post("/otp/whatsapp/request", otpLimiter, otpHandler.RequestWhatsAppOTP)
	public.Post("/otp/whatsapp/verify", otpLimiter, otpHandler.VerifyWhatsAppOTP)

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
	adminPendaftaran.Get("/:id/nik", middleware.ScopeWilayah(), handler.RevealNIK)
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
	uploadLimiter := middleware.RateLimit(rdb, "stor_up")

	public := v1.Group("/storage")
	public.Post("/presign-upload", uploadLimiter, handler.PresignUpload)
	public.Get("/presign-view", authMiddleware.Authenticate(), handler.PresignView)
}

// registerNotificationRoutes mendaftarkan notifikasi milik sendiri.
// Tanpa parameter userId (anti-IDOR): identitas dari JWT terverifikasi.
func registerNotificationRoutes(
	v1 fiber.Router,
	authMiddleware *middleware.AuthMiddleware,
	handler *NotificationHandler,
) {
	mine := v1.Group("/notifications", authMiddleware.Authenticate())
	mine.Get("/me", handler.Mine)
	mine.Post("/:id/read", handler.MarkRead)
}

// registerWilayahRoutes mendaftarkan daftar master wilayah (publik,
// read-only, untuk dropdown form + test lintas-wilayah).
func registerWilayahRoutes(
	v1 fiber.Router,
	rdb *redis.Client,
	handler *WilayahHandler,
) {
	wilayahLimiter := middleware.RateLimit(rdb, "wil_pub")

	public := v1.Group("/wilayah")
	public.Use(wilayahLimiter)
	public.Get("/provinsi", handler.ListProvinsi)
	public.Get("/kabupaten/:provinsi_id", handler.ListKabupaten)
	public.Get("/kecamatan", handler.ListKecamatan)
	public.Get("/desa", handler.ListDesa)
	public.Get("/kodepos", handler.ListKodepos)
}

// registerUserAdminRoutes mendaftarkan manajemen akun admin (Super Admin).
func registerUserAdminRoutes(
	v1 fiber.Router,
	rdb *redis.Client,
	authMiddleware *middleware.AuthMiddleware,
	handler *UserAdminHandler,
) {
	grp := v1.Group("/admin/users",
		authMiddleware.Authenticate(),
		middleware.RequireRoles(domain.RoleSuperAdmin),
		middleware.MutatingRateLimit(rdb, "usr_mut"),
	)
	grp.Get("", handler.List)
	grp.Get("/counts", handler.Counts)
	grp.Post("", handler.Create)
	grp.Put("/:id", handler.Update)
	grp.Delete("/:id", handler.Delete)
}

// registerWilayahAdminRoutes mendaftarkan endpoint master wilayah admin
// (Super/Nasional): daftar, detail, pengurus, ubah status, tambah dari master.
func registerWilayahAdminRoutes(
	v1 fiber.Router,
	rdb *redis.Client,
	authMiddleware *middleware.AuthMiddleware,
	handler *WilayahAdminHandler,
) {
	grp := v1.Group("/admin/wilayah",
		authMiddleware.Authenticate(),
		middleware.RequireRoles(domain.RoleSuperAdmin, domain.RoleAdminNasional),
		middleware.MutatingRateLimit(rdb, "wil_mut"),
	)
	grp.Get("/cards", handler.Cards)
	grp.Get("", handler.List)
	grp.Post("", handler.Add)
	grp.Get("/:type/:id/detail", handler.Detail)
	grp.Get("/:type/:id/pengurus", handler.Pengurus)
	grp.Patch("/:type/:id", handler.SetStatus)
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
