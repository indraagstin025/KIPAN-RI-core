package service

import (
	"context"
	"fmt"
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
	byNIA     map[string]*domain.Anggota
	byID      map[int]*domain.Anggota
	links     map[int]string
	nikExists bool
}

func (f *fakeAnggotaRepo) ExistsByNikHash(_ context.Context, _ string) (bool, error) {
	return f.nikExists, nil
}

func (f *fakeAnggotaRepo) GetByNIA(_ context.Context, nia string) (*domain.Anggota, error) {
	if a, ok := f.byNIA[nia]; ok {
		return a, nil
	}
	return nil, domain.ErrNotFound
}

func (f *fakeAnggotaRepo) GetByID(_ context.Context, id int) (*domain.Anggota, error) {
	if a, ok := f.byID[id]; ok && a != nil {
		return a, nil
	}
	return nil, domain.ErrNotFound
}

func (f *fakeAnggotaRepo) SetKTAPDFKey(_ context.Context, _ int, _ string) error {
	return nil
}

func (f *fakeAnggotaRepo) SetStatus(_ context.Context, id int, status domain.AnggotaStatus) error {
	if a, ok := f.byID[id]; ok && a != nil {
		a.Status = status
	}
	return nil
}

func (f *fakeAnggotaRepo) RiwayatByAnggotaIDs(_ context.Context, ids []int) (map[int]string, error) {
	out := make(map[int]string, len(ids))
	for _, id := range ids {
		out[id] = "-"
	}
	return out, nil
}

func (f *fakeAnggotaRepo) AllocateNIA(_ context.Context, _, _, year int) (string, error) {
	return fmt.Sprintf("KIPAN-IND-9999-%d-000001", year), nil
}

func (f *fakeAnggotaRepo) Create(_ context.Context, a *domain.Anggota) (*domain.Anggota, error) {
	if a.ID == 0 {
		a.ID = 100
	}
	return a, nil
}

func (f *fakeAnggotaRepo) Update(_ context.Context, _ *domain.Anggota) error {
	return nil
}

func (f *fakeAnggotaRepo) SetUserID(_ context.Context, id int, userID string) error {
	if f.links == nil {
		f.links = map[int]string{}
	}
	f.links[id] = userID
	if a, ok := f.byID[id]; ok && a != nil {
		a.UserID = &userID
	}
	return nil
}

func (f *fakeAnggotaRepo) GetByUserID(_ context.Context, userID string) (*domain.Anggota, error) {
	for _, a := range f.byNIA {
		if a != nil && a.UserID != nil && *a.UserID == userID {
			return a, nil
		}
	}
	for id, uid := range f.links {
		if uid == userID {
			if a, ok := f.byID[id]; ok {
				return a, nil
			}
		}
	}
	return nil, domain.ErrNotFound
}

func (f *fakeAnggotaRepo) ListAnggota(_ context.Context, _, _ *int, _, _ string, _, _ int) ([]domain.AnggotaListItem, error) {
	return []domain.AnggotaListItem{}, nil
}

func (f *fakeAnggotaRepo) CountAnggota(_ context.Context, _, _ *int, _, _ string) (int, error) {
	return 0, nil
}

var _ repository.AnggotaRepository = (*fakeAnggotaRepo)(nil)

// Uji Batch 2: pesan WA berisi nomor REG + tautan lacak.
func TestRegistrantWAMessage(t *testing.T) {
	msg := registrantWAMessage("Rizki Pratama", "REG-202610-00042", "https://sim.kipan.id")
	for _, want := range []string{
		"Rizki Pratama",
		"REG-202610-00042",
		"https://sim.kipan.id/lacak?nomor=REG-202610-00042",
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("pesan WA tidak memuat %q:\n%s", want, msg)
		}
	}
}

