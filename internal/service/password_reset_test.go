package service

// Uji Batch 3: password reset — fail-closed saat dependensi tidak lengkap.

import (
	"context"
	"testing"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
)

func TestPasswordResetFailClosedTanpaDependensi(t *testing.T) {
	svc := NewPasswordResetService(nil, PasswordResetDeps{})

	if err := svc.ForgotPassword(context.Background(), "a@b.co", domain.AuditContext{}); err == nil {
		t.Fatal("tanpa Redis/email, ForgotPassword harus gagal fail-closed")
	}
	if err := svc.ResetPassword(context.Background(), "token", "PasswordBaru123!", domain.AuditContext{}); err == nil {
		t.Fatal("tanpa Redis, ResetPassword harus gagal fail-closed")
	}
}

func TestResetTokenKosongDitolak(t *testing.T) {
	svc := NewPasswordResetService(nil, PasswordResetDeps{})
	err := svc.ResetPassword(context.Background(), "   ", "PasswordBaru123!", domain.AuditContext{})
	if err == nil {
		t.Fatal("token kosong harus ditolak")
	}
}
