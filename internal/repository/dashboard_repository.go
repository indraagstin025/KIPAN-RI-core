package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
)

// DashboardRepository menghitung agregat dashboard ter-scope wilayah.
// provID/kabID berasal dari ActorContext server-side (nil = tanpa batas).
type DashboardRepository interface {
	Load(ctx context.Context, provID, kabID *int) (*domain.DashboardData, error)
}

type dashboardRepo struct{ db *sqlx.DB }

func NewDashboardRepository(db *sqlx.DB) DashboardRepository { return &dashboardRepo{db: db} }

// scopeClause membangun klausa WHERE provinsi/kabupaten untuk alias tabel.
func scopeClause(alias string, provID, kabID *int) (string, []interface{}) {
	where := []string{"1 = 1"}
	args := []interface{}{}
	if provID != nil {
		args = append(args, *provID)
		where = append(where, fmt.Sprintf("%s.provinsi_id = $%d", alias, len(args)))
	}
	if kabID != nil {
		args = append(args, *kabID)
		where = append(where, fmt.Sprintf("%s.kabupaten_id = $%d", alias, len(args)))
	}
	return strings.Join(where, " AND "), args
}

func (r *dashboardRepo) Load(ctx context.Context, provID, kabID *int) (*domain.DashboardData, error) {
	out := &domain.DashboardData{
		PendaftaranByStatus: make([]domain.DashboardStatusCount, 0),
		WilayahDistribusi:   make([]domain.DashboardWilayahCount, 0),
		Trend:               make([]domain.DashboardTrendPoint, 0),
		Recent:              make([]domain.DashboardRecentItem, 0),
	}

	// 1. Ringkasan anggota.
	var agg struct {
		Total int `db:"total"`
		Aktif int `db:"aktif"`
		Baru  int `db:"baru"`
	}
	wA, aA := scopeClause("a", provID, kabID)
	if err := r.db.GetContext(ctx, &agg,
		`SELECT COUNT(*) AS total,
		        COUNT(*) FILTER (WHERE a.status = 'AKTIF') AS aktif,
		        COUNT(*) FILTER (WHERE a.tanggal_angkat >= date_trunc('month', CURRENT_DATE)) AS baru
		 FROM anggota a WHERE `+wA, aA...); err != nil {
		return nil, err
	}
	out.Summary.TotalAnggota = agg.Total
	out.Summary.AnggotaAktif = agg.Aktif
	out.Summary.AnggotaBaru = agg.Baru

	// 2. Pendaftaran per status + hitung menunggu verifikasi.
	wP, aP := scopeClause("p", provID, kabID)
	if err := r.db.SelectContext(ctx, &out.PendaftaranByStatus,
		`SELECT p.status AS status, COUNT(*) AS jumlah
		 FROM pendaftaran p WHERE `+wP+` GROUP BY p.status`, aP...); err != nil {
		return nil, err
	}
	for _, s := range out.PendaftaranByStatus {
		if s.Status == string(domain.PendaftaranStatusDraft) || s.Status == string(domain.PendaftaranStatusDiverifikasi) {
			out.Summary.MenungguVerifikasi += s.Jumlah
		}
	}

	// 3. Pengurus aktif.
	wG, aG := scopeClause("pg", provID, kabID)
	if err := r.db.GetContext(ctx, &out.Summary.TotalPengurus,
		`SELECT COUNT(*) FROM pengurus pg WHERE pg.status = 'Aktif' AND `+wG, aG...); err != nil {
		return nil, err
	}

	// 4. Total master wilayah aktif (nasional).
	if err := r.db.GetContext(ctx, &out.Summary.TotalProvinsi,
		`SELECT COUNT(*) FROM wilayah_provinsi WHERE is_active`); err != nil {
		return nil, err
	}
	if err := r.db.GetContext(ctx, &out.Summary.TotalKabupaten,
		`SELECT COUNT(*) FROM wilayah_kabupaten WHERE is_active`); err != nil {
		return nil, err
	}

	// 5. Distribusi anggota per wilayah (level mengikuti scope).
	switch {
	case kabID != nil:
		out.WilayahLabel = "Per Kecamatan"
		if err := r.db.SelectContext(ctx, &out.WilayahDistribusi,
			`SELECT COALESCE(NULLIF(a.kecamatan, ''), '(tidak diisi)') AS nama, COUNT(*) AS jumlah
			 FROM anggota a WHERE a.kabupaten_id = $1 GROUP BY 1 ORDER BY jumlah DESC LIMIT 8`,
			*kabID); err != nil {
			return nil, err
		}
	case provID != nil:
		out.WilayahLabel = "Per Kabupaten/Kota"
		if err := r.db.SelectContext(ctx, &out.WilayahDistribusi,
			`SELECT k.nama AS nama, COUNT(*) AS jumlah
			 FROM anggota a JOIN wilayah_kabupaten k ON k.id = a.kabupaten_id
			 WHERE a.provinsi_id = $1 GROUP BY k.nama ORDER BY jumlah DESC LIMIT 8`,
			*provID); err != nil {
			return nil, err
		}
	default:
		out.WilayahLabel = "Per Provinsi"
		if err := r.db.SelectContext(ctx, &out.WilayahDistribusi,
			`SELECT pr.nama AS nama, COUNT(*) AS jumlah
			 FROM anggota a JOIN wilayah_provinsi pr ON pr.id = a.provinsi_id
			 GROUP BY pr.nama ORDER BY jumlah DESC LIMIT 8`); err != nil {
			return nil, err
		}
	}

	// 6. Tren pendaftaran 7 bulan terakhir.
	if err := r.db.SelectContext(ctx, &out.Trend,
		`SELECT to_char(date_trunc('month', p.created_at), 'YYYY-MM') AS bulan, COUNT(*) AS jumlah
		 FROM pendaftaran p
		 WHERE p.created_at >= date_trunc('month', CURRENT_DATE) - INTERVAL '6 months' AND `+wP+`
		 GROUP BY 1 ORDER BY 1`, aP...); err != nil {
		return nil, err
	}

	// 7. Pendaftaran terbaru (non-PII).
	if err := r.db.SelectContext(ctx, &out.Recent,
		`SELECT p.id AS id, p.nama_lengkap AS nama, p.status AS status,
		        k.nama AS kabupaten, p.created_at AS created_at
		 FROM pendaftaran p LEFT JOIN wilayah_kabupaten k ON k.id = p.kabupaten_id
		 WHERE `+wP+` ORDER BY p.created_at DESC LIMIT 5`, aP...); err != nil {
		return nil, err
	}

	return out, nil
}
