package domain

// capabilities.go adalah SATU sumber kebenaran wewenang admin (role × capability).
// Middleware (RequireCapability), halaman Role & Wewenang, dan test RBAC membaca
// dari registry ini agar definisi hak akses tidak tersebar/berbeda.

// Capability adalah wewenang fungsional yang dapat dimiliki sebuah role.
type Capability string

const (
	CapViewDashboard       Capability = "view_dashboard"        // lihat dasbor ter-scope
	CapViewPendaftaran     Capability = "view_pendaftaran"      // lihat antrean/detail pendaftaran
	CapVerifyPendaftaran   Capability = "verify_pendaftaran"    // putusan verifikasi (setujui/tolak/perbaikan)
	CapManageKepengurusan  Capability = "manage_kepengurusan"   // kelola SK & pengurus
	CapCreateSK            Capability = "create_sk"             // susun & ajukan SK
	CapManageOutbox        Capability = "manage_outbox"         // kelola antrian email
	CapViewLaporan         Capability = "view_laporan"          // laporan & statistik
	CapViewAudit           Capability = "view_audit"            // penelusur audit
	CapCreateJabatan       Capability = "create_jabatan"        // tambah entri master jabatan (semua admin)
	CapManageJabatan       Capability = "manage_jabatan"        // ubah/nonaktifkan master jabatan
	CapManageWilayah       Capability = "manage_wilayah"        // master wilayah
	CapManageUsers         Capability = "manage_users"          // manajemen akun admin (Super)
	CapManageRegionalUsers Capability = "manage_regional_users" // kelola akun Provinsi/Kabupaten (Super+Nasional)
	CapManageOrganisasi    Capability = "manage_organisasi"     // profil organisasi
	CapManageBackup        Capability = "manage_backup"         // backup database
)

