package kta

import (
	"bytes"
	"fmt"
	"time"

	"github.com/jung-kurt/gofpdf"
	"github.com/skip2/go-qrcode"
)

// CardData adalah data yang tercetak di KTA. Foto tidak dicetak di PDF
// tahap ini (butuh fetch object + resize); verifikasi visual tetap lewat
// endpoint /kta/:nia yang menampilkan foto dari server.
type CardData struct {
	NIA           string
	NamaLengkap   string
	Status        string
	TanggalAngkat time.Time
	VerifyURL     string
}

// QRBytes membuat QR PNG dari URL verifikasi (murni, unit-testable).
func QRBytes(verifyURL string) ([]byte, error) {
	if verifyURL == "" {
		return nil, fmt.Errorf("verify URL kosong")
	}
	return qrcode.Encode(verifyURL, qrcode.Medium, 256)
}

// RenderPDF merender kartu KTA satu halaman (CR80 landscape-ish A6) murni
// server-side. Frontend hanya mengunduh artefak ini (RULES 19).
func RenderPDF(card CardData, qrPNG []byte) ([]byte, error) {
	if card.NIA == "" || card.NamaLengkap == "" {
		return nil, fmt.Errorf("data kartu tidak lengkap")
	}
	if len(qrPNG) == 0 {
		return nil, fmt.Errorf("QR code kosong")
	}

	pdf := gofpdf.New("L", "mm", "A6", "")
	pdf.SetMargins(10, 10, 10)
	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 16)
	pdf.CellFormat(0, 10, "KARTU TANDA ANGGOTA", "", 1, "C", false, 0, "")
	pdf.SetFont("Helvetica", "", 11)
	pdf.CellFormat(0, 7, "Kader Inti Pemuda Anti Narkoba (KIPAN)", "", 1, "C", false, 0, "")
	pdf.Ln(3)

	pdf.SetFont("Helvetica", "", 12)
	row := func(label, value string) {
		pdf.SetFont("Helvetica", "B", 12)
		pdf.CellFormat(38, 8, label, "", 0, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 12)
		pdf.CellFormat(0, 8, value, "", 1, "L", false, 0, "")
	}
	row("NIA", card.NIA)
	row("Nama", card.NamaLengkap)
	row("Status", card.Status)
	row("Tanggal Angkat", card.TanggalAngkat.Format("02-01-2006"))

	// QR di kanan bawah; gofpdf butuh reader bernama dengan tipe terdaftar.
	imgOpts := gofpdf.ImageOptions{ImageType: "PNG", ReadDpi: true}
	pdf.RegisterImageOptionsReader("qr", imgOpts, bytes.NewReader(qrPNG))
	pageW, pageH := pdf.GetPageSize()
	pdf.ImageOptions("qr", pageW-50, pageH-50, 38, 38, false, imgOpts, 0, "")
	pdf.SetFont("Helvetica", "I", 8)
	pdf.SetXY(10, pageH-14)
	pdf.CellFormat(0, 5, "Pindai QR untuk verifikasi keaslian di "+card.VerifyURL, "", 0, "L", false, 0, "")

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("gagal render PDF KTA: %w", err)
	}
	return buf.Bytes(), nil
}
