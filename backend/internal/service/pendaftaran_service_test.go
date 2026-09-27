package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/kipan-indonesia/sim-kipan-core/config"
	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/crypto"
)

// fakeAnggotaRepo adalah anggotaRepo in-memory untuk uji VerifyKTA.
type fakeAnggotaRepo struct {
	byNIA map[string]*domain.Anggota
}

func (f *fakeAnggotaRepo) ExistsByNikHash(_ context.Context, _ string) (bool, error) {
	return false, nil
}

func (f *fakeAnggotaRepo) GetByNIA(_ context.Context, nia string) (*domain.Anggota, error) {
	if a, ok := f.byNIA[nia]; ok {
		return a, nil
	}
	return nil, domain.ErrNotFound
}

var _ repository.AnggotaRepository = (*fakeAnggotaRepo)(nil)

func TestValidateSubmitRequestAcceptsValidPayload(t *testing.T) {
	service := NewPendaftaranService(nil, nil, nil, nil)

	req := domain.PendaftaranSubmitRequest{
		NamaLengkap:        "Rizki Pratama",
		NIK:                "3201010101010001",
		TempatLahir:        "Bandung",
		TanggalLahir:       time.Date(1998, 5, 15, 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
		JenisKelamin:       "L",
		Agama:              "Islam",
		Pendidikan:         "S1",
		Pekerjaan:          "Software Engineer",
		StatusPribadi:      "Belum Menikah",
		Alamat:             "Jl. Merdeka No. 10, Bandung",
		ProvinsiID:         32,
		KabupatenID:        3273,
		Kecamatan:          "Cidadap",
		Desa:               "Ciumbuleuit",
		KodePos:            "40142",
		Email:              "rizki@example.com",
		Whatsapp:           "081234567890",
		Motivasi:           "Ingin belajar dan berkontribusi",
		FotoKey:            "uploads/foto.jpg",
		KTPKey:             "uploads/ktp.jpg",
		CVKey:              "uploads/cv.pdf",
		SKKey:              "uploads/sk.pdf",
		SuratPernyataanKey: "uploads/pernyataan.pdf",
		SuratSehatKey:      "uploads/sehat.pdf",
	}

	if err := service.ValidateSubmitRequest(req); err != nil {
		t.Fatalf("expected valid request to pass validation: %v", err)
	}
}

func TestValidateSubmitRequestRejectsInvalidNIK(t *testing.T) {
	service := NewPendaftaranService(nil, nil, nil, nil)

	req := domain.PendaftaranSubmitRequest{
		NamaLengkap:   "Rizki Pratama",
		NIK:           "12345",
		TempatLahir:   "Bandung",
		TanggalLahir:  time.Date(1998, 5, 15, 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
		JenisKelamin:  "L",
		Alamat:        "Jl. Merdeka No. 10, Bandung",
		ProvinsiID:    32,
		KabupatenID:   3273,
		Kecamatan:     "Cidadap",
		Desa:          "Ciumbuleuit",
		Email:         "rizki@example.com",
		Whatsapp:      "081234567890",
		FotoKey:       "uploads/foto.jpg",
		KTPKey:        "uploads/ktp.jpg",
	}

	if err := service.ValidateSubmitRequest(req); err == nil {
		t.Fatal("expected invalid NIK to be rejected")
	}
}

func validSubmitRequest() domain.PendaftaranSubmitRequest {
	return domain.PendaftaranSubmitRequest{
		NamaLengkap:        "Rizki Pratama",
		NIK:                "3201010101010001",
		TempatLahir:        "Bandung",
		TanggalLahir:       time.Date(1998, 5, 15, 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
		JenisKelamin:       "L",
		Alamat:             "Jl. Merdeka No. 10, Bandung",
		ProvinsiID:         32,
		KabupatenID:        3273,
		Kecamatan:          "Cidadap",
		Desa:               "Ciumbuleuit",
		KodePos:            "40142",
		Email:              "rizki@example.com",
		Whatsapp:           "081234567890",
		Motivasi:           "Ingin belajar dan berkontribusi",
		FotoKey:            "uploads/foto.jpg",
		KTPKey:             "uploads/ktp.jpg",
	}
}

func TestValidateSubmitRequestTable(t *testing.T) {
	service := NewPendaftaranService(nil, nil, nil, nil)

	cases := []struct {
		name   string
		mutate func(*domain.PendaftaranSubmitRequest)
	}{
		{"email tanpa @", func(r *domain.PendaftaranSubmitRequest) { r.Email = "asal" }},
		{"email terlalu panjang", func(r *domain.PendaftaranSubmitRequest) {
			r.Email = strings.Repeat("a", 255) + "@b.co"
		}},
		{"WA prefix 62 valid", nil}, // dicek terpisah sebagai kasus valid
		{"WA abjad", func(r *domain.PendaftaranSubmitRequest) { r.Whatsapp = "abc123" }},
		{"WA diawali 080", func(r *domain.PendaftaranSubmitRequest) { r.Whatsapp = "080000000000" }},
		{"WA terlalu panjang", func(r *domain.PendaftaranSubmitRequest) { r.Whatsapp = "08123456789012345" }},
		{"NIK tanggal fiktif", func(r *domain.PendaftaranSubmitRequest) { r.NIK = "3201019901010001" }},
		{"NIK nol semua", func(r *domain.PendaftaranSubmitRequest) { r.NIK = "0000000000000000" }},
		{"DOB masa depan", func(r *domain.PendaftaranSubmitRequest) {
			r.TanggalLahir = time.Now().Add(24 * time.Hour).Format(time.RFC3339)
		}},
		{"DOB anak 5 tahun", func(r *domain.PendaftaranSubmitRequest) {
			r.TanggalLahir = time.Now().AddDate(-5, 0, 0).Format(time.RFC3339)
		}},
		{"nama 1 char", func(r *domain.PendaftaranSubmitRequest) { r.NamaLengkap = "A" }},
		{"nama 200 char", func(r *domain.PendaftaranSubmitRequest) {
			r.NamaLengkap = strings.Repeat("a", 200)
		}},
		{"nama XSS", func(r *domain.PendaftaranSubmitRequest) { r.NamaLengkap = "<script>alert(1)</script>" }},
		{"alamat XSS", func(r *domain.PendaftaranSubmitRequest) { r.Alamat = "Jl <img src=x onerror=alert(1)>" }},
		{"motivasi XSS", func(r *domain.PendaftaranSubmitRequest) { r.Motivasi = "<b>halo</b>" }},
		{"key traversal", func(r *domain.PendaftaranSubmitRequest) { r.KTPKey = "../../etc/passwd" }},
		{"key karakter ilegal", func(r *domain.PendaftaranSubmitRequest) { r.KTPKey = "uploads/ktp?.jpg" }},
		{"foto kosong", func(r *domain.PendaftaranSubmitRequest) { r.FotoKey = "" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.mutate == nil {
				return // kasus valid, diuji di bawah
			}
			req := validSubmitRequest()
			tc.mutate(&req)
			if err := service.ValidateSubmitRequest(req); err == nil {
				t.Fatalf("expected %q to be rejected", tc.name)
			}
		})
	}

	// Prefix 62 tetap valid.
	req := validSubmitRequest()
	req.Whatsapp = "6281234567890"
	if err := service.ValidateSubmitRequest(req); err != nil {
		t.Fatalf("expected 62-prefix WhatsApp to pass: %v", err)
	}
	req.Whatsapp = "+6281234567890"
	if err := service.ValidateSubmitRequest(req); err != nil {
		t.Fatalf("expected +62-prefix WhatsApp to pass: %v", err)
	}
}

func TestActorCanAccessWilayah(t *testing.T) {
	prov32, prov33 := 32, 33
	kab3273, kab3204 := 3273, 3204

	cases := []struct {
		name   string
		actor  domain.ActorContext
		prov   int
		kab    int
		access bool
	}{
		{"super admin bebas", domain.ActorContext{Role: domain.RoleSuperAdmin}, 33, 3204, true},
		{"nasional bebas", domain.ActorContext{Role: domain.RoleAdminNasional}, 33, 3204, true},
		{"provinsi cocok", domain.ActorContext{Role: domain.RoleAdminProvinsi, ProvinsiID: &prov32}, 32, 3273, true},
		{"provinsi beda", domain.ActorContext{Role: domain.RoleAdminProvinsi, ProvinsiID: &prov32}, 33, 3204, false},
		{"provinsi tanpa wilayah", domain.ActorContext{Role: domain.RoleAdminProvinsi}, 32, 3273, false},
		{"kabupaten cocok", domain.ActorContext{Role: domain.RoleAdminKabupaten, ProvinsiID: &prov32, KabupatenID: &kab3273}, 32, 3273, true},
		{"kabupaten beda", domain.ActorContext{Role: domain.RoleAdminKabupaten, ProvinsiID: &prov32, KabupatenID: &kab3273}, 32, kab3204, false},
		{"kabupaten provinsi inkonsisten", domain.ActorContext{Role: domain.RoleAdminKabupaten, ProvinsiID: &prov33, KabupatenID: &kab3273}, 32, 3273, false},
		{"kabupaten tanpa kabupaten", domain.ActorContext{Role: domain.RoleAdminKabupaten, ProvinsiID: &prov32}, 32, 3273, false},
		{"role tak dikenal", domain.ActorContext{Role: "ANON"}, 32, 3273, false},
		{"role kosong", domain.ActorContext{}, 32, 3273, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.actor.CanAccessWilayah(tc.prov, tc.kab); got != tc.access {
				t.Fatalf("CanAccessWilayah(%d,%d) = %v, harap %v", tc.prov, tc.kab, got, tc.access)
			}
		})
	}
}

func TestVerifyKTAAcceptsActiveKey(t *testing.T) {
	const keyActive = "aa00112233445566778899aabbccddeeffaa00112233445566778899aabbccdd"
	member := &domain.Anggota{
		ID:            7,
		NIA:           "KIPAN-32-3273-2026-00007",
		NamaLengkap:   "Uji Rotasi",
		Status:        domain.AnggotaStatusAktif,
		TanggalAngkat: time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC),
	}
	sig, err := crypto.KTASignature(member.NIA, "2026-09-27", member.ID, keyActive)
	if err != nil {
		t.Fatalf("KTASignature gagal: %v", err)
	}
	cfg := &config.Config{}
	cfg.Crypto.KTASigningKey = keyActive
	svc := NewPendaftaranService(cfg, nil, &fakeAnggotaRepo{byNIA: map[string]*domain.Anggota{member.NIA: member}}, nil)

	res, err := svc.VerifyKTA(context.Background(), member.NIA, sig)
	if err != nil {
		t.Fatalf("VerifyKTA gagal: %v", err)
	}
	if !res.Valid || res.NamaLengkap != "Uji Rotasi" {
		t.Fatalf("kartu valid ditolak: %+v", res)
	}
}

func TestVerifyKTAAcceptsPreviousKeyAfterRotation(t *testing.T) {
	const keyOld = "bb00112233445566778899aabbccddeeffbb00112233445566778899aabbccdd"
	const keyNew = "cc00112233445566778899aabbccddeeffcc00112233445566778899aabbccdd"
	member := &domain.Anggota{
		ID:            8,
		NIA:           "KIPAN-32-3273-2026-00008",
		NamaLengkap:   "Uji Rotasi Lama",
		Status:        domain.AnggotaStatusAktif,
		TanggalAngkat: time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC),
	}
	// Kartu diterbitkan SEBELUM rotasi (kunci lama).
	sig, err := crypto.KTASignature(member.NIA, "2026-09-27", member.ID, keyOld)
	if err != nil {
		t.Fatalf("KTASignature gagal: %v", err)
	}
	// Rotasi: aktif = baru, prev = lama.
	cfg := &config.Config{}
	cfg.Crypto.KTASigningKey = keyNew
	cfg.Crypto.KTASigningKeyPrev = keyOld
	svc := NewPendaftaranService(cfg, nil, &fakeAnggotaRepo{byNIA: map[string]*domain.Anggota{member.NIA: member}}, nil)

	res, err := svc.VerifyKTA(context.Background(), member.NIA, sig)
	if err != nil {
		t.Fatalf("VerifyKTA gagal: %v", err)
	}
	if !res.Valid {
		t.Fatal("kartu lama DITOLAK setelah rotasi — rotasi mematikan kartu beredar")
	}
}

