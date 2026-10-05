package handler

import (
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/middleware"
	"github.com/kipan-indonesia/sim-kipan-core/internal/service/auth"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/response"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/validator"
)

// auditContextOf membangun konteks forensik transport untuk audit trail
// (RULES 21): IP client (menghormati TrustedProxies), user agent, dan
// request ID. Murni baca request — bukan keputusan bisnis.
func auditContextOf(c *fiber.Ctx) domain.AuditContext {
	return domain.AuditContext{
		IP:        c.IP(),
		UserAgent: c.Get("User-Agent"),
		RequestID: requestIDOf(c),
	}
}

func requestIDOf(c *fiber.Ctx) string {
	if id, ok := c.Locals("requestid").(string); ok && id != "" {
		return id
	}
	return c.GetRespHeader("X-Request-ID")
}

type AuthHandler struct {
	authService  auth.AuthService
	validator    *validator.CustomValidator
	refreshTTL   time.Duration
	secureCookie bool
	// L-6: atribut cookie configurable (default aman dari config).
	cookieSameSite string
	cookiePath     string
	cookieDomain   string
}

func NewAuthHandler(
	authService auth.AuthService,
	validator *validator.CustomValidator,
	refreshTTL time.Duration,
	secureCookie bool,
	cookieSameSite, cookiePath, cookieDomain string,
) *AuthHandler {
	if refreshTTL <= 0 {
		refreshTTL = 7 * 24 * time.Hour
	}
	if cookieSameSite == "" {
		cookieSameSite = "Strict"
	}
	if cookiePath == "" {
		cookiePath = "/api/v1/auth"
	}
	return &AuthHandler{
		authService:    authService,
		validator:      validator,
		refreshTTL:     refreshTTL,
		secureCookie:   secureCookie,
		cookieSameSite: cookieSameSite,
		cookiePath:     cookiePath,
		cookieDomain:   cookieDomain,
	}
}

// Login mengotentikasi kredensial dan menerbitkan access token.
// Refresh token HANYA dikirim via HttpOnly cookie — TIDAK di JSON body.
func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req auth.LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Format data JSON tidak valid")
	}

	if errs := h.validator.ValidateStruct(req); len(errs) > 0 {
		return response.ValidationError(c, "Validasi gagal", errs)
	}

	refreshToken, resp, err := h.authService.Login(c.Context(), req, auditContextOf(c))
	if err != nil {
		return response.FromError(c, err)
	}

	h.setRefreshTokenCookie(c, refreshToken, h.refreshTTL)
	return response.Success(c, "Login berhasil", resp)
}

// RefreshToken melakukan rotasi refresh token (RTR).
// Token baru menggantikan token lama di cookie HttpOnly.
func (h *AuthHandler) RefreshToken(c *fiber.Ctx) error {
	// Fix C-11: HANYA baca dari cookie. Fallback ke body dihapus karena
	// melemahkan proteksi HttpOnly (XSS bisa baca dari body).
	tokenStr := c.Cookies("refresh_token")
	if tokenStr == "" {
		return response.Unauthorized(c, "Refresh token tidak ditemukan. Silakan login kembali")
	}

	newRefreshToken, resp, err := h.authService.RefreshToken(c.Context(), tokenStr, auditContextOf(c))
	if err != nil {
		h.clearRefreshTokenCookie(c)
		return response.FromError(c, err)
	}

	h.setRefreshTokenCookie(c, newRefreshToken, h.refreshTTL)
	return response.Success(c, "Token berhasil diperbarui", resp)
}

