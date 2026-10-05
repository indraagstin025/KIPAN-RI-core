package service

import (
	"context"
	"errors"
	"testing"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
	"github.com/kipan-indonesia/sim-kipan-core/internal/service/testutil"
)

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
	}, &testutil.FakeWilayahRepo{Prov: "Jawa Barat", Kab: "Kota Bandung"})
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
	}, &testutil.FakeWilayahRepo{Err: errors.New("db down")})
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
	}, &testutil.FakeWilayahRepo{})
	actor := domain.ActorContext{UserID: "u1", Role: domain.RoleAdminKabupaten, ProvinsiID: &prov, KabupatenID: &otherKab}
	if _, err := svc.GetDetail(context.Background(), 7, actor); err == nil {
		t.Fatal("detail lintas wilayah DITERIMA")
	}
}