func TestValidateSubmitRequestAcceptsValidPayload(t *testing.T) {
	service := NewPendaftaranService(nil, PendaftaranDeps{})

	req := domain.PendaftaranSubmitRequest{
		NamaLengkap:        "Rizki Pratama",
		NIK:                "3201010101010001",
		TempatLahir:        "Bandung",
		TanggalLahir:       time.Date(1998, 5, 15, 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
		JenisKelamin:       "L",
		TipePendaftaran:    "KADER",
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
		WaOTPToken:         "test-token-format-ok",
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
	service := NewPendaftaranService(nil, PendaftaranDeps{})

	req := domain.PendaftaranSubmitRequest{
		NamaLengkap:     "Rizki Pratama",
		NIK:             "12345",
		TempatLahir:     "Bandung",
		TanggalLahir:    time.Date(1998, 5, 15, 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
		JenisKelamin:    "L",
		TipePendaftaran: "KADER",
		Alamat:          "Jl. Merdeka No. 10, Bandung",
		ProvinsiID:      32,
		KabupatenID:     3273,
		Kecamatan:       "Cidadap",
		Desa:            "Ciumbuleuit",
		Email:           "rizki@example.com",
		Whatsapp:        "081234567890",
		WaOTPToken:      "test-token-format-ok",
		FotoKey:         "uploads/foto.jpg",
		KTPKey:          "uploads/ktp.jpg",
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
		TipePendaftaran:    "KADER",
		Agama:              "Islam",
		Pendidikan:         "S1",
		Pekerjaan:          "Software Engineer",
		Alamat:             "Jl. Merdeka No. 10, Bandung",
		ProvinsiID:         32,
		KabupatenID:        3273,
		Kecamatan:          "Cidadap",
		Desa:               "Ciumbuleuit",
		KodePos:            "40142",
		Email:              "rizki@example.com",
		Whatsapp:           "081234567890",
		WaOTPToken:         "test-token-format-ok",
		Motivasi:           "Ingin belajar dan berkontribusi",
		FotoKey:            "uploads/foto.jpg",
		KTPKey:             "uploads/ktp.jpg",
		CVKey:              "uploads/cv.pdf",
		SuratPernyataanKey: "uploads/pernyataan.pdf",
		SuratSehatKey:      "uploads/sehat.pdf",
	}
}

func TestValidateSubmitRequestTable(t *testing.T) {
	service := NewPendaftaranService(nil, PendaftaranDeps{})

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
		{"DOB usia 40 tahun (di atas 30)", func(r *domain.PendaftaranSubmitRequest) {
			r.TanggalLahir = time.Now().AddDate(-40, 0, 0).Format(time.RFC3339)
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
		{"agama kosong", func(r *domain.PendaftaranSubmitRequest) { r.Agama = "" }},
		{"pendidikan kosong", func(r *domain.PendaftaranSubmitRequest) { r.Pendidikan = "  " }},
		{"pekerjaan kosong", func(r *domain.PendaftaranSubmitRequest) { r.Pekerjaan = "" }},
		{"kecamatan kosong", func(r *domain.PendaftaranSubmitRequest) { r.Kecamatan = "" }},
		{"desa kosong", func(r *domain.PendaftaranSubmitRequest) { r.Desa = "" }},
		{"kode pos kosong", func(r *domain.PendaftaranSubmitRequest) { r.KodePos = "" }},
		{"kode pos 4 digit", func(r *domain.PendaftaranSubmitRequest) { r.KodePos = "1234" }},
		{"kode pos alfanumerik", func(r *domain.PendaftaranSubmitRequest) { r.KodePos = "40A42" }},
		{"motivasi kosong", func(r *domain.PendaftaranSubmitRequest) { r.Motivasi = "" }},
		{"motivasi terlalu pendek", func(r *domain.PendaftaranSubmitRequest) { r.Motivasi = "Ikut kipan" }},
		{"cv kosong", func(r *domain.PendaftaranSubmitRequest) { r.CVKey = "" }},
		{"surat pernyataan kosong", func(r *domain.PendaftaranSubmitRequest) { r.SuratPernyataanKey = "" }},
		{"surat sehat kosong", func(r *domain.PendaftaranSubmitRequest) { r.SuratSehatKey = "" }},
		{"agama XSS", func(r *domain.PendaftaranSubmitRequest) { r.Agama = "Isl<script>am" }},
		{"wa otp token kosong", func(r *domain.PendaftaranSubmitRequest) { r.WaOTPToken = "" }},
		{"wa otp token spasi", func(r *domain.PendaftaranSubmitRequest) { r.WaOTPToken = "   " }},
		{"tipe kosong", func(r *domain.PendaftaranSubmitRequest) { r.TipePendaftaran = "" }},
		{"tipe salah", func(r *domain.PendaftaranSubmitRequest) { r.TipePendaftaran = "ADMIN" }},
		{"pengurus ditolak walau ada SK", func(r *domain.PendaftaranSubmitRequest) { r.TipePendaftaran = "PENGURUS"; r.SKKey = "uploads/sk.pdf" }},
		{"foto dan KTP key sama", func(r *domain.PendaftaranSubmitRequest) { r.KTPKey = r.FotoKey }},
		{"CV dan surat pernyataan key sama", func(r *domain.PendaftaranSubmitRequest) { r.SuratPernyataanKey = r.CVKey }},
		{"SK dan foto key sama (kader)", func(r *domain.PendaftaranSubmitRequest) { r.SKKey = r.FotoKey }},
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

	// Jalur PENGURUS DITOLAK (kader saja yang boleh daftar; pengurus via
	// pengangkatan SK). Lowercase tetap dinormalisasi sebelum ditolak.
	req := validSubmitRequest()
	req.TipePendaftaran = "pengurus"
	req.SKKey = "uploads/sk.pdf"
	if err := service.ValidateSubmitRequest(req); err == nil {
		t.Fatal("expected pengurus+SK to be rejected")
	}

	// SK kosong + status pribadi kosong tetap lolos (kader tidak melampirkan SK).
	req = validSubmitRequest()
	req.SKKey = ""
	req.StatusPribadi = ""
	if err := service.ValidateSubmitRequest(req); err != nil {
		t.Fatalf("expected empty SK to pass: %v", err)
	}

	// Prefix 62 tetap valid.
	req.Whatsapp = "6281234567890"
	if err := service.ValidateSubmitRequest(req); err != nil {
		t.Fatalf("expected 62-prefix WhatsApp to pass: %v", err)
	}
	req.Whatsapp = "+6281234567890"
	if err := service.ValidateSubmitRequest(req); err != nil {
		t.Fatalf("expected +62-prefix WhatsApp to pass: %v", err)
	}

	// Batas umur 16-30 (selaras KIPAN_INDONESIA): 16 dan 30 lolos.
	for _, age := range []int{16, 30} {
		req := validSubmitRequest()
		req.TanggalLahir = time.Now().AddDate(-age, 0, 0).Format(time.RFC3339)
		if err := service.ValidateSubmitRequest(req); err != nil {
			t.Fatalf("expected age %d to pass: %v", age, err)
		}
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
		{"user tanpa akses wilayah", domain.ActorContext{Role: domain.RoleUser, UserID: "u9"}, 32, 3273, false},
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
		NIA:           "KIPAN-IND-3273-2026-000007",
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
	svc := NewVerificationService(cfg, VerificationDeps{AnggotaRepo: &fakeAnggotaRepo{byNIA: map[string]*domain.Anggota{member.NIA: member}}})

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
		NIA:           "KIPAN-IND-3273-2026-000008",
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
	svc := NewVerificationService(cfg, VerificationDeps{AnggotaRepo: &fakeAnggotaRepo{byNIA: map[string]*domain.Anggota{member.NIA: member}}})

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
		NIA:           "KIPAN-IND-3273-2026-000009",
		Status:        domain.AnggotaStatusAktif,
		TanggalAngkat: time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC),
	}
	sig, err := crypto.KTASignature(member.NIA, "2026-09-27", member.ID, keyEvil)
	if err != nil {
		t.Fatalf("KTASignature gagal: %v", err)
	}
	cfg := &config.Config{}
	cfg.Crypto.KTASigningKey = keyActive
	svc := NewVerificationService(cfg, VerificationDeps{AnggotaRepo: &fakeAnggotaRepo{byNIA: map[string]*domain.Anggota{member.NIA: member}}})

	res, err := svc.VerifyKTA(context.Background(), member.NIA, sig)
	if err != nil {
		t.Fatalf("VerifyKTA gagal: %v", err)
	}
	if res.Valid {
		t.Fatal("signature kunci asing DITERIMA")
	}
}

func TestMatchOwnerProof(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		storedEmail, storedWA string
		proofEmail, proofWA   string
		ok                    bool
	}{
		{"cocok persis", "a@b.co", "081234567890", "a@b.co", "081234567890", true},
		{"email case-insensitive + spasi", "A@B.co", "081234567890", "  a@b.co ", "081234567890", true},
		{"WA 62 setara 08", "a@b.co", "081234567890", "a@b.co", "6281234567890", true},
		{"WA +62 setara 08", "a@b.co", "081234567890", "a@b.co", "+6281234567890", true},
		{"WA strip/spasi diabaikan", "a@b.co", "081234567890", "a@b.co", "0812-3456-7890", true},
		{"email salah", "a@b.co", "081234567890", "x@b.co", "081234567890", false},
		{"WA salah", "a@b.co", "081234567890", "a@b.co", "081234567899", false},
		{"keduanya salah", "a@b.co", "081234567890", "x@y.zz", "080000000000", false},
		{"bukti kosong", "a@b.co", "081234567890", "", "", false},
		{"email kosong", "a@b.co", "081234567890", "", "081234567890", false},
		{"WA kosong", "a@b.co", "081234567890", "a@b.co", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := MatchOwnerProof(tc.storedEmail, tc.storedWA, tc.proofEmail, tc.proofWA); got != tc.ok {
				t.Fatalf("MatchOwnerProof = %v, harap %v", got, tc.ok)
			}
		})
	}
}

func TestValidateSubmitRequestRejectsInvalidWhatsapp(t *testing.T) {
	service := NewPendaftaranService(nil, PendaftaranDeps{})

	req := domain.PendaftaranSubmitRequest{
		NamaLengkap:     "Rizki Pratama",
		NIK:             "3201010101010001",
		TempatLahir:     "Bandung",
		TanggalLahir:    time.Date(1998, 5, 15, 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
		JenisKelamin:    "L",
		TipePendaftaran: "KADER",
		Alamat:          "Jl. Merdeka No. 10, Bandung",
		ProvinsiID:      32,
		KabupatenID:     3273,
		Kecamatan:       "Cidadap",
		Desa:            "Ciumbuleuit",
		Email:           "rizki@example.com",
		Whatsapp:        "abc123",
		FotoKey:         "uploads/foto.jpg",
		KTPKey:          "uploads/ktp.jpg",
	}

	if err := service.ValidateSubmitRequest(req); err == nil {
		t.Fatal("expected invalid WhatsApp number to be rejected")
	}
}
