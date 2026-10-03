package domain

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTClaims adalah payload standar access token SIM-KIPAN.
// Didefinisikan di domain (L-3) agar service/handler tidak bergantung pada
// paket middleware (dependency direction: ke dalam).
// Field ProvinsiID/KabupatenID menggunakan *int agar konsisten dengan
// domain.User dan bisa langsung dikonsumsi repository tanpa konversi.
type JWTClaims struct {
	UserID      string `json:"user_id"`
	Email       string `json:"email"`
	Name        string `json:"name,omitempty"`
	Role        Role   `json:"role"`
	TipeUser    string `json:"tipe_user,omitempty"`
	ProvinsiID  *int   `json:"provinsi_id,omitempty"`
	KabupatenID *int   `json:"kabupaten_id,omitempty"`
	jwt.RegisteredClaims
}

// ============================================================
// ENUMERASI & TIPE DATA KHUSUS (DOMAIN ENUMS)
// Sesuai ERD_DAN_SKEMA_DATABASE_CORE_KIPAN.md
// ============================================================

// Role tingkat wewenang admin SIM-KIPAN
type Role string

const (
	RoleSuperAdmin     Role = "SUPER_ADMIN"
	RoleAdminNasional  Role = "ADMIN_NASIONAL"
	RoleAdminProvinsi  Role = "ADMIN_PROVINSI"
	RoleAdminKabupaten Role = "ADMIN_KABUPATEN"
	// RoleUser adalah akun anggota (bukan admin): dibuat otomatis saat
	// pendaftaran DISETUJUI. Tanpa akses /admin/* (deny-by-default di
	// RequireRoles + ScopeWilayah). Pembedaan Kader vs Pengurus memakai
	// UserTipe, bukan role terpisah.
	RoleUser Role = "USER"
)

func (r Role) IsValid() bool {
	switch r {
	case RoleSuperAdmin, RoleAdminNasional, RoleAdminProvinsi, RoleAdminKabupaten, RoleUser:
		return true
	}
	return false
}

// UserTipe membedakan jenis akun: KADER (default saat approve) / PENGURUS
// (diangkat via SK) untuk role USER, dan ADMIN untuk akun admin (role<>USER).
type UserTipe string

const (
	UserTipeKader    UserTipe = "KADER"
	UserTipePengurus UserTipe = "PENGURUS"
	UserTipeAdmin    UserTipe = "ADMIN"
)

// UserStatus status akun admin
type UserStatus string

const (
	UserStatusAktif     UserStatus = "Aktif"
	UserStatusNonaktif  UserStatus = "Nonaktif"
	UserStatusSuspended UserStatus = "Suspended"
)

// PendaftaranStatus status verifikasi pendaftaran calon anggota
type PendaftaranStatus string

const (
	PendaftaranStatusDraft        PendaftaranStatus = "DRAFT"
	PendaftaranStatusDiverifikasi PendaftaranStatus = "DIVERIFIKASI"
	PendaftaranStatusPerbaikan    PendaftaranStatus = "PERBAIKAN"
	PendaftaranStatusDisetujui    PendaftaranStatus = "DISETUJUI"
	PendaftaranStatusDitolak      PendaftaranStatus = "DITOLAK"
	PendaftaranStatusKedaluwarsa  PendaftaranStatus = "KEDALUWARSA"
)

// AnggotaStatus status resmi kader KIPAN
type AnggotaStatus string

const (
	AnggotaStatusAktif         AnggotaStatus = "AKTIF"
	AnggotaStatusNonaktif      AnggotaStatus = "NONAKTIF"
	AnggotaStatusDemisioner    AnggotaStatus = "DEMISIONER"
	AnggotaStatusDiberhentikan AnggotaStatus = "DIBERHENTIKAN"
	AnggotaStatusMeninggal     AnggotaStatus = "MENINGGAL"
)

// SKStatus status Surat Keputusan
type SKStatus string

const (
	SKStatusAktif      SKStatus = "Aktif"
	SKStatusTidakAktif SKStatus = "TidakAktif"
	SKStatusDigantikan SKStatus = "Digantikan"
)

// SKApprovalStatus tahapan persetujuan SK berjenjang
type SKApprovalStatus string

