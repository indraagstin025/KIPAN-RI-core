package anggota

// Uji B3: tambah/sunting/status anggota langsung oleh admin.

import (
	"context"
	"strings"
	"testing"

	"github.com/kipan-indonesia/sim-kipan-core/config"
	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/service/testutil"
)

func adminCfg() *config.Config {
	cfg := &config.Config{}
	cfg.Crypto.AESMasterKey = testutil.RevealTestKey
	cfg.Crypto.BlindIndexKey = testutil.RevealTestKey
	return cfg
}

func validCreateReq() domain.AnggotaCreateRequest {
	return domain.AnggotaCreateRequest{
		NamaLengkap: "Kader Baru Uji", NIK: "3201010101010001",
		TempatLahir: "Bandung", TanggalLahir: "2000-01-01T00:00:00Z",
		JenisKelamin: "L", Agama: "Islam", Pendidikan: "S1", Pekerjaan: "Mahasiswa",
		Alamat: "Jl. Uji No. 1, Bandung", ProvinsiID: 32, KabupatenID: 3273,
		Kecamatan: "Coblong", Desa: "Dago", KodePos: "40135",
		Email: "kader.baru@example.com", Whatsapp: "081234567890", Angkatan: "2026",
	}
}

func TestCreateAnggotaSukses(t *testing.T) {
	repo := &testutil.FakeAnggotaRepo{}
	svc := NewAnggotaService(adminCfg(), AnggotaDeps{AnggotaRepo: repo, WilayahRepo: &testutil.FakeWilayahRepo{}})
	out, err := svc.CreateAnggota(context.Background(), validCreateReq(), superActor32(), domain.AuditContext{})
	if err != nil {
		t.Fatalf("CreateAnggota gagal: %v", err)
	}
	if out.ID == 0 || out.NIA == "" {
		t.Fatalf("anggota tidak tersimpan dengan NIA: %+v", out)
	}
	if out.NIKHash == "" || out.NIKEncrypted == "" {
		t.Fatal("NIK wajib disimpan sebagai hash + terenkripsi")
	}
	if out.Status != domain.AnggotaStatusAktif {
		t.Fatalf("status default harus AKTIF, dapat %q", out.Status)
	}
}

func TestCreateAnggotaDuplikat(t *testing.T) {
	repo := &testutil.FakeAnggotaRepo{NikExists: true}
	svc := NewAnggotaService(adminCfg(), AnggotaDeps{AnggotaRepo: repo, WilayahRepo: &testutil.FakeWilayahRepo{}})
	if _, err := svc.CreateAnggota(context.Background(), validCreateReq(), superActor32(), domain.AuditContext{}); err == nil {
		t.Fatal("NIK duplikat seharusnya ditolak")
	}
}

func TestCreateAnggotaScopeDitolak(t *testing.T) {
	repo := &testutil.FakeAnggotaRepo{}
	svc := NewAnggotaService(adminCfg(), AnggotaDeps{AnggotaRepo: repo, WilayahRepo: &testutil.FakeWilayahRepo{}})
	req := validCreateReq()
	req.ProvinsiID = 99
	req.KabupatenID = 9999
	kab := domain.ActorContext{UserID: "u-kab", Name: "Kab", Role: domain.RoleAdminKabupaten, ProvinsiID: testutil.IntPtr(32), KabupatenID: testutil.IntPtr(3273)}
	if _, err := svc.CreateAnggota(context.Background(), req, kab, domain.AuditContext{}); err == nil {
		t.Fatal("wilayah di luar kewenangan harus ditolak")
	}
}

func TestUpdateAnggota(t *testing.T) {
	member := &domain.Anggota{
		ID: 7, NamaLengkap: "Lama", ProvinsiID: 32, KabupatenID: 3273,
		JenisKelamin: "L", Status: domain.AnggotaStatusAktif,
		Email: "lama@example.com", Whatsapp: "081234567890", Alamat: "Jl. Lama",
	}
	repo := &testutil.FakeAnggotaRepo{ByID: map[int]*domain.Anggota{7: member}}
	svc := NewAnggotaService(adminCfg(), AnggotaDeps{AnggotaRepo: repo, WilayahRepo: &testutil.FakeWilayahRepo{}})
	nama := "Nama Baru"
	out, err := svc.UpdateAnggota(context.Background(), 7, domain.AnggotaUpdateRequest{NamaLengkap: &nama}, superActor32(), domain.AuditContext{})
	if err != nil {
		t.Fatalf("UpdateAnggota gagal: %v", err)
	}
	if out.NamaLengkap != "Nama Baru" {
		t.Fatalf("nama tidak ter-update: %q", out.NamaLengkap)
	}
	result := domain.AnggotaUpdateRequest{KodePos: ptrTo("abc")}
	if _, err := svc.UpdateAnggota(context.Background(), 7, result, superActor32(), domain.AuditContext{}); err == nil {
		t.Fatal("kode pos tidak valid seharusnya ditolak")
	}
}

func TestSetAnggotaStatusSoftDelete(t *testing.T) {
	member := &domain.Anggota{ID: 8, ProvinsiID: 32, KabupatenID: 3273, Status: domain.AnggotaStatusAktif}
	repo := &testutil.FakeAnggotaRepo{ByID: map[int]*domain.Anggota{8: member}}
	svc := NewAnggotaService(adminCfg(), AnggotaDeps{AnggotaRepo: repo, WilayahRepo: &testutil.FakeWilayahRepo{}})
	out, err := svc.SetAnggotaStatus(context.Background(), 8, domain.AnggotaStatusRequest{Status: "NONAKTIF"}, superActor32(), domain.AuditContext{})
	if err != nil {
		t.Fatalf("SetAnggotaStatus gagal: %v", err)
	}
	if out.Status != domain.AnggotaStatusNonaktif || member.Status != domain.AnggotaStatusNonaktif {
		t.Fatalf("status harus NONAKTIF, dapat %q", out.Status)
	}
}

func TestExportCSVHeader(t *testing.T) {
	repo := &testutil.FakeAnggotaRepo{}
	svc := NewAnggotaService(adminCfg(), AnggotaDeps{AnggotaRepo: repo, WilayahRepo: &testutil.FakeWilayahRepo{}})
	data, err := svc.ExportCSV(context.Background(), superActor32(), "", "")
	if err != nil {
		t.Fatalf("ExportCSV gagal: %v", err)
	}
	if !strings.HasPrefix(string(data), "NIA,Nama,Pekerjaan,Riwayat,Provinsi,Kabupaten,Status") {
		t.Fatalf("header CSV salah: %q", string(data))
	}
}

func ptrTo(s string) *string { return &s }
