package domain

import "time"

// PendaftaranSubmitRequest adalah payload masuk untuk pendaftaran calon anggota.
// Semua field bersifat server-side validated agar tidak mempercayai data klien secara mentah.
type PendaftaranSubmitRequest struct {
	NamaLengkap        string `json:"nama_lengkap"`
	NIK                string `json:"nik"`
	TempatLahir        string `json:"tempat_lahir"`
	TanggalLahir       string `json:"tanggal_lahir"`
	JenisKelamin       string `json:"jenis_kelamin"`
	Agama              string `json:"agama,omitempty"`
	Pendidikan         string `json:"pendidikan,omitempty"`
	Pekerjaan          string `json:"pekerjaan,omitempty"`
	StatusPribadi      string `json:"status_pribadi,omitempty"`
	Alamat             string `json:"alamat"`
	ProvinsiID         int    `json:"provinsi_id"`
	KabupatenID        int    `json:"kabupaten_id"`
	Kecamatan          string `json:"kecamatan,omitempty"`
	Desa               string `json:"desa,omitempty"`
	KodePos            string `json:"kode_pos,omitempty"`
	Email              string `json:"email"`
	Whatsapp           string `json:"whatsapp"`
	Motivasi           string `json:"motivation,omitempty"`
	FotoKey            string `json:"foto_key,omitempty"`
	KTPKey             string `json:"ktp_key,omitempty"`
	CVKey              string `json:"cv_key,omitempty"`
	SKKey              string `json:"sk_key,omitempty"`
	SuratPernyataanKey string `json:"surat_pernyataan_key,omitempty"`
	SuratSehatKey      string `json:"surat_sehat_key,omitempty"`
}

// PendaftaranApprovalAction action yang diizinkan untuk admin saat memproses pendaftaran.
type PendaftaranApprovalAction string

const (
	PendaftaranActionVerifikasi PendaftaranApprovalAction = "VERIFIKASI"
	PendaftaranActionPerbaikan  PendaftaranApprovalAction = "PERBAIKAN"
	PendaftaranActionTolak      PendaftaranApprovalAction = "TOLAK"
	PendaftaranActionSetujui    PendaftaranApprovalAction = "SETUJUI"
)

// PendaftaranStatusTransition validasi transisi status yang sah.
type PendaftaranStatusTransition struct {
	From   PendaftaranStatus
	To     PendaftaranStatus
	Action PendaftaranApprovalAction
}

// PendaftaranStatusTransitionRules daftar state machine yang boleh terjadi.
var PendaftaranStatusTransitionRules = []PendaftaranStatusTransition{
	{From: PendaftaranStatusDiajukan, To: PendaftaranStatusDiverifikasi, Action: PendaftaranActionVerifikasi},
	{From: PendaftaranStatusDiverifikasi, To: PendaftaranStatusPerbaikan, Action: PendaftaranActionPerbaikan},
	{From: PendaftaranStatusDiverifikasi, To: PendaftaranStatusDitolak, Action: PendaftaranActionTolak},
	{From: PendaftaranStatusDiverifikasi, To: PendaftaranStatusDisetujui, Action: PendaftaranActionSetujui},
	{From: PendaftaranStatusPerbaikan, To: PendaftaranStatusDiverifikasi, Action: PendaftaranActionVerifikasi},
}

// IsAllowedTransition mengembalikan true jika transisi status dan aksi yang diminta sah.
func IsAllowedTransition(from PendaftaranStatus, to PendaftaranStatus, action PendaftaranApprovalAction) bool {
	for _, rule := range PendaftaranStatusTransitionRules {
		if rule.From == from && rule.To == to && rule.Action == action {
			return true
		}
	}
	return false
}

// PendaftaranCreateResult digunakan saat pendaftaran baru disimpan.
type PendaftaranCreateResult struct {
	ID               int    `json:"id"`
	NomorPendaftaran string `json:"nomor_pendaftaran"`
	Status           string `json:"status"`
}

// PendaftaranTrackingResponse adalah DTO publik minimal untuk pelacakan
// mandiri: tanpa nama, kontak, alamat, maupun object key (RULES 12).
type PendaftaranTrackingResponse struct {
	NomorPendaftaran string    `json:"nomor_pendaftaran"`
	Status           string    `json:"status"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// KTAVerificationResponse adalah DTO publik verifikasi KTA: hanya data yang
// memang boleh tampil ke publik + verdict. Tanpa kontak, NIK, maupun key.
type KTAVerificationResponse struct {
	NIA           string     `json:"nia"`
	Valid         bool       `json:"valid"`
	NamaLengkap   string     `json:"nama_lengkap,omitempty"`
	Status        string     `json:"status,omitempty"`
	TanggalAngkat *time.Time `json:"tanggal_angkat,omitempty"`
}

// PendaftaranDetailResponse response detail pendaftaran untuk admin/public.
type PendaftaranDetailResponse struct {
	ID               int       `json:"id"`
	NomorPendaftaran string    `json:"nomor_pendaftaran"`
	NamaLengkap      string    `json:"nama_lengkap"`
	TempatLahir      string    `json:"tempat_lahir"`
	TanggalLahir     time.Time `json:"tanggal_lahir"`
	JenisKelamin     string    `json:"jenis_kelamin"`
	Alamat           string    `json:"alamat"`
	ProvinsiID       int       `json:"provinsi_id"`
	KabupatenID      int       `json:"kabupaten_id"`
	Email            string    `json:"email"`
	Whatsapp         string    `json:"whatsapp"`
	Status           string    `json:"status"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}