const (
	SKApprovalStatusDraft            SKApprovalStatus = "DRAFT"
	SKApprovalStatusMenungguProvinsi SKApprovalStatus = "MENUNGGU_PROVINSI"
	SKApprovalStatusMenungguNasional SKApprovalStatus = "MENUNGGU_NASIONAL"
	SKApprovalStatusDisetujui        SKApprovalStatus = "DISETUJUI"
	SKApprovalStatusDitolak          SKApprovalStatus = "DITOLAK"
)

// TingkatWilayah level hirarki organisasi
type TingkatWilayah string

const (
	LevelNasional  TingkatWilayah = "NASIONAL"
	LevelProvinsi  TingkatWilayah = "PROVINSI"
	LevelKabupaten TingkatWilayah = "KABUPATEN"
)

// PengurusStatus status keaktifan menjabat
type PengurusStatus string

const (
	PengurusStatusAktif            PengurusStatus = "Aktif"
	PengurusStatusDemisioner       PengurusStatus = "Demisioner"
	PengurusStatusDiberhentikan    PengurusStatus = "Diberhentikan"
	PengurusStatusMengundurkanDiri PengurusStatus = "Mengundurkan Diri"
	PengurusStatusMeninggal        PengurusStatus = "Meninggal"
)

// NotificationType jenis notifikasi in-app
type NotificationType string

const (
	NotifTypePendaftaran NotificationType = "PENDAFTARAN"
	NotifTypeVerifikasi  NotificationType = "VERIFIKASI"
	NotifTypeSK          NotificationType = "SK"
	NotifTypeSistem      NotificationType = "SISTEM"
)

// ============================================================
// 1. MASTER WILAYAH & STRUKTURAL (BPS & JABATAN)
// ============================================================

