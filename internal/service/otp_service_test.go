package service

// Uji Batch 3: OTP WhatsApp (tanpa Redis — bagian murni + fail-closed).

import (
	"context"
	"testing"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
)

func TestGenerateOTPCodeEnamDigit(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		code, err := generateOTPCode(6)
		if err != nil {
			t.Fatalf("generate gagal: %v", err)
		}
		if len(code) != 6 || code[0] == '0' {
			t.Fatalf("format salah: %q", code)
		}
		for _, ch := range code {
			if ch < '0' || ch > '9' {
				t.Fatalf("non-digit: %q", code)
			}
		}
		seen[code] = true
	}
	if len(seen) < 40 {
		t.Fatalf("keacakan buruk: %d unik dari 50", len(seen))
	}
}

func TestNormalizeOTPTarget(t *testing.T) {
	a, err := normalizeOTPTarget("081234567890")
	if err != nil {
		t.Fatalf("08 valid ditolak: %v", err)
	}
	for _, variant := range []string{"6281234567890", "+6281234567890", " 081234567890 "} {
		b, err := normalizeOTPTarget(variant)
		if err != nil {
			t.Fatalf("%q ditolak: %v", variant, err)
		}
		if b != a {
			t.Fatalf("%q menormalisasi ke %q, harap %q", variant, b, a)
		}
	}
	for _, bad := range []string{"", "abc123", "080000000000", "08123456789012345"} {
		if _, err := normalizeOTPTarget(bad); err == nil {
			t.Fatalf("%q harus ditolak", bad)
		}
	}
}

func TestOTPReadyFailClosed(t *testing.T) {
	svc := NewOTPService(nil, OtpDeps{})
	if _, err := svc.RequestOTP(context.Background(), "081234567890"); err == nil {
		t.Fatal("tanpa Redis/gateway RequestOTP harus gagal (fail-closed)")
	}
	if _, err := svc.VerifyOTP(context.Background(), "081234567890", "123456"); err == nil {
		t.Fatal("tanpa Redis/gateway VerifyOTP harus gagal (fail-closed)")
	}
	if err := svc.VerifyAndConsume(context.Background(), "081234567890", "tok"); err == nil {
		t.Fatal("tanpa Redis/gateway VerifyAndConsume harus gagal (fail-closed)")
	}
}

func TestTooManyOTP429(t *testing.T) {
	err := tooManyOTP(42)
	appErr, ok := err.(*domain.AppError)
	if !ok || appErr.Code != 429 {
		t.Fatalf("harap 429, dapat %v", err)
	}
}
