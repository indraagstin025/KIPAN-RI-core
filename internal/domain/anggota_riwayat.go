package domain

import "time"

// AnggotaRiwayatItem adalah satu entri timeline riwayat anggota (gabungan
// perjalanan pendaftaran + kepengurusan).
type AnggotaRiwayatItem struct {
	Waktu      time.Time `json:"waktu"`
	Sumber     string    `json:"sumber"` // PENDAFTARAN | KEPENGURUSAN
	Aksi       string    `json:"aksi"`
	Label      string    `json:"label"`
	Oleh       string    `json:"oleh"`
	Keterangan string    `json:"keterangan,omitempty"`
}
