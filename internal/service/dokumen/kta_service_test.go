package dokumen

import (
	"context"
	"testing"

	"github.com/kipan-indonesia/sim-kipan-core/config"
	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/service/testutil"
)

// stubDocStore menerbitkan URL baca palsu untuk uji KTA mandiri.
type stubDocStore struct{}

func (stubDocStore) Configured() bool { return true }
func (stubDocStore) PutKTADocument(_ context.Context, _ string, _ []byte) (string, error) {
	return "kta/x.pdf", nil
}
func (stubDocStore) PresignKTADocument(_ context.Context, _ string) (string, error) {
	return "https://s3.example/kta.pdf", nil
}

func TestGetMyKTA(t *testing.T) {
	uid := "user-kader-1"
	key := "kta/KIPAN-1.pdf"
	member := &domain.Anggota{ID: 31, NIA: "KIPAN-IND-3273-2026-000031", NamaLengkap: "Kader", UserID: &uid, KTAPDFKey: &key}
	cfg := &config.Config{}
	svc := NewKTAService(cfg, KTADeps{
		AnggotaRepo: &testutil.FakeAnggotaRepo{ByNIA: map[string]*domain.Anggota{member.NIA: member}},
		DocStore:    stubDocStore{},
	})

	url, err := svc.GetMyKTADocumentURL(context.Background(), uid, domain.AuditContext{})
	if err != nil {
		t.Fatalf("KTA sendiri gagal: %v", err)
	}
	if url == "" {
		t.Fatal("harap URL presign dikembalikan")
	}

	if _, err := svc.GetMyKTADocumentURL(context.Background(), "user-tanpa-anggota", domain.AuditContext{}); err == nil {
		t.Fatal("user tanpa anggota harus 404")
	}
}
