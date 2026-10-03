package nia

import (
	"fmt"
	"regexp"
)

// Format kanonis NIA (RULES 18): KIPAN-IND-[KAB]-[TAHUN]-[NO_URUT].
// Segmen "IND" tetap (kode Indonesia); KAB adalah kode BPS kabupaten/kota
// 4 digit (sudah memuat kode provinsi, mis. "3204"); TAHUN 4 digit;
// NO_URUT 6 digit. Contoh: KIPAN-IND-3204-2026-000001.
var kodePattern = regexp.MustCompile(`^[A-Za-z0-9]{1,10}$`)

const (
	prefixNIA   = "KIPAN-IND-"
	maxSequence = 999999
)

// GenerateNIA menghasilkan nomor induk anggota. kabKode wajib kode BPS resmi
// kabupaten/kota (bukan ID serial) agar NIA stabil dan terbaca.
func GenerateNIA(kabKode string, tahun, seq int) (string, error) {
	if !kodePattern.MatchString(kabKode) {
		return "", fmt.Errorf("kode kabupaten tidak valid: %q", kabKode)
	}
	if tahun <= 0 {
		return "", fmt.Errorf("tahun tidak valid: %d", tahun)
	}
	if seq <= 0 || seq > maxSequence {
		return "", fmt.Errorf("sequence tidak valid: %d", seq)
	}

	return fmt.Sprintf("%s%s-%d-%06d", prefixNIA, kabKode, tahun, seq), nil
}
