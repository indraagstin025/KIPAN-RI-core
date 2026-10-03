package domain

// riwayat.go menghitung kolom RIWAYAT (TDD §5.5): jabatan aktif terbaru menang
// atas histori; bila tak ada, tampilkan riwayat terakhir; bila kosong '-'.

import (
	"fmt"
	"time"
)

// PengurusRiwayatRow adalah baris mentah riwayat kepengurusan satu anggota.
// StatusEfektif sudah dihitung di DB via pengurus_status_efektif (TDD §5.4).
type PengurusRiwayatRow struct {
	AnggotaID       int        `db:"anggota_id"`
	Level           string     `db:"level"`
	Jabatan         string     `db:"jabatan"`
	ProvinsiNama    *string    `db:"provinsi_nama"`
	KabupatenNama   *string    `db:"kabupaten_nama"`
	TanggalTerbit   time.Time  `db:"tanggal_terbit"`
	TanggalBerakhir *time.Time `db:"tanggal_berakhir"`
	StatusEfektif   string     `db:"status_efektif"`
	TanggalMulai    time.Time  `db:"tanggal_mulai"`
}

// wilayahRiwayat mengembalikan label wilayah tampil untuk baris riwayat.
func wilayahRiwayat(r PengurusRiwayatRow) string {
	switch TingkatWilayah(r.Level) {
	case LevelNasional:
		return "Nasional"
	case LevelProvinsi:
		if r.ProvinsiNama != nil && *r.ProvinsiNama != "" {
			return *r.ProvinsiNama
		}
		return "Provinsi"
	default:
		if r.KabupatenNama != nil && *r.KabupatenNama != "" {
			return *r.KabupatenNama
		}
		return "Kabupaten/Kota"
	}
}

// periodeRiwayat memformat rentang tahun bakti dari SK (mis. 2023-2026).
func periodeRiwayat(r PengurusRiwayatRow) string {
	if r.TanggalBerakhir != nil {
		return fmt.Sprintf("%d-%d", r.TanggalTerbit.Year(), r.TanggalBerakhir.Year())
	}
	return fmt.Sprintf("%d", r.TanggalTerbit.Year())
}

// FormatRiwayat menghitung teks kolom RIWAYAT dari kumpulan baris pengurus.
func FormatRiwayat(rows []PengurusRiwayatRow) string {
	if len(rows) == 0 {
		return "-"
	}

	// latest: baris terbaru menurut TanggalMulai (tie-break TanggalTerbit),
	// opsional difilter.
	latest := func(keep func(PengurusRiwayatRow) bool) (PengurusRiwayatRow, bool) {
		var best PengurusRiwayatRow
		found := false
		for _, r := range rows {
			if keep != nil && !keep(r) {
				continue
			}
			if !found || r.TanggalMulai.After(best.TanggalMulai) ||
				(r.TanggalMulai.Equal(best.TanggalMulai) && r.TanggalTerbit.After(best.TanggalTerbit)) {
				best = r
				found = true
			}
		}
		return best, found
	}

	if r, ok := latest(func(x PengurusRiwayatRow) bool { return x.StatusEfektif == "Aktif" }); ok {
		return fmt.Sprintf("Pengurus Aktif %s %s (Periode %s)", r.Jabatan, wilayahRiwayat(r), periodeRiwayat(r))
	}
	if r, ok := latest(nil); ok {
		return fmt.Sprintf("Demisioner %s %s (Periode %s)", r.Jabatan, wilayahRiwayat(r), periodeRiwayat(r))
	}
	return "-"
}
