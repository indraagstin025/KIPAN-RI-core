package service

import (
	"context"
	"testing"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/service/svcutil"
	"github.com/kipan-indonesia/sim-kipan-core/internal/service/testutil"
)

func TestNormalizeNomor(t *testing.T) {
	valid := []string{
		"REG-202609-0001",    // warisan 4 digit
		"REG-202609-00001",   // baru 5 digit
		"reg-202609-00001",   // normalisasi case
		"  REG-202609-0001 ", // normalisasi spasi
		"REG-202609-000001",  // >5 digit (periode padat, T10)
	}
	for _, nomor := range valid {
		if _, err := svcutil.NormalizeNomor(nomor); err != nil {
			t.Fatalf("expected %q to be accepted: %v", nomor, err)
		}
	}
	invalid := []string{
		"",
		"asal",
		"REG-202609-001", // 3 digit
		"REG-2026-00001", // periode pendek
		"REG-202609-00001-EXTRA-MUATAN-PANJANG-LEBIH-DARI-30",
	}
	for _, nomor := range invalid {
		if _, err := svcutil.NormalizeNomor(nomor); err == nil {
			t.Fatalf("expected %q to be rejected", nomor)
		}
	}
}

func TestAnggotaListForbiddenRole(t *testing.T) {
	svc := NewAnggotaService(nil, AnggotaDeps{AnggotaRepo: &testutil.FakeAnggotaRepo{}})
	actor := domain.ActorContext{UserID: "u1", Role: "PENDAFTAR"}
	if _, _, err := svc.ListAnggota(context.Background(), actor, "", "", 1, 25); err == nil {
		t.Fatal("expected unknown role to be rejected")
	}
}

func TestAnggotaListInvalidStatus(t *testing.T) {
	svc := NewAnggotaService(nil, AnggotaDeps{AnggotaRepo: &testutil.FakeAnggotaRepo{}})
	actor := domain.ActorContext{UserID: "u1", Role: domain.RoleSuperAdmin}
	if _, _, err := svc.ListAnggota(context.Background(), actor, "BOGUS", "", 1, 25); err == nil {
		t.Fatal("expected invalid status filter to be rejected")
	}
}

func TestGetPublicAnggota(t *testing.T) {
	repo := &testutil.FakeAnggotaRepo{ByNIA: map[string]*domain.Anggota{
		"KIPAN-IND-3273-2026-000001": {
			ID: 7, NIA: "KIPAN-IND-3273-2026-000001", NamaLengkap: "Rizki Pratama",
			ProvinsiID: 2, KabupatenID: 3, Status: domain.AnggotaStatusAktif,
		},
	}}
	svc := NewAnggotaService(nil, AnggotaDeps{AnggotaRepo: repo})

	if _, err := svc.GetPublicAnggota(context.Background(), "asal"); err == nil {
		t.Fatal("expected invalid NIA to be rejected")
	}
	if _, err := svc.GetPublicAnggota(context.Background(), "KIPAN-IND-3273-2026-099999"); err == nil {
		t.Fatal("expected unknown NIA to return not found")
	}
	info, err := svc.GetPublicAnggota(context.Background(), "kipan-ind-3273-2026-000001")
	if err != nil {
		t.Fatalf("expected known NIA to pass: %v", err)
	}
	if info.NamaLengkap != "Rizki Pratama" || info.Status != "AKTIF" {
		t.Fatalf("unexpected public info: %+v", info)
	}
}
