package generator

import (
	"fmt"
)

// GenerateRegistrationNumber menghasilkan nomor pendaftaran dengan format
// REG-YYYYMM-XXXXX (contoh: REG-202609-00001).
//
// year dan month menentukan periode; seq adalah nomor urut dalam periode
// tersebut. seq WAJIB dialokasikan dari database (NextRegistrationSequence)
// agar atomik — jangan pernah menghitung via SELECT MAX di aplikasi.
func GenerateRegistrationNumber(year, month, seq int) (string, error) {
	if year <= 0 {
		return "", fmt.Errorf("tahun pendaftaran tidak valid: %d", year)
	}
	if month < 1 || month > 12 {
		return "", fmt.Errorf("bulan pendaftaran tidak valid: %d", month)
	}
	if seq <= 0 {
		return "", fmt.Errorf("sequence tidak valid: %d", seq)
	}

	return fmt.Sprintf("REG-%d%02d-%05d", year, month, seq), nil
}
