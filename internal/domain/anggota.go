package domain

import "time"

// AnggotaListItem adalah proyeksi antrean/daftar anggota untuk admin:
// tanpa NIK, tanpa kontak sensitif, tanpa object key. Detail lengkap
// (termasuk kontak) hanya di endpoint :id dalam jurisdiction aktor.
type AnggotaListItem struct {
	ID            int       `db:"id" json:"id"`
	NIA           string    `db:"nia" json:"nia"`
	NamaLengkap   string    `db:"nama_lengkap" json:"nama_lengkap"`
	Status        string    `db:"status" json:"status"`
	Pekerjaan     string    `db:"pekerjaan" json:"pekerjaan"`
	Riwayat       string    `db:"-" json:"riwayat"` // dihitung server-side (TDD §5.5)
	ProvinsiID    int       `db:"provinsi_id" json:"provinsi_id"`
	ProvinsiNama  string    `db:"provinsi_nama" json:"provinsi_nama"`
	KabupatenID   int       `db:"kabupaten_id" json:"kabupaten_id"`
	KabupatenNama string    `db:"kabupaten_nama" json:"kabupaten_nama"`
	TanggalAngkat time.Time `db:"tanggal_angkat" json:"tanggal_angkat"`
	CreatedAt     time.Time `db:"created_at" json:"created_at"`
}

// AnggotaPublicInfo adalah DTO publik MINIMAL pengganti cek-anggota lama.
// Whitelist (keputusan pemilik): hanya nama pendaftar, wilayah, dan status.
// TIDAK memuat NIK, tanggal angkat, alamat, kontak, maupun object key.
// Pencarian hanya by NIA (pencarian by NIK mentah dari publik adalah oracle
// PII — ditolak); NIA dikembalikan sebagai echo identifier pencarian.
type AnggotaPublicInfo struct {
	NIA           string `json:"nia"`
	NamaLengkap   string `json:"nama_lengkap"`
	Status        string `json:"status"`
	ProvinsiNama  string `json:"provinsi_nama"`
	KabupatenNama string `json:"kabupaten_nama"`
}