// Logout mencabut sesi dan membersihkan refresh cookie.
//
// SENGAJA tanpa middleware Authenticate(): route didaftarkan publik agar
// pencabutan via refresh cookie tetap jalan saat access token kedaluwarsa.
// Selalu kembalikan sukses (idempoten) agar tidak menjadi oracle sesi.
func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	tokenStr := c.Cookies("refresh_token")

	// L-3: pakai helper Bearer tunggal dari middleware (best-effort;
	// header boleh kosong di jalur logout publik).
	rawAccess, _ := middleware.ExtractBearerToken(c.Get("Authorization"))

	_ = h.authService.Logout(c.Context(), rawAccess, tokenStr, auditContextOf(c))
	h.clearRefreshTokenCookie(c)

	return response.Success(c, "Logout berhasil", nil)
}

// Me mengembalikan profil pengguna terotentikasi.
func (h *AuthHandler) Me(c *fiber.Ctx) error {
	claims := middleware.GetUser(c)
	if claims == nil {
		return response.Unauthorized(c, "Tidak terotentikasi")
	}

	profile, err := h.authService.GetProfile(c.Context(), claims.UserID)
	if err != nil {
		return response.FromError(c, err)
	}

	return response.Success(c, "Data pengguna", profile)
}

// ChangePassword memverifikasi kata sandi saat ini dan menyimpan hash baru.
func (h *AuthHandler) ChangePassword(c *fiber.Ctx) error {
	claims := middleware.GetUser(c)
	if claims == nil {
		return response.Unauthorized(c, "Tidak terotentikasi")
	}

	var req auth.ChangePasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Format data tidak valid")
	}

	if errs := h.validator.ValidateStruct(req); len(errs) > 0 {
		return response.ValidationError(c, "Validasi gagal", errs)
	}

	// L-4: teruskan access token pemanggil agar service mem-blacklist-nya
	// (best-effort; header selalu ada karena route ini butuh Authenticate).
	rawAccess, _ := middleware.ExtractBearerToken(c.Get("Authorization"))

	if err := h.authService.ChangePassword(c.Context(), claims.UserID, req, rawAccess, auditContextOf(c)); err != nil {
		return response.FromError(c, err)
	}

	// Password berubah → seluruh sesi lain dicabut oleh service.
	// Hapus juga cookie di browser ini agar user login ulang segar.
	h.clearRefreshTokenCookie(c)

	return response.Success(c,
		"Kata sandi berhasil diperbarui. Seluruh sesi lain telah dihentikan, silakan login kembali.", nil)
}

// ============================================================
// Cookie Helpers
// ============================================================

// setRefreshTokenCookie menyimpan refresh token di HttpOnly cookie.
// Path dibatasi ke endpoint /auth agar cookie tidak dikirim ke request lain.
//
// KEAMANAN (flag Secure — RULES #8):
// Nilai flag Secure diambil dari konfigurasi server (APP_ENV != development),
// BUKAN dari header request (X-Forwarded-Proto) yang bisa dipalsukan klien.
// Di belakang reverse proxy (Caddy/Nginx), c.Protocol() selalu "http" karena
// TLS di-terminate di proxy — sehingga keputusan berbasis header tidak bisa
// dipercaya dan cookie berisiko terbit tanpa Secure (kirim via HTTP polos).
func (h *AuthHandler) setRefreshTokenCookie(c *fiber.Ctx, token string, ttl time.Duration) {
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}

	c.Cookie(&fiber.Cookie{
		Name:     "refresh_token",
		Value:    token,
		Expires:  time.Now().Add(ttl),
		MaxAge:   int(ttl.Seconds()),
		HTTPOnly: true,
		Secure:   h.secureCookie,
		SameSite: h.cookieSameSite,
		Path:     h.cookiePath,
		Domain:   h.cookieDomain,
	})
}

func (h *AuthHandler) clearRefreshTokenCookie(c *fiber.Ctx) {
	c.Cookie(&fiber.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Expires:  time.Now().Add(-1 * time.Hour),
		MaxAge:   -1,
		HTTPOnly: true,
		Secure:   h.secureCookie,
		SameSite: h.cookieSameSite,
		Path:     h.cookiePath,
		Domain:   h.cookieDomain,
	})
}