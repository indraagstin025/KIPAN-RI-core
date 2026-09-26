package nia

import "fmt"

// GenerateNIA menghasilkan nomor induk anggota dengan format:
// KIPAN.{provinsi}.{kabupaten}.{tahun}.{seq}
func GenerateNIA(provinsiID, kabupatenID, tahun, seq int) (string, error) {
	if provinsiID <= 0 {
		return "", fmt.Errorf("provinsi tidak valid: %d", provinsiID)
	}
	if kabupatenID <= 0 {
		return "", fmt.Errorf("kabupaten tidak valid: %d", kabupatenID)
	}
	if tahun <= 0 {
		return "", fmt.Errorf("tahun tidak valid: %d", tahun)
	}
	if seq < 0 {
		return "", fmt.Errorf("sequence tidak valid: %d", seq)
	}

	return fmt.Sprintf("KIPAN.%02d.%d.%d.%04d", provinsiID, kabupatenID, tahun, seq), nil
}