// RoleInfo adalah metadata satu role untuk katalog Role & Wewenang.
type RoleInfo struct {
	Key         Role   `json:"key"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

// RoleCatalog adalah katalog read-only role + matriks wewenang.
type RoleCatalog struct {
	Roles        []RoleInfo       `json:"roles"`
	Capabilities []CapabilityInfo `json:"capabilities"`
}

// roleRegistry adalah daftar role (termasuk USER) untuk katalog.
var roleRegistry = []RoleInfo{
	{RoleSuperAdmin, "Super Admin", "Akses penuh seluruh Indonesia termasuk konfigurasi sistem."},
	{RoleAdminNasional, "Admin Nasional (DPN)", "Cakupan seluruh Indonesia; tanpa pengaturan sistem (Super eksklusif)."},
	{RoleAdminProvinsi, "Admin Provinsi (DPD)", "Cakupan satu provinsi + kab/kota di bawahnya."},
	{RoleAdminKabupaten, "Admin Kabupaten/Kota (DPC)", "Cakupan satu kabupaten/kota."},
	{RoleUser, "Anggota", "Akun kader: profil & e-KTA sendiri."},
}

// AllRoles mengembalikan katalog role (salinan).
func AllRoles() []RoleInfo {
	out := make([]RoleInfo, len(roleRegistry))
	copy(out, roleRegistry)
	return out
}

// CapabilityInfo adalah metadata satu capability untuk katalog UI & audit.
type CapabilityInfo struct {
	Key         Capability `json:"key"`
	Label       string     `json:"label"`
	Description string     `json:"description"`
	Roles       []Role     `json:"roles"`
}

// allAdminRoles adalah daftar peran admin (tanpa USER) untuk wewenang umum.
var allAdminRoles = []Role{RoleSuperAdmin, RoleAdminNasional, RoleAdminProvinsi, RoleAdminKabupaten}

// capabilityRegistry adalah daftar lengkap & terurut capability (sumber data).
var capabilityRegistry = []CapabilityInfo{
	{CapViewDashboard, "Lihat Dasbor", "Melihat ringkasan & analitik sesuai yurisdiksi.", allAdminRoles},
	{CapViewPendaftaran, "Lihat Pendaftaran", "Melihat antrean & detail pendaftaran sesuai yurisdiksi.", allAdminRoles},
	{CapVerifyPendaftaran, "Verifikasi Pendaftaran", "Memutuskan verifikasi pendaftaran (setujui/tolak/perbaikan).", []Role{RoleSuperAdmin, RoleAdminNasional, RoleAdminKabupaten}},
	{CapManageKepengurusan, "Kelola Kepengurusan", "Menyusun/mengelola SK dan personalia pengurus.", allAdminRoles},
	{CapCreateSK, "Buat & Ajukan SK", "Menyusun dan mengajukan Surat Keputusan.", allAdminRoles},
	{CapManageOutbox, "Kelola Antrian Email", "Melihat & mengirim ulang email outbox.", allAdminRoles},
	{CapViewLaporan, "Lihat Laporan", "Melihat laporan & statistik ter-scope.", allAdminRoles},
	{CapViewAudit, "Lihat Audit", "Menelusuri jejak audit (filter & ekspor).", []Role{RoleSuperAdmin, RoleAdminNasional}},
	{CapCreateJabatan, "Tambah Jabatan", "Menambah entri jabatan baru (tanpa level).", allAdminRoles},
	{CapManageJabatan, "Kelola Jabatan", "Mengubah/menonaktifkan master jabatan.", []Role{RoleSuperAdmin, RoleAdminNasional}},
	{CapManageWilayah, "Master Wilayah", "Mengelola master data wilayah.", []Role{RoleSuperAdmin, RoleAdminNasional}},
	{CapManageUsers, "Manajemen Pengguna", "Mengelola semua akun admin.", []Role{RoleSuperAdmin}},
	{CapManageRegionalUsers, "Kelola Akun Regional", "Mengelola akun admin Provinsi/Kabupaten.", []Role{RoleSuperAdmin, RoleAdminNasional}},
	{CapManageOrganisasi, "Profil Organisasi", "Mengelola profil organisasi & halaman publik.", []Role{RoleSuperAdmin}},
	{CapManageBackup, "Database Backup", "Membuat & mengunduh backup database.", []Role{RoleSuperAdmin}},
}

// capabilityIndex memetakan key → metadata untuk lookup cepat.
var capabilityIndex = func() map[Capability]CapabilityInfo {
	m := make(map[Capability]CapabilityInfo, len(capabilityRegistry))
	for _, info := range capabilityRegistry {
		m[info.Key] = info
	}
	return m
}()

// copyRoles mengembalikan salinan slice role agar pemanggil tidak bisa memutasi
// registry internal.
func copyRoles(roles []Role) []Role {
	out := make([]Role, len(roles))
	copy(out, roles)
	return out
}

// AllCapabilities mengembalikan katalog lengkap (salinan) terurut.
func AllCapabilities() []CapabilityInfo {
	out := make([]CapabilityInfo, len(capabilityRegistry))
	for i, info := range capabilityRegistry {
		info.Roles = copyRoles(info.Roles)
		out[i] = info
	}
	return out
}

// KnownCapability mengembalikan true bila key terdaftar di registry.
func KnownCapability(cap Capability) bool {
	_, ok := capabilityIndex[cap]
	return ok
}

// RolesForCapability mengembalikan salinan daftar role yang memiliki wewenang.
func RolesForCapability(cap Capability) []Role {
	info, ok := capabilityIndex[cap]
	if !ok {
		return nil
	}
	return copyRoles(info.Roles)
}

// HasCapability mengembalikan true bila role memiliki wewenang tertentu.
func HasCapability(role Role, cap Capability) bool {
	info, ok := capabilityIndex[cap]
	if !ok {
		return false
	}
	for _, r := range info.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// CapabilitiesForRole mengembalikan daftar capability milik role (terurut
// sesuai registry). Role tak dikenal = daftar kosong.
func CapabilitiesForRole(role Role) []Capability {
	out := make([]Capability, 0, len(capabilityRegistry))
	for _, info := range capabilityRegistry {
		if HasCapability(role, info.Key) {
			out = append(out, info.Key)
		}
	}
	return out
}
