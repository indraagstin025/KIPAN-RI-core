package service

import (
	"testing"
	"time"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
)

func TestValidateSubmitRequestAcceptsValidPayload(t *testing.T) {
	service := NewPendaftaranService(nil)

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
	service := NewPendaftaranService(nil)

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

func TestValidateSubmitRequestRejectsInvalidWhatsapp(t *testing.T) {
	service := NewPendaftaranService(nil)

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
