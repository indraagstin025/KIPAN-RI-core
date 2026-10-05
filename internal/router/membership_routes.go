package router

// membership_routes.go mendaftarkan endpoint pendaftaran (publik) dan antrean
// verifikasi admin, termasuk OTP WhatsApp.

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/handler"
	"github.com/kipan-indonesia/sim-kipan-core/internal/middleware"
)

// registerMembershipRoutes mendaftarkan endpoint /pendaftaran (publik) dan
// /admin/pendaftaran (verifikasi admin, ter-scope wilayah).
func registerMembershipRoutes(
	v1 fiber.Router,
	rdb *redis.Client,
	authMiddleware *middleware.AuthMiddleware,
	h *handler.PendaftaranHandler,
	otpHandler *handler.OTPHandler,
) {
	// Limiter publik (anti enumerasi/spam) + limiter mutasi sensitif.
	publicLimiter := middleware.RateLimit(rdb, "mem_pub")
	adminMutasiLimiter := middleware.MutatingRateLimit(rdb, "mem_mut")
	// Limiter token revisi per-nomor (3 permintaan / 24 jam per nomor).
	revTokenLimiter := middleware.RevisionTokenLimiter(rdb, 3, 24*time.Hour)
	// Limiter pelacakan per-nomor (nomor REG sekuensial mudah dienumerasi).
	trackLimiter := middleware.TrackLimiter(rdb, 15, time.Minute)

	public := v1.Group("/pendaftaran")
	public.Use(publicLimiter)
	public.Post("", h.Submit)
	public.Get("/track/:nomor", trackLimiter, h.TrackStatus)
	public.Post("/revisi/request-token", revTokenLimiter, h.RequestRevisionToken)
	public.Put("/revisi/:nomor", h.SubmitRevision)
	public.Get("/kta/:nia", h.VerifyKTA)

	// OTP WhatsApp untuk submit awal (publik, limiter tersendiri).
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
	adminPendaftaran.Get("", middleware.ScopeWilayah(), h.ListQueue)
	adminPendaftaran.Get("/:id", middleware.ScopeWilayah(), h.Detail)
	adminPendaftaran.Get("/:id/nik", middleware.ScopeWilayah(), h.RevealNIK)
	adminPendaftaran.Post("/:id/verifikasi", middleware.ScopeWilayah(), h.Verify)
	adminPendaftaran.Post("/:id/perbaikan", middleware.ScopeWilayah(), h.RequestRevision)
	adminPendaftaran.Post("/:id/tolak", middleware.ScopeWilayah(), h.Reject)
	adminPendaftaran.Post("/:id/setujui", middleware.ScopeWilayah(), h.Approve)
	// Bypass pemulihan (matriks §9.3): hanya Super Admin & Admin Nasional,
	// alasan wajib + audit khusus (ditegakkan di service).
	adminPendaftaran.Post("/:id/bypass",
		middleware.RequireRoles(domain.RoleSuperAdmin, domain.RoleAdminNasional),
		middleware.ScopeWilayah(), h.Bypass)
}
