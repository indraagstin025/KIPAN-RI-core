package handler

// Uji transport OTP WhatsApp (Batch 3): parsing + pemetaan error.

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/service"
)

type stubOTPService struct {
	reqRes   *service.OTPRequestResult
	reqErr   error
	verRes   *service.OTPVerifyResult
	verErr   error
}

func (s *stubOTPService) RequestOTP(context.Context, string) (*service.OTPRequestResult, error) {
	return s.reqRes, s.reqErr
}

func (s *stubOTPService) VerifyOTP(context.Context, string, string) (*service.OTPVerifyResult, error) {
	return s.verRes, s.verErr
}

func (s *stubOTPService) VerifyAndConsume(context.Context, string, string) error {
	return nil
}

var _ service.OTPService = (*stubOTPService)(nil)

func testOTPApp(stub *stubOTPService) *fiber.App {
	app := fiber.New()
	h := NewOTPHandler(stub)
	app.Post("/pendaftaran/otp/whatsapp/request", h.RequestWhatsAppOTP)
	app.Post("/pendaftaran/otp/whatsapp/verify", h.VerifyWhatsAppOTP)
	return app
}

func TestOTPRequestButuhNomor(t *testing.T) {
	app := testOTPApp(&stubOTPService{reqRes: &service.OTPRequestResult{}})
	req := httptest.NewRequest("POST", "/pendaftaran/otp/whatsapp/request", stringReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request gagal: %v", err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("harap 400 tanpa nomor, dapat %d", resp.StatusCode)
	}
}

func TestOTPVerifyTeruskanService(t *testing.T) {
	stub := &stubOTPService{
		verRes: &service.OTPVerifyResult{VerifiedToken: "tok", ExpiresIn: 900},
	}
	app := testOTPApp(stub)
	req := httptest.NewRequest("POST", "/pendaftaran/otp/whatsapp/verify",
		stringReader(`{"whatsapp":"081234567890","code":"123456"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request gagal: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("harap 200, dapat %d", resp.StatusCode)
	}
}

func TestOTPVerifySalah429(t *testing.T) {
	stub := &stubOTPService{verErr: domain.NewValidationError("Kode salah atau kedaluwarsa")}
	app := testOTPApp(stub)
	req := httptest.NewRequest("POST", "/pendaftaran/otp/whatsapp/verify",
		stringReader(`{"whatsapp":"081234567890","code":"000000"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request gagal: %v", err)
	}
	if resp.StatusCode != fiber.StatusUnprocessableEntity {
		t.Fatalf("harap 422, dapat %d", resp.StatusCode)
	}
}
