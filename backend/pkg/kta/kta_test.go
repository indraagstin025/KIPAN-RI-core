package kta

import (
	"bytes"
	"testing"
	"time"
)

func TestQRBytesRoundtrip(t *testing.T) {
	qr, err := QRBytes("https://kipan.id/v/KIPAN-32-3273-2026-00001?sig=abc")
	if err != nil {
		t.Fatalf("QRBytes gagal: %v", err)
	}
	if !bytes.HasPrefix(qr, []byte{0x89, 0x50, 0x4E, 0x47}) {
		t.Fatal("QR bukan PNG valid")
	}
	if _, err := QRBytes(""); err == nil {
		t.Fatal("URL kosong DITERIMA")
	}
}

func TestRenderPDFValid(t *testing.T) {
	qr, err := QRBytes("https://kipan.id/v/KIPAN-32-3273-2026-00001?sig=abc")
	if err != nil {
		t.Fatalf("QRBytes gagal: %v", err)
	}
	pdf, err := RenderPDF(CardData{
		NIA:           "KIPAN-32-3273-2026-00001",
		NamaLengkap:   "Uji KTA",
		Status:        "AKTIF",
		TanggalAngkat: time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC),
		VerifyURL:     "https://kipan.id/v/KIPAN-32-3273-2026-00001?sig=abc",
	}, qr)
	if err != nil {
		t.Fatalf("RenderPDF gagal: %v", err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Fatal("output bukan PDF valid")
	}
	if len(pdf) < 1000 {
		t.Fatalf("PDF terlalu kecil (%d byte), render mungkin gagal", len(pdf))
	}
}

func TestRenderPDFRejectsIncomplete(t *testing.T) {
	qr, _ := QRBytes("https://kipan.id/x")
	if _, err := RenderPDF(CardData{}, qr); err == nil {
		t.Fatal("kartu kosong DITERIMA")
	}
	if _, err := RenderPDF(CardData{NIA: "x", NamaLengkap: "y"}, nil); err == nil {
		t.Fatal("QR kosong DITERIMA")
	}
}
