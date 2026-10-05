package handler

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/kipan-indonesia/sim-kipan-core/internal/service/notify"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/response"
)

// OTPHandler melayani verifikasi kepemilikan WhatsApp pendaftar (Batch 3).
// Thin-handler: parse DTO lalu delegasi ke OTPService. Publik (pendaftar
// belum punya akun) dengan rate-limit ketat di route.
type OTPHandler struct {
	service notify.OTPService
}

func NewOTPHandler(service notify.OTPService) *OTPHandler {
	return &OTPHandler{service: service}
}

type otpRequestPayload struct {
	Whatsapp string `json:"whatsapp"`
}

type otpVerifyPayload struct {
	Whatsapp string `json:"whatsapp"`
	Code     string `json:"code"`
}

// RequestWhatsAppOTP menerbitkan kode 6-digit ke nomor pendaftar.
// Selalu 200 seragam bila format valid (tanpa sinyal keterdaftaran nomor).
func (h *OTPHandler) RequestWhatsAppOTP(c *fiber.Ctx) error {
	var req otpRequestPayload
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Format data tidak valid")
	}
	if strings.TrimSpace(req.Whatsapp) == "" {
		return response.BadRequest(c, "Nomor WhatsApp wajib diisi")
	}
	res, err := h.service.RequestOTP(c.Context(), req.Whatsapp)
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Kode verifikasi dikirim ke WhatsApp", res)
}

// VerifyWhatsAppOTP menukar kode benar menjadi token terverifikasi yang
// wajib disertakan sebagai wa_otp_token saat POST /pendaftaran.
func (h *OTPHandler) VerifyWhatsAppOTP(c *fiber.Ctx) error {
	var req otpVerifyPayload
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Format data tidak valid")
	}
	if strings.TrimSpace(req.Whatsapp) == "" || strings.TrimSpace(req.Code) == "" {
		return response.BadRequest(c, "Nomor WhatsApp dan kode wajib diisi")
	}
	res, err := h.service.VerifyOTP(c.Context(), req.Whatsapp, req.Code)
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Nomor WhatsApp terverifikasi", res)
}
