package kta

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

// KTA anggota MENINGGAL/NONAKTIF/DIBERHENTIKAN tidak berlaku (matriks Tabel
// 23 + keputusan produk): unduhan admin maupun mandiri ditolak.
func TestKTATolakStatusTakBerlaku(t *testing.T) {
	for _, st := range []domain.AnggotaStatus{
		domain.AnggotaStatusMeninggal,
		domain.AnggotaStatusNonaktif,
		domain.AnggotaStatusDiberhentikan,
	} {
		uid := "user-1"
		key := "kta/x.pdf"
		member := &domain.Anggota{ID: 31, NIA: "KIPAN-IND-3273-2026-000031", NamaLengkap: "Kader",
			Status: st, ProvinsiID: 32, KabupatenID: 3273, UserID: &uid, KTAPDFKey: &key}
		svc := NewKTAService(nil, KTADeps{
			AnggotaRepo: &testutil.FakeAnggotaRepo{
				ByID:  map[int]*domain.Anggota{31: member},
				ByNIA: map[string]*domain.Anggota{member.NIA: member},
			},
			DocStore: stubDocStore{},
		})
		if _, err := svc.GetKTADocumentURL(context.Background(), 31, testutil.SuperActor(), domain.AuditContext{}); err == nil {
			t.Fatalf("status %s: unduhan admin harus ditolak", st)
		}
		if _, err := svc.GetMyKTADocumentURL(context.Background(), uid, domain.AuditContext{}); err == nil {
			t.Fatalf("status %s: unduhan mandiri harus ditolak", st)
		}
	}
}
