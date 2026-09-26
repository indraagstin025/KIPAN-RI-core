package generator

import (
	"fmt"
	"time"
)

// GenerateRegistrationNumber menghasilkan nomor pendaftaran dengan format
// REG-YYYYMM-XXXXX.
//
// Parameter year dipakai sebagai tahun pendaftaran, sedangkan seq dipakai sebagai
// nomor urut pendaftar dalam bulan berjalan.
func GenerateRegistrationNumber(year int, seq int) (string, error) {
	if year <= 0 {
		return "", fmt.Errorf("tahun pendaftaran tidak valid: %d", year)
	}
	if seq < 0 {
		return "", fmt.Errorf("sequence tidak valid: %d", seq)
	}

	month := time.Now().Month()
	if year != time.Now().Year() {
		month = time.Now().Month()
	}

	return fmt.Sprintf("REG-%d%02d-%05d", year, month, seq), nil
}
