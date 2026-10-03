package domain

// ============================================================
// MASTER WILAYAH — ADMIN (Super/Nasional)
// ============================================================

// WilayahAdminItem proyeksi baris tabel admin (provinsi & kabupaten).
type WilayahAdminItem struct {
	ID           int     `db:"id" json:"id"`
	Kode         string  `db:"kode" json:"kode"`
	Nama         string  `db:"nama" json:"nama"`
	IsActive     bool    `db:"is_active" json:"is_active"`
	ProvinsiID   *int    `db:"provinsi_id" json:"provinsi_id,omitempty"`
	ProvinsiNama *string `db:"provinsi_nama" json:"provinsi_nama,omitempty"`
	JmlKabupaten int     `db:"jml_kabupaten" json:"jml_kabupaten"`
	JmlPengurus  int     `db:"jml_pengurus" json:"jml_pengurus"`
	Ketua        *string `db:"ketua" json:"ketua,omitempty"`
}

// TrenBulan satu titik tren pengurus per bulan (format YYYY-MM).
type TrenBulan struct {
	Bulan  string `json:"bulan"`
	Jumlah int    `json:"jumlah"`
}

// WilayahStatistik ringkasan pengurus sebuah wilayah.
type WilayahStatistik struct {
	TotalPengurus  int         `json:"total_pengurus"`
	PengurusAktif  int         `json:"pengurus_aktif"`
	TotalKabupaten int         `json:"total_kabupaten"`
	Tren           []TrenBulan `json:"tren"`
}

// WilayahDetailAdmin gabungan info + pengurus + statistik + activity.
type WilayahDetailAdmin struct {
	Type         string           `json:"type"`
	ID           int              `json:"id"`
	Kode         string           `json:"kode"`
	Nama         string           `json:"nama"`
	IsActive     bool             `json:"is_active"`
	ProvinsiID   *int             `json:"provinsi_id,omitempty"`
	ProvinsiNama *string          `json:"provinsi_nama,omitempty"`
	Statistik    WilayahStatistik `json:"statistik"`
	Pengurus     []PengurusDetail `json:"pengurus"`
	Activity     []ActivityLog    `json:"activity"`
}
