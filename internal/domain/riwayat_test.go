package domain

import (
	"testing"
	"time"
)

func ptrStr(s string) *string        { return &s }
func ptrTime(t time.Time) *time.Time { return &t }

// TestFormatRiwayat mengunci algoritma tiga kolom RIWAYAT (TDD §5.5) dengan
// tiga skenario kader "Mirwan".
func TestFormatRiwayat(t *testing.T) {
	jabar := "Jawa Barat"
	bbr := "Bandung Barat"

	// 1) Sedang menjabat di provinsi (aktif menang atas histori).
	aktif := []PengurusRiwayatRow{
		{Level: "KABUPATEN", Jabatan: "Ketua Umum", KabupatenNama: &bbr,
			TanggalTerbit:   time.Date(2017, 1, 1, 0, 0, 0, 0, time.UTC),
			TanggalBerakhir: ptrTime(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)),
			StatusEfektif:   "Demisioner", TanggalMulai: time.Date(2017, 1, 1, 0, 0, 0, 0, time.UTC)},
		{Level: "PROVINSI", Jabatan: "Ketua Umum", ProvinsiNama: &jabar,
			TanggalTerbit:   time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			TanggalBerakhir: ptrTime(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)),
			StatusEfektif:   "Aktif", TanggalMulai: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)},
	}
	if got, want := FormatRiwayat(aktif), "Pengurus Aktif Ketua Umum Jawa Barat (Periode 2023-2026)"; got != want {
		t.Fatalf("aktif: mau %q, dapat %q", want, got)
	}

	// 2) Purna tugas dua tingkat → tampilkan riwayat TERAKHIR.
	purna := []PengurusRiwayatRow{
		{Level: "KABUPATEN", Jabatan: "Ketua Umum", KabupatenNama: &bbr,
			TanggalTerbit:   time.Date(2017, 1, 1, 0, 0, 0, 0, time.UTC),
			TanggalBerakhir: ptrTime(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)),
			StatusEfektif:   "Demisioner", TanggalMulai: time.Date(2017, 1, 1, 0, 0, 0, 0, time.UTC)},
		{Level: "PROVINSI", Jabatan: "Ketua Umum", ProvinsiNama: &jabar,
			TanggalTerbit:   time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
			TanggalBerakhir: ptrTime(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)),
			StatusEfektif:   "Demisioner", TanggalMulai: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)},
	}
	if got, want := FormatRiwayat(purna), "Demisioner Ketua Umum Jawa Barat (Periode 2020-2023)"; got != want {
		t.Fatalf("purna: mau %q, dapat %q", want, got)
	}

	// 3) Kader biasa.
	if got := FormatRiwayat(nil); got != "-" {
		t.Fatalf("kosong: mau '-', dapat %q", got)
	}
}
