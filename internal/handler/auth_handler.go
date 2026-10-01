package handler

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/kipan-indonesia/sim-kipan-core/internal/middleware"
	"github.com/kipan-indonesia/sim-kipan-core/internal/service"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/response"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/validator"
)

type AuthHandler struct {
	authService service.AuthService
	validator   *validator.CustomValidator
	refreshTTL  time.Duration
}

func NewAuthHandler(
	authService service.AuthService,
	validator *validator.CustomValidator,
	refreshTTL time.Duration,
) *AuthHandler {
	if refreshTTL <= 0 {
		refreshTTL = 7 * 24 * time.Hour
	}
	return &AuthHandler{
		authService: authService,
		validator:   validator,
		refreshTTL:  refreshTTL,
	}
}

// Login mengotentikasi kredensial dan menerbitkan access token.
// Refresh token HANYA dikirim via HttpOnly cookie — TIDAK di JSON body.
func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req service.LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Format data JSON tidak valid")
	}

	if errs := h.validator.ValidateStruct(req); len(errs) > 0 {
		return response.ValidationError(c, "Validasi gagal", errs)
	}

	refreshToken, resp, err := h.authService.Login(c.Context(), req)
	if err != nil {
		return response.FromError(c, err)
	}

	setRefreshTokenCookie(c, refreshToken, h.refreshTTL)
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

	newRefreshToken, resp, err := h.authService.RefreshToken(c.Context(), tokenStr)
	if err != nil {
		clearRefreshTokenCookie(c)
		return response.FromError(c, err)
	}

	setRefreshTokenCookie(c, newRefreshToken, h.refreshTTL)
	return response.Success(c, "Token berhasil diperbarui", resp)
}

// Logout mencabut sesi dan membersihkan refresh cookie.
func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	tokenStr := c.Cookies("refresh_token")

	authHeader := c.Get("Authorization")
	parts := strings.SplitN(authHeader, " ", 2)
	rawAccess := ""
	if len(parts) == 2 {
		rawAccess = strings.TrimSpace(parts[1])
	}

	_ = h.authService.Logout(c.Context(), rawAccess, tokenStr)
	clearRefreshTokenCookie(c)

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

	var req service.ChangePasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Format data tidak valid")
	}

	if errs := h.validator.ValidateStruct(req); len(errs) > 0 {
		return response.ValidationError(c, "Validasi gagal", errs)
	}

	if err := h.authService.ChangePassword(c.Context(), claims.UserID, req); err != nil {
		return response.FromError(c, err)
	}

	// Password berubah → seluruh sesi lain dicabut oleh service.
	// Hapus juga cookie di browser ini agar user login ulang segar.
	clearRefreshTokenCookie(c)

	return response.Success(c,
		"Kata sandi berhasil diperbarui. Seluruh sesi lain telah dihentikan, silakan login kembali.", nil)
}

// ============================================================
// Cookie Helpers
// ============================================================

// setRefreshTokenCookie menyimpan refresh token di HttpOnly cookie.
// Path dibatasi ke endpoint /auth agar cookie tidak dikirim ke request lain.
func setRefreshTokenCookie(c *fiber.Ctx, token string, ttl time.Duration) {
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}

	// Deteksi HTTPS di belakang reverse proxy (Caddy/Nginx).
	// X-Forwarded-Proto di-set oleh proxy yang terminate TLS.
	isHTTPS := c.Protocol() == "https" ||
		strings.EqualFold(c.Get("X-Forwarded-Proto"), "https")

	c.Cookie(&fiber.Cookie{
		Name:     "refresh_token",
		Value:    token,
		Expires:  time.Now().Add(ttl),
		MaxAge:   int(ttl.Seconds()),
		HTTPOnly: true,
		Secure:   isHTTPS,
		SameSite: "Strict",
		Path:     "/api/v1/auth",
	})
}

func clearRefreshTokenCookie(c *fiber.Ctx) {
	c.Cookie(&fiber.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Expires:  time.Now().Add(-1 * time.Hour),
		MaxAge:   -1,
		HTTPOnly: true,
		Secure:   c.Protocol() == "https",
		SameSite: "Strict",
		Path:     "/api/v1/auth",
	})
}