package svcutil

import (
	"context"
	"testing"

	"github.com/kipan-indonesia/sim-kipan-core/config"
	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
)

func TestWriteAuditNilRepoNoop(t *testing.T) {
	// Repo nil tidak boleh panic/gagal — operasi utama tetap jalan.
	WriteAudit(context.Background(), nil, domain.AuditContext{},
		nil, "x", "y", "e", "1", "SUBMIT", nil)
}

func TestDegradedSkip(t *testing.T) {
	dev := &config.Config{}
	dev.App.Env = "development"
	if !DegradedSkip(dev, "x") {
		t.Fatal("dev harus skip")
	}
	if !DegradedSkip(nil, "x") {
		t.Fatal("cfg nil harus skip (bukan production)")
	}
	prod := &config.Config{}
	prod.App.Env = "production"
	if DegradedSkip(prod, "x") {
		t.Fatal("production TIDAK boleh skip (fail-closed)")
	}
}

func TestUnavailableIs503(t *testing.T) {
	err := Unavailable("pendaftaran")
	appErr, ok := err.(*domain.AppError)
	if !ok || appErr.Code != 503 {
		t.Fatalf("harap 503 AppError, dapat %v", err)
	}
}

func TestPublicURLFrom(t *testing.T) {
	if got := PublicURLFrom(nil); got != "http://localhost:5173" {
		t.Fatalf("fallback salah: %s", got)
	}
}
