package service

import (
	"context"
	"testing"

	"github.com/kipan-indonesia/sim-kipan-core/config"
	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
)

func TestWriteAuditNilRepoNoop(t *testing.T) {
	// Repo nil tidak boleh panic/gagal — operasi utama tetap jalan.
	writeAudit(context.Background(), nil, domain.AuditContext{},
		nil, "x", "y", "e", "1", "SUBMIT", nil)
}

func TestDegradedSkip(t *testing.T) {
	dev := &config.Config{}
	dev.App.Env = "development"
	if !degradedSkip(dev, "x") {
		t.Fatal("dev harus skip")
	}
	if !degradedSkip(nil, "x") {
		t.Fatal("cfg nil harus skip (bukan production)")
	}
	prod := &config.Config{}
	prod.App.Env = "production"
	if degradedSkip(prod, "x") {
		t.Fatal("production TIDAK boleh skip (fail-closed)")
	}
}

func TestUnavailableIs503(t *testing.T) {
	err := unavailable("pendaftaran")
	appErr, ok := err.(*domain.AppError)
	if !ok || appErr.Code != 503 {
		t.Fatalf("harap 503 AppError, dapat %v", err)
	}
}
