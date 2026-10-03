package repository

// laporan_repository.go — agregat laporan & statistik ter-scope wilayah.

import (
	"context"
	"strconv"

	"github.com/jmoiron/sqlx"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
)

// LaporanAnomaliRow adalah baris mentah untuk deteksi anomali NIA (NIK perlu
// didekripsi di service, tidak pernah meninggalkan server).
type LaporanAnomaliRow struct {
	NIA          string `db:"nia"`
	Nama         string `db:"nama_lengkap"`
	NIKEncrypted string `db:"nik_encrypted"`
	KodeDomisili string `db:"kode"`
}

// LaporanRepository menghitung agregat laporan (provID/kabID dari ActorContext).
type LaporanRepository interface {
	Load(ctx context.Context, provID, kabID *int) (*domain.LaporanData, error)
	// ListAnomaliCandidates mengambil calon anomali (perlu dekripsi NIK).
	ListAnomaliCandidates(ctx context.Context, provID, kabID *int, limit int) ([]LaporanAnomaliRow, error)
}

type laporanRepo struct{ db *sqlx.DB }

func NewLaporanRepository(db *sqlx.DB) LaporanRepository { return &laporanRepo{db: db} }

func (r *laporanRepo) Load(ctx context.Context, provID, kabID *int) (*domain.LaporanData, error) {
	out := &domain.LaporanData{
		AnggotaByStatus:     make([]domain.LaporanCount, 0),
		PendaftaranByStatus: make([]domain.LaporanCount, 0),
		PengurusByLevel:     make([]domain.LaporanCount, 0),
		DemografiUsia:       make([]domain.LaporanCount, 0),
		DemografiPendidikan: make([]domain.LaporanCount, 0),
		DemografiPekerjaan:  make([]domain.LaporanCount, 0),
		Wilayah:             make([]domain.LaporanCount, 0),
		Tren:                make([]domain.LaporanTrendPoint, 0),
	}

	wA, aA := scopeClause("a", provID, kabID)
	wP, aP := scopeClause("p", provID, kabID)
	wG, aG := scopeClause("pg", provID, kabID)
	wS, aS := scopeClause("sk", provID, kabID)

	// 1. Ringkasan (query terpisah agar placeholder scope tidak tumpang-tindih).
	var (
		totalAnggota, anggotaAktif, totalPengurus, totalSKAktif int
	)
	if err := r.db.GetContext(ctx, &totalAnggota,
		`SELECT COUNT(*) FROM anggota a WHERE `+wA, aA...); err != nil {
		return nil, err
	}
	if err := r.db.GetContext(ctx, &anggotaAktif,
		`SELECT COUNT(*) FROM anggota a WHERE a.status = 'AKTIF' AND `+wA, aA...); err != nil {
		return nil, err
	}
	if err := r.db.GetContext(ctx, &totalPengurus,
		`SELECT COUNT(*) FROM pengurus pg JOIN surat_keputusan sk ON sk.id = pg.surat_keputusan_id
		 WHERE pengurus_status_efektif(pg.status, sk.status, sk.tanggal_berakhir, CURRENT_DATE) = 'Aktif' AND `+wG, aG...); err != nil {
		return nil, err
	}
	if err := r.db.GetContext(ctx, &totalSKAktif,
		`SELECT COUNT(*) FROM surat_keputusan sk WHERE sk.status = 'Aktif' AND sk.approval_status = 'DISETUJUI' AND `+wS, aS...); err != nil {
		return nil, err
	}
	out.Summary.TotalAnggota = totalAnggota
	out.Summary.AnggotaAktif = anggotaAktif
	out.Summary.TotalPengurus = totalPengurus
	out.Summary.TotalSKAktif = totalSKAktif

	// 2. Anggota per status.
	if err := r.db.SelectContext(ctx, &out.AnggotaByStatus,
		`SELECT a.status AS label, COUNT(*) AS jumlah FROM anggota a WHERE `+wA+` GROUP BY a.status ORDER BY jumlah DESC`, aA...); err != nil {
		return nil, err
	}

	// 3. Pendaftaran per status (+ menunggu verifikasi).
	if err := r.db.SelectContext(ctx, &out.PendaftaranByStatus,
		`SELECT p.status AS label, COUNT(*) AS jumlah FROM pendaftaran p WHERE `+wP+` GROUP BY p.status ORDER BY jumlah DESC`, aP...); err != nil {
		return nil, err
	}
	for _, c := range out.PendaftaranByStatus {
		if c.Label == string(domain.PendaftaranStatusDraft) || c.Label == string(domain.PendaftaranStatusDiverifikasi) {
			out.Summary.MenungguVerif += c.Jumlah
		}
	}

	// 4. Pengurus aktif per level (efektif).
	if err := r.db.SelectContext(ctx, &out.PengurusByLevel,
		`SELECT pg.level AS label, COUNT(*) AS jumlah
		 FROM pengurus pg JOIN surat_keputusan sk ON sk.id = pg.surat_keputusan_id
		 WHERE pengurus_status_efektif(pg.status, sk.status, sk.tanggal_berakhir, CURRENT_DATE) = 'Aktif' AND `+wG+`
		 GROUP BY pg.level ORDER BY jumlah DESC`, aG...); err != nil {
		return nil, err
	}

	// 5. Demografi usia (kelompok).
	if err := r.db.SelectContext(ctx, &out.DemografiUsia, `
		SELECT CASE
			WHEN date_part('year', age(a.tanggal_lahir)) < 21 THEN '<=20'
			WHEN date_part('year', age(a.tanggal_lahir)) BETWEEN 21 AND 25 THEN '21-25'
			WHEN date_part('year', age(a.tanggal_lahir)) BETWEEN 26 AND 30 THEN '26-30'
			ELSE '>30' END AS label,
			COUNT(*) AS jumlah
		FROM anggota a WHERE `+wA+` GROUP BY 1 ORDER BY 1`, aA...); err != nil {
		return nil, err
	}

	// 6. Demografi pendidikan.
	if err := r.db.SelectContext(ctx, &out.DemografiPendidikan,
		`SELECT COALESCE(NULLIF(a.pendidikan, ''), '(tidak diisi)') AS label, COUNT(*) AS jumlah
		 FROM anggota a WHERE `+wA+` GROUP BY 1 ORDER BY jumlah DESC LIMIT 10`, aA...); err != nil {
		return nil, err
	}

	// 7. Demografi pekerjaan.
	if err := r.db.SelectContext(ctx, &out.DemografiPekerjaan,
		`SELECT COALESCE(NULLIF(a.pekerjaan, ''), '(tidak diisi)') AS label, COUNT(*) AS jumlah
		 FROM anggota a WHERE `+wA+` GROUP BY 1 ORDER BY jumlah DESC LIMIT 10`, aA...); err != nil {
		return nil, err
	}

	// 8. Distribusi wilayah (level mengikuti scope).
	switch {
	case kabID != nil:
		out.WilayahLabel = "Per Kecamatan"
		if err := r.db.SelectContext(ctx, &out.Wilayah,
			`SELECT COALESCE(NULLIF(a.kecamatan, ''), '(tidak diisi)') AS label, COUNT(*) AS jumlah
			 FROM anggota a WHERE a.kabupaten_id = $1 GROUP BY 1 ORDER BY jumlah DESC LIMIT 10`, *kabID); err != nil {
			return nil, err
		}
	case provID != nil:
		out.WilayahLabel = "Per Kabupaten/Kota"
		if err := r.db.SelectContext(ctx, &out.Wilayah,
			`SELECT k.nama AS label, COUNT(*) AS jumlah
			 FROM anggota a JOIN wilayah_kabupaten k ON k.id = a.kabupaten_id
			 WHERE a.provinsi_id = $1 GROUP BY k.nama ORDER BY jumlah DESC LIMIT 10`, *provID); err != nil {
			return nil, err
		}
	default:
		out.WilayahLabel = "Per Provinsi"
		if err := r.db.SelectContext(ctx, &out.Wilayah,
			`SELECT pr.nama AS label, COUNT(*) AS jumlah
			 FROM anggota a JOIN wilayah_provinsi pr ON pr.id = a.provinsi_id
			 GROUP BY pr.nama ORDER BY jumlah DESC LIMIT 10`); err != nil {
			return nil, err
		}
	}

	// 9. Tren anggota baru 12 bulan.
	if err := r.db.SelectContext(ctx, &out.Tren,
		`SELECT to_char(date_trunc('month', a.tanggal_angkat), 'YYYY-MM') AS bulan, COUNT(*) AS jumlah
		 FROM anggota a
		 WHERE a.tanggal_angkat >= date_trunc('month', CURRENT_DATE) - INTERVAL '11 months' AND `+wA+`
		 GROUP BY 1 ORDER BY 1`, aA...); err != nil {
		return nil, err
	}

	return out, nil
}

// ListAnomaliCandidates mengambil calon anomali NIA (NIK terenkripsi) ter-scope.
func (r *laporanRepo) ListAnomaliCandidates(ctx context.Context, provID, kabID *int, limit int) ([]LaporanAnomaliRow, error) {
	if limit <= 0 || limit > 5000 {
		limit = 3000
	}
	wA, aA := scopeClause("a", provID, kabID)
	aA = append(aA, limit)
	rows := make([]LaporanAnomaliRow, 0)
	query := `SELECT a.nia, a.nama_lengkap, a.nik_encrypted, k.kode
		FROM anggota a JOIN wilayah_kabupaten k ON k.id = a.kabupaten_id
		WHERE ` + wA + ` ORDER BY a.id LIMIT $` + strconv.Itoa(len(aA))
	if err := r.db.SelectContext(ctx, &rows, query, aA...); err != nil {
		return nil, err
	}
	return rows, nil
}
