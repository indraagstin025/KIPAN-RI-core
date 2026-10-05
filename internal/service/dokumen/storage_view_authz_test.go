package dokumen

// Regression test keamanan (SEC-STORE-BOLA): RequestViewPresign WAJIB
// mengotorisasi akses baca dokumen berdasarkan yurisdiksi aktor, bukan
// sekadar menerima pemanggil yang sudah terautentikasi.

import (
	"context"
	"errors"
	"testing"

	"github.com/kipan-indonesia/sim-kipan-core/config"
	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
	storagepkg "github.com/kipan-indonesia/sim-kipan-core/pkg/storage"
)

type fakeDocResolver struct {
	owner *repository.DocumentOwner
	err   error
}

func (f fakeDocResolver) ResolveOwner(context.Context, string) (*repository.DocumentOwner, error) {
	return f.owner, f.err
}

func intPtrAuthz(v int) *int { return &v }

func appErrCode(err error) int {
	var ae *domain.AppError
	if errors.As(err, &ae) {
		return ae.Code
	}
	return 0
}

// newConfiguredStorageService membangun StorageService dengan S3 client
// verifikasi (kredensial dummy). Presign hanyalah penandatanganan lokal —
// tidak menyentuh jaringan — sehingga cukup untuk menguji otorisasi.
func newConfiguredStorageService(t *testing.T, resolver DocumentOwnerResolver) *StorageService {
	t.Helper()
	client, err := storagepkg.NewClient("http://127.0.0.1:9000", "auto", "k", "s")
	if err != nil {
		t.Fatalf("gagal membuat client uji: %v", err)
	}
	return NewStorageService(&config.Config{}, client, nil, resolver)
}

func TestRequestViewPresignEnforcesJurisdiction(t *testing.T) {
	ctx := context.Background()
	key := "uploads/pendaftaran/202610/aaaaaaaa-0000-0000-0000-000000000001.jpg"
	audit := domain.AuditContext{IP: "127.0.0.1"}
	kabBandung := domain.ActorContext{
		UserID: "u1", Role: domain.RoleAdminKabupaten,
		ProvinsiID: intPtrAuthz(2), KabupatenID: intPtrAuthz(3),
	}

	// (1) pemilik di luar yurisdiksi -> 403
	svc := newConfiguredStorageService(t, fakeDocResolver{
		owner: &repository.DocumentOwner{ProvinsiID: 1, KabupatenID: 1, Entity: "pendaftaran", EntityID: "9"},
	})
	if _, err := svc.RequestViewPresign(ctx, key, kabBandung, audit); appErrCode(err) != 403 {
		t.Fatalf("harap 403 lintas-yurisdiksi, dapat %v", err)
	}

	// (2) key tanpa pemilik -> 404
	svc = newConfiguredStorageService(t, fakeDocResolver{err: domain.ErrNotFound})
	if _, err := svc.RequestViewPresign(ctx, key, kabBandung, audit); appErrCode(err) != 404 {
		t.Fatalf("harap 404 key tanpa pemilik, dapat %v", err)
	}

	// (3) pemilik dalam yurisdiksi -> tiket terbit
	svc = newConfiguredStorageService(t, fakeDocResolver{
		owner: &repository.DocumentOwner{ProvinsiID: 2, KabupatenID: 3, Entity: "pendaftaran", EntityID: "9"},
	})
	res, err := svc.RequestViewPresign(ctx, key, kabBandung, audit)
	if err != nil || res == nil || res.ViewURL == "" {
		t.Fatalf("harap tiket baca untuk dokumen sewilayah, dapat res=%v err=%v", res, err)
	}

	// (4) super admin (scope nasional) tetap boleh
	svc = newConfiguredStorageService(t, fakeDocResolver{
		owner: &repository.DocumentOwner{ProvinsiID: 1, KabupatenID: 1},
	})
	national := domain.ActorContext{UserID: "sup", Role: domain.RoleSuperAdmin}
	if _, err := svc.RequestViewPresign(ctx, key, national, audit); err != nil {
		t.Fatalf("harap super admin diizinkan, dapat %v", err)
	}

	// (5) resolver nil -> fail-closed (tidak menerbitkan tiket)
	svc = newConfiguredStorageService(t, nil)
	if _, err := svc.RequestViewPresign(ctx, key, kabBandung, audit); err == nil {
		t.Fatal("harap tolak (fail-closed) tanpa resolver")
	}
}
