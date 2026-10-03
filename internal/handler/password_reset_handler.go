package handler

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kipan-indonesia/sim-kipan-core/internal/service"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/response"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/validator"
)

// PasswordResetHandler melayani reset kata sandi mandiri via email (Batch 3).
// Thin-handler: parse + validasi DTO, delegasi ke service.
type PasswordResetHandler struct {
	service   service.PasswordResetService
	validator *validator.CustomValidator
}

func NewPasswordResetHandler(service service.PasswordResetService, validator *validator.CustomValidator) *PasswordResetHandler {
	return &PasswordResetHandler{service: service, validator: validator}
}

type forgotPasswordPayload struct {
	Email string `json:"email" validate:"required,email"`
}

type resetPasswordPayload struct {
	Token       string `json:"token" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,password_strength"`
}

// Forgot mengirim tautan reset ke email terdaftar. Selalu sukses dengan
// pesan sama untuk email apa pun (anti-enumeration).
func (h *PasswordResetHandler) Forgot(c *fiber.Ctx) error {
	var req forgotPasswordPayload
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Format data tidak valid")
	}
	if errs := h.validator.ValidateStruct(req); len(errs) > 0 {
		return response.ValidationError(c, "Validasi gagal", errs)
	}
	if err := h.service.ForgotPassword(c.Context(), req.Email, auditContextOf(c)); err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c,
		"Jika email terdaftar dan aktif, tautan reset kata sandi telah dikirim. Periksa inbox/spam Anda.", nil)
}

// Reset menukar token sekali pakai dengan kata sandi baru.
func (h *PasswordResetHandler) Reset(c *fiber.Ctx) error {
	var req resetPasswordPayload
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Format data tidak valid")
	}
	if errs := h.validator.ValidateStruct(req); len(errs) > 0 {
		return response.ValidationError(c, "Validasi gagal", errs)
	}
	if err := h.service.ResetPassword(c.Context(), req.Token, req.NewPassword, auditContextOf(c)); err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Kata sandi berhasil diatur ulang. Silakan login dengan kata sandi baru.", nil)
}

// Set menukar token "buat kata sandi" (dari email pengangkatan akun) dengan
// kata sandi baru. Payload sama dengan reset.
func (h *PasswordResetHandler) Set(c *fiber.Ctx) error {
	var req resetPasswordPayload
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Format data tidak valid")
	}
	if errs := h.validator.ValidateStruct(req); len(errs) > 0 {
		return response.ValidationError(c, "Validasi gagal", errs)
	}
	if err := h.service.SetPassword(c.Context(), req.Token, req.NewPassword, auditContextOf(c)); err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Kata sandi berhasil dibuat. Silakan login.", nil)
}
