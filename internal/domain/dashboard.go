package domain

import "time"

// ============================================================
// DASHBOARD ANALITIK (agregat, ter-scope wilayah)
// ============================================================

// DashboardSummary ringkasan angka utama.
type DashboardSummary struct {
	TotalAnggota       int `json:"total_anggota"`
	AnggotaAktif       int `json:"anggota_aktif"`
	AnggotaBaru        int `json:"anggota_baru_bulan_ini"`
	MenungguVerifikasi int `json:"menunggu_verifikasi"`
	TotalPengurus      int `json:"total_pengurus"`
	TotalProvinsi      int `json:"total_provinsi"`
	TotalKabupaten     int `json:"total_kabupaten"`
}

// DashboardStatusCount jumlah pendaftaran per status.
type DashboardStatusCount struct {
	Status string `json:"status" db:"status"`
	Jumlah int    `json:"jumlah" db:"jumlah"`
}

// DashboardWilayahCount distribusi anggota per wilayah (provinsi/kabupaten/kecamatan).
type DashboardWilayahCount struct {
	Nama   string `json:"nama" db:"nama"`
	Jumlah int    `json:"jumlah" db:"jumlah"`
}

// DashboardTrendPoint tren pendaftaran bulanan.
type DashboardTrendPoint struct {
	Bulan  string `json:"bulan" db:"bulan"`
	Jumlah int    `json:"jumlah" db:"jumlah"`
}

// DashboardRecentItem pendaftaran terbaru (non-PII).
type DashboardRecentItem struct {
	ID        int       `json:"id" db:"id"`
	Nama      string    `json:"nama" db:"nama"`
	Status    string    `json:"status" db:"status"`
	Kabupaten *string   `json:"kabupaten,omitempty" db:"kabupaten"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// DashboardData payload lengkap dashboard.
type DashboardData struct {
	Summary             DashboardSummary        `json:"summary"`
	PendaftaranByStatus []DashboardStatusCount  `json:"pendaftaran_by_status"`
	WilayahDistribusi   []DashboardWilayahCount `json:"wilayah_distribusi"`
	WilayahLabel        string                  `json:"wilayah_label"`
	Trend               []DashboardTrendPoint   `json:"trend"`
	Recent              []DashboardRecentItem   `json:"recent"`
}
