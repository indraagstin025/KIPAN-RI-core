package domain

// laporan.go — DTO laporan & statistik ter-scope (TDD §6.8).

// LaporanSummary ringkasan angka utama.
type LaporanSummary struct {
	TotalAnggota  int `db:"total_anggota" json:"total_anggota"`
	AnggotaAktif  int `db:"anggota_aktif" json:"anggota_aktif"`
	TotalPengurus int `db:"total_pengurus" json:"total_pengurus"`
	TotalSKAktif  int `db:"total_sk_aktif" json:"total_sk_aktif"`
	MenungguVerif int `db:"-" json:"menunggu_verifikasi"`
}

// LaporanCount hitungan berlabel (status/level/demografi/wilayah).
type LaporanCount struct {
	Label  string `db:"label" json:"label"`
	Jumlah int    `db:"jumlah" json:"jumlah"`
}

// LaporanTrendPoint tren bulanan.
type LaporanTrendPoint struct {
	Bulan  string `db:"bulan" json:"bulan"`
	Jumlah int    `db:"jumlah" json:"jumlah"`
}

// LaporanAnomali satu temuan anomali kode NIA vs NIK (TDD laporan anomali).
type LaporanAnomali struct {
	NIA          string `json:"nia"`
	Nama         string `json:"nama"`
	KodeNIK      string `json:"kode_nik"`
	KodeDomisili string `json:"kode_domisili"`
}

// LaporanData agregat laporan lengkap.
type LaporanData struct {
	Summary             LaporanSummary      `json:"summary"`
	AnggotaByStatus     []LaporanCount      `json:"anggota_by_status"`
	PendaftaranByStatus []LaporanCount      `json:"pendaftaran_by_status"`
	PengurusByLevel     []LaporanCount      `json:"pengurus_by_level"`
	DemografiUsia       []LaporanCount      `json:"demografi_usia"`
	DemografiPendidikan []LaporanCount      `json:"demografi_pendidikan"`
	DemografiPekerjaan  []LaporanCount      `json:"demografi_pekerjaan"`
	WilayahLabel        string              `json:"wilayah_label"`
	Wilayah             []LaporanCount      `json:"wilayah"`
	Tren                []LaporanTrendPoint `json:"tren"`
	AnomaliNIA          []LaporanAnomali    `json:"anomali_nia"`
	AnomaliCount        int                 `json:"anomali_count"`
}
