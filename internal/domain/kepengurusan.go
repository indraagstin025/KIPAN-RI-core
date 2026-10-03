package domain

import "time"

// ============================================================
// KEPENGURUSAN: SK, JABATAN, PENGURUS
// ============================================================

// SKApprovalAction aksi pada rantai persetujuan SK.
type SKApprovalAction string

const (
	// SKActionAjukan: pengelola SK mengajukan DRAFT ke tahap berikutnya
	// (KABUPATEN -> MENUNGGU_PROVINSI, PROVINSI -> MENUNGGU_NASIONAL,
	// NASIONAL -> DISETUJUI/final).
	SKActionAjukan SKApprovalAction = "AJUKAN"
	// SKActionTeruskan: Admin Provinsi meneruskan MENUNGGU_PROVINSI -> MENUNGGU_NASIONAL.
	SKActionTeruskan SKApprovalAction = "TERUSKAN"
	// SKActionSahkan: Nasional/Super mengesahkan MENUNGGU_NASIONAL -> DISETUJUI (final).
	SKActionSahkan SKApprovalAction = "SAHKAN"
	// SKActionTolak: Nasional/Super menolak MENUNGGU_NASIONAL -> DITOLAK.
	SKActionTolak SKApprovalAction = "TOLAK"
)

// SKCreateRequest payload pembuatan SK. Level & wilayah hanya dihormati untuk
// Super/Nasional; Provinsi/Kabupaten dipaksa ke wilayahnya oleh server.
type SKCreateRequest struct {
	NomorSK         string    `json:"nomor_sk"`
	Judul           string    `json:"judul"`
	Level           string    `json:"level"`
	ProvinsiID      *int      `json:"provinsi_id"`
	KabupatenID     *int      `json:"kabupaten_id"`
	TanggalTerbit   time.Time `json:"tanggal_terbit"`
	TanggalBerakhir time.Time `json:"tanggal_berakhir"`
	FileSKKey       string    `json:"file_sk_key"`
}

// SKListItem proyeksi daftar SK (non-PII) untuk tabel admin.
type SKListItem struct {
	ID              int        `db:"id" json:"id"`
	NomorSK         string     `db:"nomor_sk" json:"nomor_sk"`
	Judul           string     `db:"judul" json:"judul"`
	Level           string     `db:"level" json:"level"`
	ProvinsiID      *int       `db:"provinsi_id" json:"provinsi_id,omitempty"`
	KabupatenID     *int       `db:"kabupaten_id" json:"kabupaten_id,omitempty"`
	ProvinsiNama    *string    `db:"provinsi_nama" json:"provinsi_nama,omitempty"`
	KabupatenNama   *string    `db:"kabupaten_nama" json:"kabupaten_nama,omitempty"`
	TanggalTerbit   time.Time  `db:"tanggal_terbit" json:"tanggal_terbit"`
	TanggalBerakhir *time.Time `db:"tanggal_berakhir" json:"tanggal_berakhir,omitempty"`
	Status          string     `db:"status" json:"status"`
	ApprovalStatus  string     `db:"approval_status" json:"approval_status"`
	JumlahPengurus  int        `db:"jumlah_pengurus" json:"jumlah_pengurus"`
	CreatedAt       time.Time  `db:"created_at" json:"created_at"`
}

// AddPengurusRequest payload pengangkatan anggota ke sebuah SK.
type AddPengurusRequest struct {
	AnggotaID  int  `json:"anggota_id"`
	JabatanID  int  `json:"jabatan_id"`
	Konfirmasi bool `json:"konfirmasi"`
}

// PengurusDetail proyeksi pengurus (join anggota + jabatan + SK) untuk
// tabel admin dan panel SK.
type PengurusDetail struct {
	ID                int        `db:"id" json:"id"`
	AnggotaID         int        `db:"anggota_id" json:"anggota_id"`
	NIA               string     `db:"nia" json:"nia"`
	NamaLengkap       string     `db:"nama_lengkap" json:"nama_lengkap"`
	SuratKeputusanID  int        `db:"surat_keputusan_id" json:"surat_keputusan_id"`
	NomorSK           string     `db:"nomor_sk" json:"nomor_sk"`
	JabatanID         int        `db:"jabatan_id" json:"jabatan_id"`
	Jabatan           string     `db:"jabatan" json:"jabatan"`
	IsInti            bool       `db:"is_inti" json:"is_inti"`
	Level             string     `db:"level" json:"level"`
	ProvinsiID        *int       `db:"provinsi_id" json:"provinsi_id,omitempty"`
	KabupatenID       *int       `db:"kabupaten_id" json:"kabupaten_id,omitempty"`
	ProvinsiNama      *string    `db:"provinsi_nama" json:"provinsi_nama,omitempty"`
	KabupatenNama     *string    `db:"kabupaten_nama" json:"kabupaten_nama,omitempty"`
	Status            string     `db:"status" json:"status"`
	KeteranganStatus  *string    `db:"keterangan_status" json:"keterangan_status,omitempty"`
	SKTanggalBerakhir *time.Time `db:"sk_tanggal_berakhir" json:"sk_tanggal_berakhir,omitempty"`
	TanggalMulai      time.Time  `db:"tanggal_mulai" json:"tanggal_mulai"`
	TanggalSelesai    *time.Time `db:"tanggal_selesai" json:"tanggal_selesai,omitempty"`
	CreatedAt         time.Time  `db:"created_at" json:"created_at"`
}

// PengurusStats ringkasan jumlah pengurus aktif (ter-scope) untuk kartu dasbor.
type PengurusStats struct {
	Total        int `db:"total" json:"total"`
	Nasional     int `db:"nasional" json:"nasional"`
	Provinsi     int `db:"provinsi" json:"provinsi"`
	Kabupaten    int `db:"kabupaten" json:"kabupaten"`
	AkanBerakhir int `db:"akan_berakhir" json:"akan_berakhir"`
}

// JabatanRequest payload kelola master jabatan (Super/Nasional).
type JabatanRequest struct {
	Nama     string `json:"nama"`
	Level    string `json:"level"`
	IsInti   bool   `json:"is_inti"`
	IsActive bool   `json:"is_active"`
	Urutan   int    `json:"urutan"`
}

// UpdateJabatanRequest payload ganti jabatan pengurus.
type UpdateJabatanRequest struct {
	JabatanID int `json:"jabatan_id"`
}
