package nia

import (
	"fmt"
	"regexp"
)

// Format kanonis NIA (RULES 18): KIPAN-[PROV]-[KAB]-[TAHUN]-[NO_URUT].
// PROV/KAB adalah kode BPS wilayah (mis. "32", "3273"), TAHUN 4 digit,
// NO_URUT 5 digit. Contoh: KIPAN-32-3273-2026-00001.
var kodePattern = regexp.MustCompile(`^[A-Za-z0-9]{1,10}$`)

// GenerateNIA menghasilkan nomor induk anggota. provKode/kabKode wajib kode
// BPS resmi dari tabel wilayah (bukan ID serial) agar NIA stabil dan terbaca.
func GenerateNIA(provKode, kabKode string, tahun, seq int) (string, error) {
	if !kodePattern.MatchString(provKode) {
		return "", fmt.Errorf("kode provinsi tidak valid: %q", provKode)
	}
	if !kodePattern.MatchString(kabKode) {
		return "", fmt.Errorf("kode kabupaten tidak valid: %q", kabKode)
	}
	if tahun <= 0 {
		return "", fmt.Errorf("tahun tidak valid: %d", tahun)
	}
	if seq <= 0 {
		return "", fmt.Errorf("sequence tidak valid: %d", seq)
	}

	return fmt.Sprintf("KIPAN-%s-%s-%d-%05d", provKode, kabKode, tahun, seq), nil
}