func TestVerifyKTARejectsUnknownKey(t *testing.T) {
	const keyActive = "aa00112233445566778899aabbccddeeffaa00112233445566778899aabbccdd"
	const keyEvil = "ee00112233445566778899aabbccddeeffee00112233445566778899aabbccdd"
	member := &domain.Anggota{
		ID:            9,
		NIA:           "KIPAN-32-3273-2026-00009",
		Status:        domain.AnggotaStatusAktif,
		TanggalAngkat: time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC),
	}
	sig, err := crypto.KTASignature(member.NIA, "2026-09-27", member.ID, keyEvil)
	if err != nil {
		t.Fatalf("KTASignature gagal: %v", err)
	}
	cfg := &config.Config{}
	cfg.Crypto.KTASigningKey = keyActive
	svc := NewPendaftaranService(cfg, nil, &fakeAnggotaRepo{byNIA: map[string]*domain.Anggota{member.NIA: member}}, nil)

	res, err := svc.VerifyKTA(context.Background(), member.NIA, sig)
	if err != nil {
		t.Fatalf("VerifyKTA gagal: %v", err)
	}
	if res.Valid {
		t.Fatal("signature kunci asing DITERIMA")
	}
}

func TestValidateSubmitRequestRejectsInvalidWhatsapp(t *testing.T) {
	service := NewPendaftaranService(nil, nil, nil, nil)

	req := domain.PendaftaranSubmitRequest{
		NamaLengkap:   "Rizki Pratama",
		NIK:           "3201010101010001",
		TempatLahir:   "Bandung",
		TanggalLahir:  time.Date(1998, 5, 15, 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
		JenisKelamin:  "L",
		Alamat:        "Jl. Merdeka No. 10, Bandung",
		ProvinsiID:    32,
		KabupatenID:   3273,
		Kecamatan:     "Cidadap",
		Desa:          "Ciumbuleuit",
		Email:         "rizki@example.com",
		Whatsapp:      "abc123",
		FotoKey:       "uploads/foto.jpg",
		KTPKey:        "uploads/ktp.jpg",
	}

	if err := service.ValidateSubmitRequest(req); err == nil {
		t.Fatal("expected invalid WhatsApp number to be rejected")
	}
}
