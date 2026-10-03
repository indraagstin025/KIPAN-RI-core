package service

import (
	"context"
	"errors"
	"testing"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
)

type fakeWilayahRepo struct {
	prov, kab string
	err       error
}

func (f *fakeWilayahRepo) ExistsProvinsi(context.Context, int) (bool, error) {
	return true, nil
}
func (f *fakeWilayahRepo) KabupatenInProvinsi(context.Context, int, int) (bool, error) {
	return true, nil
}
func (f *fakeWilayahRepo) ListProvinsi(context.Context) ([]domain.WilayahProvinsi, error) {
	return nil, errFakeUnimpl
}
func (f *fakeWilayahRepo) ListKabupaten(context.Context, int) ([]domain.WilayahKabupaten, error) {
	return nil, errFakeUnimpl
}
func (f *fakeWilayahRepo) GetNames(context.Context, int, int) (string, string, error) {
	return f.prov, f.kab, f.err
}

var _ repository.WilayahRepository = (*fakeWilayahRepo)(nil)

func detailSvc(item *domain.Pendaftaran, w repository.WilayahRepository) PendaftaranService {
	return NewPendaftaranService(nil, PendaftaranDeps{
		Repo:        &fakePendaftaranRepo{item: item},
		WilayahRepo: w,
	})
}

func TestGetDetailMemuatNamaWilayah(t *testing.T) {
	prov, kab := 2, 3
	svc := detailSvc(&domain.Pendaftaran{
		ID: 7, ProvinsiID: prov, KabupatenID: kab,
		Status: domain.PendaftaranStatusDraft,
	}, &fakeWilayahRepo{prov: "Jawa Barat", kab: "Kota Bandung"})
	actor := domain.ActorContext{UserID: "u1", Role: domain.RoleSuperAdmin}
	got, err := svc.GetDetail(context.Background(), 7, actor)
	if err != nil {
		t.Fatalf("GetDetail gagal: %v", err)
	}
	if got.ProvinsiNama != "Jawa Barat" || got.KabupatenNama != "Kota Bandung" {
		t.Fatalf("nama wilayah salah: %+v", got)
	}
	if got.ProvinsiID != prov || got.KabupatenID != kab {
		t.Fatalf("id wilayah berubah: %+v", got)
	}
}

func TestGetDetailTetapSuksesBilaNamaGagal(t *testing.T) {
	prov, kab := 2, 3
	svc := detailSvc(&domain.Pendaftaran{
		ID: 7, ProvinsiID: prov, KabupatenID: kab,
		Status: domain.PendaftaranStatusDraft,
	}, &fakeWilayahRepo{err: errors.New("db down")})
	actor := domain.ActorContext{UserID: "u1", Role: domain.RoleSuperAdmin}
	got, err := svc.GetDetail(context.Background(), 7, actor)
	if err != nil {
		t.Fatalf("detail tidak boleh gagal karena nama: %v", err)
	}
	if got.ProvinsiNama != "" || got.KabupatenNama != "" {
		t.Fatalf("nama harus kosong saat lookup gagal: %+v", got)
	}
}

func TestGetDetailLintasWilayahTetapDitolak(t *testing.T) {
	prov, kab := 2, 3
	otherKab := 99
	svc := detailSvc(&domain.Pendaftaran{
		ID: 7, ProvinsiID: prov, KabupatenID: kab,
		Status: domain.PendaftaranStatusDraft,
	}, &fakeWilayahRepo{})
	actor := domain.ActorContext{UserID: "u1", Role: domain.RoleAdminKabupaten, ProvinsiID: &prov, KabupatenID: &otherKab}
	if _, err := svc.GetDetail(context.Background(), 7, actor); err == nil {
		t.Fatal("detail lintas wilayah DITERIMA")
	}
}