// WilayahProvinsi referensi 38 provinsi Indonesia (BPS)
type WilayahProvinsi struct {
	ID        int       `db:"id" json:"id"`
	Kode      string    `db:"kode" json:"kode"` // Kode BPS 2 digit, e.g. "32"
	Nama      string    `db:"nama" json:"nama"`
	IsActive  bool      `db:"is_active" json:"is_active"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// WilayahKabupaten referensi 514 kabupaten/kota (BPS)
type WilayahKabupaten struct {
	ID         int       `db:"id" json:"id"`
	ProvinsiID int       `db:"provinsi_id" json:"provinsi_id"`
	Kode       string    `db:"kode" json:"kode"` // Kode BPS 4 digit, e.g. "3273"
	Nama       string    `db:"nama" json:"nama"`
	IsActive   bool      `db:"is_active" json:"is_active"`
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
	UpdatedAt  time.Time `db:"updated_at" json:"updated_at"`
}

// WilayahKecamatan adalah entri kecamatan dari upstream wilayah.id.
// TIDAK disimpan di DB (submit kecamatan tetap teks bebas); hanya bantuan
// dropdown. Kode dinormalisasi tanpa titik agar konsisten dengan kolom kode.
type WilayahKecamatan struct {
	Kode string `json:"kode"` // 6 digit tanpa titik, e.g. "327301"
	Nama string `json:"nama"`
}

// WilayahDesa adalah entri desa/kelurahan dari upstream wilayah.id.
// Sama seperti kecamatan: tidak disimpan, hanya saran dropdown.
type WilayahDesa struct {
	Kode string `json:"kode"` // 10 digit tanpa titik, e.g. "3273081001"
	Nama string `json:"nama"`
}

// WilayahKodepos adalah satu opsi kode pos hasil pencocokan.
// Dropdown hanya menampilkan kode (tanpa nama).
type WilayahKodepos struct {
	KodePos string `json:"kode_pos"` // 5 digit, e.g. "40559"
}

// Jabatan referensi jabatan struktural pengurus KIPAN (master terkonfigurasi).
// Jabatan inti (is_inti) hanya boleh satu pemegang per SK. Jabatan berlevel
// (NASIONAL/PROVINSI/KABUPATEN); nama boleh sama di tiap level.
type Jabatan struct {
	ID        int            `db:"id" json:"id"`
	Nama      string         `db:"nama" json:"nama"` // Ketua, Sekretaris, Bendahara, dst
	Level     TingkatWilayah `db:"level" json:"level"`
	IsInti    bool           `db:"is_inti" json:"is_inti"`
	IsActive  bool           `db:"is_active" json:"is_active"`
	Urutan    int            `db:"urutan" json:"urutan"`
	CreatedAt time.Time      `db:"created_at" json:"created_at"`
	UpdatedAt time.Time      `db:"updated_at" json:"updated_at"`
}

// ============================================================
// 2. AUTH, USERS & REFRESH TOKENS
// Soft Delete: deleted_at + Partial Unique Index pada email
// ============================================================

// User entitas admin & verifikator SIM-KIPAN
type User struct {
	ID            string     `db:"id" json:"id"`
	Email         string     `db:"email" json:"email"`
	PasswordHash  string     `db:"password_hash" json:"-"` // Argon2id hash
	Name          string     `db:"name" json:"name"`
	Role          Role       `db:"role" json:"role"`
	TipeUser      UserTipe   `db:"tipe_user" json:"tipe_user,omitempty"`
	Status        UserStatus `db:"status" json:"status"`
	AvatarURL     *string    `db:"avatar_url" json:"avatar_url,omitempty"`
	ProvinsiID    *int       `db:"provinsi_id" json:"provinsi_id,omitempty"`
	ProvinsiNama  *string    `db:"provinsi_nama" json:"provinsi_nama,omitempty"`
	KabupatenID   *int       `db:"kabupaten_id" json:"kabupaten_id,omitempty"`
	KabupatenNama *string    `db:"kabupaten_nama" json:"kabupaten_nama,omitempty"`
	LastLoginAt   *time.Time `db:"last_login_at" json:"last_login_at,omitempty"`
	CreatedAt     time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt     *time.Time `db:"deleted_at" json:"deleted_at,omitempty"` // Soft delete
}

// UserRefreshToken penyimpanan token refresh untuk rotasi sesi & deteksi pencurian token
type UserRefreshToken struct {
	ID        string    `db:"id" json:"id"`
	UserID    string    `db:"user_id" json:"user_id"`
	TokenHash string    `db:"token_hash" json:"-"` // SHA-256 dari token raw
	FamilyID  string    `db:"family_id" json:"family_id"`
	IsRevoked bool      `db:"is_revoked" json:"is_revoked"`
	ExpiresAt time.Time `db:"expires_at" json:"expires_at"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// ============================================================
// 3. PENDAFTARAN CALON ANGGOTA & JEJAK AUDIT
// Status-based (Data historis tidak pernah dihapus)
// ============================================================

// Pendaftaran formulir registrasi calon kader
type Pendaftaran struct {
	ID               int       `db:"id" json:"id"`
	NomorPendaftaran string    `db:"nomor_pendaftaran" json:"nomor_pendaftaran"` // REG-YYYYMM-XXXXX
	NamaLengkap      string    `db:"nama_lengkap" json:"nama_lengkap"`
	NIKHash          string    `db:"nik_hash" json:"-"`      // HMAC-SHA256 Blind Index
	NIKEncrypted     string    `db:"nik_encrypted" json:"-"` // AES-256-GCM
	TempatLahir      string    `db:"tempat_lahir" json:"tempat_lahir"`
	TanggalLahir     time.Time `db:"tanggal_lahir" json:"tanggal_lahir"`
	JenisKelamin     string    `db:"jenis_kelamin" json:"jenis_kelamin"` // L | P
	Agama            string    `db:"agama" json:"agama"`
	Pendidikan       string    `db:"pendidikan" json:"pendidikan"`
	Pekerjaan        string    `db:"pekerjaan" json:"pekerjaan"`
	StatusPribadi    string    `db:"status_pribadi" json:"status_pribadi"`
	Alamat           string    `db:"alamat" json:"alamat"`
	ProvinsiID       int       `db:"provinsi_id" json:"provinsi_id"`
	KabupatenID      int       `db:"kabupaten_id" json:"kabupaten_id"`
	Kecamatan        string    `db:"kecamatan" json:"kecamatan"`
	Desa             string    `db:"desa" json:"desa"`
	KodePos          string    `db:"kode_pos" json:"kode_pos"`
	Email            string    `db:"email" json:"email"`
	Whatsapp         string    `db:"whatsapp" json:"whatsapp"`
	Motivasi         string    `db:"motivasi" json:"motivasi"`
	// PersyaratanChecklist menyimpan pilihan checklist pendaftar sebagai
	// JSON array string (selaras form KIPAN_INDONESIA).
	PersyaratanChecklist string            `db:"persyaratan_checklist" json:"persyaratan_checklist"`
	FotoKey              string            `db:"foto_key" json:"foto_key"`
	KTPKey               string            `db:"ktp_key" json:"ktp_key"` // PRIVATE bucket
	CVKey                string            `db:"cv_key" json:"cv_key"`
	SKKey                string            `db:"sk_key" json:"sk_key"`
	SuratPernyataanKey   string            `db:"surat_pernyataan_key" json:"surat_pernyataan_key"`
	SuratSehatKey        string            `db:"surat_sehat_key" json:"surat_sehat_key"` // PRIVATE bucket
	Status               PendaftaranStatus `db:"status" json:"status"`
	Tipe                 TipePendaftaran   `db:"tipe_pendaftaran" json:"tipe_pendaftaran"`
	CatatanPerbaikan     *string           `db:"catatan_perbaikan" json:"catatan_perbaikan,omitempty"`
	RevisiTokenHash      *string           `db:"revisi_token_hash" json:"-"`
	RevisiTokenExpiresAt *time.Time        `db:"revisi_token_expires_at" json:"-"`
	AnggotaID            *int              `db:"anggota_id" json:"anggota_id,omitempty"` // Relasi saat DISETUJUI
	CreatedAt            time.Time         `db:"created_at" json:"created_at"`
	UpdatedAt            time.Time         `db:"updated_at" json:"updated_at"`
}

// PendaftaranRiwayat jejak kronologis pergantian status pendaftaran
type PendaftaranRiwayat struct {
	ID            int       `db:"id" json:"id"`
	PendaftaranID int       `db:"pendaftaran_id" json:"pendaftaran_id"`
	Aksi          string    `db:"aksi" json:"aksi"` // SUBMIT | VERIFY | REQUEST_REVISION | APPROVE | REJECT
	ActorID       *string   `db:"actor_id" json:"actor_id,omitempty"`
	ActorName     *string   `db:"actor_name" json:"actor_name,omitempty"`
	ActorRole     *string   `db:"actor_role" json:"actor_role,omitempty"`
	Catatan       *string   `db:"catatan" json:"catatan,omitempty"`
	CreatedAt     time.Time `db:"created_at" json:"created_at"`
}

// ============================================================
// 4. ANGGOTA RESMI (KADER KIPAN BER-NIA)
// Status-based: AKTIF | NONAKTIF | DEMISIONER | DIBERHENTIKAN | MENINGGAL
// ============================================================

// Anggota data kader resmi ber-Nomor Induk Anggota (NIA)
type Anggota struct {
	ID                 int             `db:"id" json:"id"`
	NIA                string          `db:"nia" json:"nia"` // KIPAN-IND-{kab}-{tahun}-{seq}
	NamaLengkap        string          `db:"nama_lengkap" json:"nama_lengkap"`
	NIKHash            string          `db:"nik_hash" json:"-"`      // HMAC Blind Index UNIQUE
	NIKEncrypted       string          `db:"nik_encrypted" json:"-"` // AES-256-GCM
	TempatLahir        string          `db:"tempat_lahir" json:"tempat_lahir"`
	TanggalLahir       time.Time       `db:"tanggal_lahir" json:"tanggal_lahir"`
	JenisKelamin       string          `db:"jenis_kelamin" json:"jenis_kelamin"`
	Agama              string          `db:"agama" json:"agama"`
	Pendidikan         string          `db:"pendidikan" json:"pendidikan"`
	Pekerjaan          string          `db:"pekerjaan" json:"pekerjaan"`
	Alamat             string          `db:"alamat" json:"alamat"`
	ProvinsiID         int             `db:"provinsi_id" json:"provinsi_id"`
	KabupatenID        int             `db:"kabupaten_id" json:"kabupaten_id"`
	Kecamatan          string          `db:"kecamatan" json:"kecamatan"`
	Desa               string          `db:"desa" json:"desa"`
	KodePos            string          `db:"kode_pos" json:"kode_pos"`
	Email              string          `db:"email" json:"email"`
	Whatsapp           string          `db:"whatsapp" json:"whatsapp"`
	FotoKey            string          `db:"foto_key" json:"foto_key"` // PUBLIC bucket
	KTPKey             string          `db:"ktp_key" json:"ktp_key"`   // PRIVATE bucket
	CVKey              string          `db:"cv_key" json:"cv_key"`
	SKKey              string          `db:"sk_key" json:"sk_key"`
	SuratPernyataanKey string          `db:"surat_pernyataan_key" json:"surat_pernyataan_key"`
	SuratSehatKey      string          `db:"surat_sehat_key" json:"surat_sehat_key"`
	Status             AnggotaStatus   `db:"status" json:"status"`
	Tipe               TipePendaftaran `db:"tipe" json:"tipe"`
	Angkatan           string          `db:"angkatan" json:"angkatan"`
	KTAQRHash          *string         `db:"kta_qr_hash" json:"kta_qr_hash,omitempty"` // HMAC digital signature anti-palsu
	KTAPDFKey          *string         `db:"kta_pdf_key" json:"kta_pdf_key,omitempty"`
	PendaftaranID      *int            `db:"pendaftaran_id" json:"pendaftaran_id,omitempty"`
	UserID             *string         `db:"user_id" json:"user_id,omitempty"` // Relasi ke akun admin (jika ada)
	TanggalDaftar      time.Time       `db:"tanggal_daftar" json:"tanggal_daftar"`
	TanggalAngkat      time.Time       `db:"tanggal_angkat" json:"tanggal_angkat"`
	CreatedAt          time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt          time.Time       `db:"updated_at" json:"updated_at"`
}

// KTAQRHashValue mengembalikan signature QR ("" bila belum terbit).
func (a *Anggota) KTAQRHashValue() string {
	if a == nil || a.KTAQRHash == nil {
		return ""
	}
	return *a.KTAQRHash
}

// ============================================================
// 5. SURAT KEPUTUSAN & KEPENGURUSAN
// Tata Kelola Organisasi (DPP, DPD, DPC)
// ============================================================

// SuratKeputusan naskah SK penetapan kepengurusan resmi
type SuratKeputusan struct {
	ID               int              `db:"id" json:"id"`
	NomorSK          string           `db:"nomor_sk" json:"nomor_sk"`
	Judul            string           `db:"judul" json:"judul"`
	Level            TingkatWilayah   `db:"level" json:"level"` // NASIONAL | PROVINSI | KABUPATEN
	ProvinsiID       *int             `db:"provinsi_id" json:"provinsi_id,omitempty"`
	KabupatenID      *int             `db:"kabupaten_id" json:"kabupaten_id,omitempty"`
	TanggalTerbit    time.Time        `db:"tanggal_terbit" json:"tanggal_terbit"`
	TanggalBerakhir  *time.Time       `db:"tanggal_berakhir" json:"tanggal_berakhir,omitempty"`
	FileSKKey        string           `db:"file_sk_key" json:"file_sk_key"` // PRIVATE bucket
	Status           SKStatus         `db:"status" json:"status"`           // Aktif | TidakAktif | Digantikan
	ApprovalStatus   SKApprovalStatus `db:"approval_status" json:"approval_status"`
	CatatanPenolakan *string          `db:"catatan_penolakan" json:"catatan_penolakan,omitempty"`
	CreatedBy        *string          `db:"created_by" json:"created_by,omitempty"`   // User ID pembuat draf
	ApprovedBy       *string          `db:"approved_by" json:"approved_by,omitempty"` // User ID pengesah final
	ApprovedAt       *time.Time       `db:"approved_at" json:"approved_at,omitempty"`
	CreatedAt        time.Time        `db:"created_at" json:"created_at"`
	UpdatedAt        time.Time        `db:"updated_at" json:"updated_at"`
}

// Pengurus susunan pengurus (Anggota + Jabatan + SK)
type Pengurus struct {
	ID               int            `db:"id" json:"id"`
	AnggotaID        int            `db:"anggota_id" json:"anggota_id"`
	SuratKeputusanID int            `db:"surat_keputusan_id" json:"surat_keputusan_id"`
	Level            TingkatWilayah `db:"level" json:"level"`
	ProvinsiID       *int           `db:"provinsi_id" json:"provinsi_id,omitempty"`
	KabupatenID      *int           `db:"kabupaten_id" json:"kabupaten_id,omitempty"`
	JabatanID        int            `db:"jabatan_id" json:"jabatan_id"`
	Status           PengurusStatus `db:"status" json:"status"`
	KeteranganStatus *string        `db:"keterangan_status" json:"keterangan_status,omitempty"`
	TanggalMulai     time.Time      `db:"tanggal_mulai" json:"tanggal_mulai"`
	TanggalSelesai   *time.Time     `db:"tanggal_selesai" json:"tanggal_selesai,omitempty"`
	CreatedAt        time.Time      `db:"created_at" json:"created_at"`
	UpdatedAt        time.Time      `db:"updated_at" json:"updated_at"`
}

// ============================================================
// 6. ACTIVITY LOG (AUDIT TRAIL FORENSIK) & NOTIFIKASI
// Append-only (Tidak ada soft delete / update)
// ============================================================

// ActivityLog jejak audit setiap aksi CRUD & approval
type ActivityLog struct {
	ID         int64     `db:"id" json:"id"`
	ActorID    *string   `db:"actor_id" json:"actor_id,omitempty"`
	ActorName  string    `db:"actor_name" json:"actor_name"`
	ActorRole  string    `db:"actor_role" json:"actor_role"`
	IPAddress  string    `db:"ip_address" json:"ip_address"`
	UserAgent  string    `db:"user_agent" json:"user_agent"`
	EntityName string    `db:"entity_name" json:"entity_name"` // "pendaftaran" | "anggota" | "surat_keputusan" | "pengurus" | "users"
	EntityID   string    `db:"entity_id" json:"entity_id"`
	Action     string    `db:"action" json:"action"`     // "CREATE" | "UPDATE" | "DELETE" | "APPROVE" | "REJECT" | "LOGIN" | "LOGOUT"
	Metadata   *string   `db:"metadata" json:"metadata"` // JSON string perubahan (PII wajib dimasking)
	RequestID  string    `db:"request_id" json:"request_id"`
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
}

// AuditContext adalah konteks forensik transport (IP, user agent, request ID)
// yang dipasok handler ke service agar audit trail lengkap tanpa service
// bergantung pada framework HTTP (domain bebas Fiber — RULES 3).
type AuditContext struct {
	IP        string
	UserAgent string
	RequestID string
}

// ActorContext adalah identitas server-side pemanggil (dari JWT claims
// terverifikasi — RULES 6: tidak pernah dari body/query client). Service
// memakai ini untuk RBAC + jurisdiction (RULES 5, 7).
type ActorContext struct {
	UserID      string
	Name        string
	Role        Role
	ProvinsiID  *int
	KabupatenID *int
}

// CanAccessWilayah menentukan apakah aktor boleh menyentuh objek di
// (provID, kabID). Murni fungsi domain agar unit-testable tanpa DB.
// SUPER_ADMIN/NASIONAL: nasional. PROVINSI: cocok provinsi. KABUPATEN:
// cocok kabupaten (dan provinsi bila aktor memilikinya — data legacy
// yang yatim provinsi tetap presisi karena ID kabupaten unik nasional).
func (a ActorContext) CanAccessWilayah(provID, kabID int) bool {
	switch a.Role {
	case RoleSuperAdmin, RoleAdminNasional:
		return true
	case RoleAdminProvinsi:
		return a.ProvinsiID != nil && *a.ProvinsiID == provID
	case RoleAdminKabupaten:
		if a.KabupatenID == nil || *a.KabupatenID != kabID {
			return false
		}
		if a.ProvinsiID != nil && *a.ProvinsiID != provID {
			return false
		}
		return true
	default:
		return false
	}
}

// Notification notifikasi in-app untuk akun admin verifikator
type Notification struct {
	ID        int64            `db:"id" json:"id"`
	UserID    string           `db:"user_id" json:"user_id"`
	Title     string           `db:"title" json:"title"`
	Message   string           `db:"message" json:"message"`
	Type      NotificationType `db:"type" json:"type"` // PENDAFTARAN | SK | SISTEM
	Link      *string          `db:"link" json:"link,omitempty"`
	IsRead    bool             `db:"is_read" json:"is_read"`
	CreatedAt time.Time        `db:"created_at" json:"created_at"`
}
