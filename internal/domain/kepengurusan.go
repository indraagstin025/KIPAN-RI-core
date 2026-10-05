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
// TanggalMulai opsional (default = tanggal terbit SK).
type AddPengurusRequest struct {
	AnggotaID    int        `json:"anggota_id"`
	JabatanID    int        `json:"jabatan_id"`
	Konfirmasi   bool       `json:"konfirmasi"`
	TanggalMulai *time.Time `json:"tanggal_mulai"`
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

// PengurusDetailResponse adalah detail lengkap satu pengurus: baris kepengurusan
// (jabatan+SK+wilayah) + biodata anggota tertaut + riwayat kepengurusannya.
type PengurusDetailResponse struct {
	Pengurus PengurusDetail   `json:"pengurus"`
	Anggota  *Anggota         `json:"anggota,omitempty"`
	Riwayat  []PengurusDetail `json:"riwayat"`
}

// PengurusStats ringkasan jumlah pengurus aktif (ter-scope) untuk kartu dasbor.
type PengurusStats struct {
	Total        int `db:"total" json:"total"`
	Nasional     int `db:"nasional" json:"nasional"`
	Provinsi     int `db:"provinsi" json:"provinsi"`
	Kabupaten    int `db:"kabupaten" json:"kabupaten"`
	AkanBerakhir int `db:"akan_berakhir" json:"akan_berakhir"`
}

// PromosiCandidate kandidat promosi pengurus: anggota AKTIF yang punya
// riwayat kepengurusan dan TIDAK sedang aktif menjabat (untuk diangkat ulang).
type PromosiCandidate struct {
	AnggotaID     int     `db:"anggota_id" json:"anggota_id"`
	NIA           string  `db:"nia" json:"nia"`
	NamaLengkap   string  `db:"nama_lengkap" json:"nama_lengkap"`
	ProvinsiID    int     `db:"provinsi_id" json:"provinsi_id"`
	KabupatenID   int     `db:"kabupaten_id" json:"kabupaten_id"`
	ProvinsiNama  *string `db:"provinsi_nama" json:"provinsi_nama,omitempty"`
	KabupatenNama *string `db:"kabupaten_nama" json:"kabupaten_nama,omitempty"`
	Jabatan       string  `db:"jabatan" json:"jabatan"`
	Level         string  `db:"level" json:"level"`
	Status        string  `db:"status" json:"status"`
}

// JabatanRequest payload kelola master jabatan. Tanpa level (tingkat dari SK).
type JabatanRequest struct {
	Nama        string `json:"nama"`
	IsKetuaUmum bool   `json:"is_ketua_umum"`
	IsInti      bool   `json:"is_inti"`
	IsActive    bool   `json:"is_active"`
	Urutan      int    `json:"urutan"`
}

// UpdateJabatanRequest payload ganti jabatan pengurus.
type UpdateJabatanRequest struct {
	JabatanID int `json:"jabatan_id"`
}

// PengurusPAWAction aksi pengakhiran masa bakti individual (PAW) — TDD §5.6.
type PengurusPAWAction string

const (
	PAWDemisioner       PengurusPAWAction = "DEMISIONER"        // purna tugas lebih awal
	PAWDiberhentikan    PengurusPAWAction = "DIBERHENTIKAN"     // sanksi pelanggaran AD/ART
	PAWMengundurkanDiri PengurusPAWAction = "MENGUNDURKAN_DIRI" // atas surat resmi
	PAWMeninggal        PengurusPAWAction = "MENINGGAL"         // purna tugas permanen
)

// MapPAWAction memetakan aksi PAW ke status pengurus. ok=false bila tidak dikenal.
func MapPAWAction(a PengurusPAWAction) (PengurusStatus, bool) {
	switch a {
	case PAWDemisioner:
		return PengurusStatusDemisioner, true
	case PAWDiberhentikan:
		return PengurusStatusDiberhentikan, true
	case PAWMengundurkanDiri:
		return PengurusStatusMengundurkanDiri, true
	case PAWMeninggal:
		return PengurusStatusMeninggal, true
	}
	return "", false
}

// PawsRequest payload PAW (pengakhiran individual).
type PawsRequest struct {
	Aksi       string `json:"aksi"`
	Keterangan string `json:"keterangan"`
}

// MutasiRequest payload mutasi jabatan/wilayah aktif (TDD D3): tutup record
// lama sebagai Demisioner, buka record baru pada SK/jabatan tujuan.
type MutasiRequest struct {
	SKID         int        `json:"sk_id"`
	JabatanID    int        `json:"jabatan_id"`
	TanggalMulai *time.Time `json:"tanggal_mulai"`
	Keterangan   string     `json:"keterangan"`
}

// ExpiredAppointment adalah pengurus yang ditutup otomatis karena masa bakti
// SK berakhir (hasil job materialisasi kedaluwarsa dinamis, TDD §5.4).
type ExpiredAppointment struct {
	ID               int     `db:"id" json:"id"`
	AnggotaID        int     `db:"anggota_id" json:"anggota_id"`
	SuratKeputusanID int     `db:"surat_keputusan_id" json:"surat_keputusan_id"`
	NomorSK          string  `db:"nomor_sk" json:"nomor_sk"`
	Jabatan          string  `db:"jabatan" json:"jabatan"`
	NamaLengkap      string  `db:"nama_lengkap" json:"nama_lengkap"`
	Level            string  `db:"level" json:"level"`
	ProvinsiID       *int    `db:"provinsi_id" json:"provinsi_id,omitempty"`
	KabupatenID      *int    `db:"kabupaten_id" json:"kabupaten_id,omitempty"`
	UserID           *string `db:"user_id" json:"user_id,omitempty"`
}

// ExpiringAppointment adalah pengurus Aktif yang masa baktinya berakhir
// dalam maxDays hari (sumber notifikasi peringatan H-30/H-7).
type ExpiringAppointment struct {
	ID              int        `db:"id" json:"id"`
	AnggotaID       int        `db:"anggota_id" json:"anggota_id"`
	NamaLengkap     string     `db:"nama_lengkap" json:"nama_lengkap"`
	Jabatan         string     `db:"jabatan" json:"jabatan"`
	NomorSK         string     `db:"nomor_sk" json:"nomor_sk"`
	TanggalBerakhir time.Time  `db:"tanggal_berakhir" json:"tanggal_berakhir"`
	Level           string     `db:"level" json:"level"`
	ProvinsiID      *int       `db:"provinsi_id" json:"provinsi_id,omitempty"`
	KabupatenID     *int       `db:"kabupaten_id" json:"kabupaten_id,omitempty"`
	NotifiedH30At   *time.Time `db:"notified_h30_at" json:"notified_h30_at,omitempty"`
	NotifiedH7At    *time.Time `db:"notified_h7_at" json:"notified_h7_at,omitempty"`
}
