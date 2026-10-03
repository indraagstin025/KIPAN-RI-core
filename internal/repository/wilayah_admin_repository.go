package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
)

// WilayahAdminRepository operasi master wilayah untuk Super/Nasional.
// Interface terpisah dari WilayahRepository (publik) agar konsumen read-only
// tak ikut terpengaruh; implementasi sama-sama di *wilayahRepo.
type WilayahAdminRepository interface {
	ListProvinsiAdmin(ctx context.Context, search, status string) ([]domain.WilayahAdminItem, error)
	ListKabupatenAdmin(ctx context.Context, provinsiID *int, search, status string) ([]domain.WilayahAdminItem, error)
	GetProvinsi(ctx context.Context, id int) (*domain.WilayahProvinsi, error)
	GetKabupaten(ctx context.Context, id int) (*domain.WilayahKabupaten, error)
	SetProvinsiActive(ctx context.Context, id int, active bool) error
	SetKabupatenActive(ctx context.Context, id int, active bool) error
	EnsureProvinsiActive(ctx context.Context, id int) error
	EnsureKabupatenActive(ctx context.Context, provinsiID, kabupatenID int) error
	CountProvinsi(ctx context.Context) (int, error)
	CountKabupaten(ctx context.Context, provinsiID *int) (int, error)
	CountPengurusAktif(ctx context.Context) (int, error)
	ListPengurusWilayah(ctx context.Context, level string, provinsiID, kabupatenID *int, all bool) ([]domain.PengurusDetail, error)
	StatsPengurusWilayah(ctx context.Context, level string, provinsiID, kabupatenID *int) (total, aktif int, err error)
	TrenPengurusWilayah(ctx context.Context, level string, provinsiID, kabupatenID *int) ([]domain.TrenBulan, error)
	ListActivityWilayah(ctx context.Context, entityName, entityID string, limit int) ([]domain.ActivityLog, error)
}

// NewWilayahAdminRepository membangun repo admin di atas *wilayahRepo.
func NewWilayahAdminRepository(db *sqlx.DB) WilayahAdminRepository { return &wilayahRepo{db: db} }

const ketuaSubqueryProvinsi = `(SELECT a.nama_lengkap FROM pengurus p
	JOIN anggota a ON a.id = p.anggota_id
	JOIN jabatan j ON j.id = p.jabatan_id
	WHERE p.provinsi_id = wp.id AND p.level = 'PROVINSI' AND p.status = 'Aktif'
	  AND j.is_inti = TRUE AND j.nama = 'Ketua' LIMIT 1) AS ketua`

const ketuaSubqueryKabupaten = `(SELECT a.nama_lengkap FROM pengurus p
	JOIN anggota a ON a.id = p.anggota_id
	JOIN jabatan j ON j.id = p.jabatan_id
	WHERE p.kabupaten_id = wk.id AND p.level = 'KABUPATEN' AND p.status = 'Aktif'
	  AND j.is_inti = TRUE AND j.nama = 'Ketua' LIMIT 1) AS ketua`

func statusClause(col, status string) string {
	switch strings.TrimSpace(status) {
	case "Aktif":
		return col + " = TRUE"
	case "Nonaktif":
		return col + " = FALSE"
	}
	return ""
}

func (r *wilayahRepo) ListProvinsiAdmin(ctx context.Context, search, status string) ([]domain.WilayahAdminItem, error) {
	items := make([]domain.WilayahAdminItem, 0)
	where := []string{"1 = 1"}
	args := []interface{}{}
	if s := strings.TrimSpace(search); s != "" {
		args = append(args, "%"+s+"%")
		where = append(where, fmt.Sprintf("(wp.nama ILIKE $%d OR wp.kode ILIKE $%d)", len(args), len(args)))
	}
	if c := statusClause("wp.is_active", status); c != "" {
		where = append(where, c)
	}
	q := `SELECT wp.id, wp.kode, wp.nama, wp.is_active,
		(SELECT COUNT(*) FROM wilayah_kabupaten k WHERE k.provinsi_id = wp.id) AS jml_kabupaten,
		(SELECT COUNT(*) FROM pengurus p WHERE p.provinsi_id = wp.id AND p.level = 'PROVINSI' AND p.status = 'Aktif') AS jml_pengurus,
		` + ketuaSubqueryProvinsi + `
		FROM wilayah_provinsi wp WHERE ` + strings.Join(where, " AND ") + ` ORDER BY wp.nama`
	if err := r.db.SelectContext(ctx, &items, q, args...); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *wilayahRepo) ListKabupatenAdmin(ctx context.Context, provinsiID *int, search, status string) ([]domain.WilayahAdminItem, error) {
	items := make([]domain.WilayahAdminItem, 0)
	where := []string{"1 = 1"}
	args := []interface{}{}
	if provinsiID != nil {
		args = append(args, *provinsiID)
		where = append(where, fmt.Sprintf("wk.provinsi_id = $%d", len(args)))
	}
	if s := strings.TrimSpace(search); s != "" {
		args = append(args, "%"+s+"%")
		where = append(where, fmt.Sprintf("(wk.nama ILIKE $%d OR wk.kode ILIKE $%d)", len(args), len(args)))
	}
	if c := statusClause("wk.is_active", status); c != "" {
		where = append(where, c)
	}
	q := `SELECT wk.id, wk.kode, wk.nama, wk.is_active, wk.provinsi_id, wp.nama AS provinsi_nama,
		0 AS jml_kabupaten,
		(SELECT COUNT(*) FROM pengurus p WHERE p.kabupaten_id = wk.id AND p.level = 'KABUPATEN' AND p.status = 'Aktif') AS jml_pengurus,
		` + ketuaSubqueryKabupaten + `
		FROM wilayah_kabupaten wk
		JOIN wilayah_provinsi wp ON wp.id = wk.provinsi_id
		WHERE ` + strings.Join(where, " AND ") + ` ORDER BY wp.nama, wk.nama`
	if err := r.db.SelectContext(ctx, &items, q, args...); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *wilayahRepo) GetProvinsi(ctx context.Context, id int) (*domain.WilayahProvinsi, error) {
	var w domain.WilayahProvinsi
	q := `SELECT id, kode, nama, is_active, created_at, updated_at FROM wilayah_provinsi WHERE id = $1`
	if err := r.db.GetContext(ctx, &w, q, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &w, nil
}

func (r *wilayahRepo) GetKabupaten(ctx context.Context, id int) (*domain.WilayahKabupaten, error) {
	var w domain.WilayahKabupaten
	q := `SELECT id, provinsi_id, kode, nama, is_active, created_at, updated_at FROM wilayah_kabupaten WHERE id = $1`
	if err := r.db.GetContext(ctx, &w, q, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &w, nil
}

func (r *wilayahRepo) SetProvinsiActive(ctx context.Context, id int, active bool) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE wilayah_provinsi SET is_active = $2, updated_at = CURRENT_TIMESTAMP WHERE id = $1`, id, active)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *wilayahRepo) SetKabupatenActive(ctx context.Context, id int, active bool) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE wilayah_kabupaten SET is_active = $2, updated_at = CURRENT_TIMESTAMP WHERE id = $1`, id, active)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *wilayahRepo) EnsureProvinsiActive(ctx context.Context, id int) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE wilayah_provinsi SET is_active = TRUE, updated_at = CURRENT_TIMESTAMP WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *wilayahRepo) EnsureKabupatenActive(ctx context.Context, provinsiID, kabupatenID int) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE wilayah_kabupaten SET is_active = TRUE, updated_at = CURRENT_TIMESTAMP
		 WHERE id = $1 AND provinsi_id = $2`, kabupatenID, provinsiID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *wilayahRepo) CountProvinsi(ctx context.Context) (int, error) {
	var n int
	err := r.db.GetContext(ctx, &n, `SELECT COUNT(*) FROM wilayah_provinsi`)
	return n, err
}

func (r *wilayahRepo) CountKabupaten(ctx context.Context, provinsiID *int) (int, error) {
	var n int
	if provinsiID == nil {
		err := r.db.GetContext(ctx, &n, `SELECT COUNT(*) FROM wilayah_kabupaten`)
		return n, err
	}
	err := r.db.GetContext(ctx, &n, `SELECT COUNT(*) FROM wilayah_kabupaten WHERE provinsi_id = $1`, *provinsiID)
	return n, err
}

func (r *wilayahRepo) CountPengurusAktif(ctx context.Context) (int, error) {
	var n int
	err := r.db.GetContext(ctx, &n, `SELECT COUNT(*) FROM pengurus WHERE status = 'Aktif'`)
	return n, err
}

// pengurusWilayahWhere membangun WHERE ter-scope untuk level + wilayah.
func pengurusWilayahWhere(level string, provinsiID, kabupatenID *int, all bool) (string, []interface{}) {
	args := []interface{}{level}
	where := []string{"p.level = $1"}
	if !all {
		where = append(where, "p.status = 'Aktif'")
	}
	if provinsiID != nil {
		args = append(args, *provinsiID)
		where = append(where, fmt.Sprintf("p.provinsi_id = $%d", len(args)))
	}
	if kabupatenID != nil {
		args = append(args, *kabupatenID)
		where = append(where, fmt.Sprintf("p.kabupaten_id = $%d", len(args)))
	}
	return strings.Join(where, " AND "), args
}

func (r *wilayahRepo) ListPengurusWilayah(ctx context.Context, level string, provinsiID, kabupatenID *int, all bool) ([]domain.PengurusDetail, error) {
	items := make([]domain.PengurusDetail, 0)
	where, args := pengurusWilayahWhere(level, provinsiID, kabupatenID, all)
	q := `SELECT ` + pengurusDetailColumns + ` ` + pengurusDetailJoins + ` WHERE ` + where +
		` ORDER BY j.urutan, a.nama_lengkap LIMIT 1000`
	if err := r.db.SelectContext(ctx, &items, q, args...); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *wilayahRepo) StatsPengurusWilayah(ctx context.Context, level string, provinsiID, kabupatenID *int) (int, int, error) {
	where, args := pengurusWilayahWhere(level, provinsiID, kabupatenID, true)
	var out struct {
		Total int `db:"total"`
		Aktif int `db:"aktif"`
	}
	q := `SELECT COUNT(*) AS total,
		COUNT(*) FILTER (WHERE p.status = 'Aktif') AS aktif
		FROM pengurus p WHERE ` + where
	if err := r.db.GetContext(ctx, &out, q, args...); err != nil {
		return 0, 0, err
	}
	return out.Total, out.Aktif, nil
}

func (r *wilayahRepo) TrenPengurusWilayah(ctx context.Context, level string, provinsiID, kabupatenID *int) ([]domain.TrenBulan, error) {
	items := make([]domain.TrenBulan, 0)
	where, args := pengurusWilayahWhere(level, provinsiID, kabupatenID, true)
	q := `SELECT to_char(date_trunc('month', p.created_at), 'YYYY-MM') AS bulan, COUNT(*) AS jumlah
		FROM pengurus p
		WHERE ` + where + `
		  AND p.created_at >= date_trunc('month', CURRENT_DATE) - INTERVAL '5 months'
		GROUP BY 1 ORDER BY 1`
	if err := r.db.SelectContext(ctx, &items, q, args...); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *wilayahRepo) ListActivityWilayah(ctx context.Context, entityName, entityID string, limit int) ([]domain.ActivityLog, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	items := make([]domain.ActivityLog, 0)
	q := `SELECT id, actor_id, actor_name, actor_role, ip_address, user_agent,
		entity_name, entity_id, action, metadata, request_id, created_at
		FROM activity_logs WHERE entity_name = $1 AND entity_id = $2
		ORDER BY created_at DESC LIMIT $3`
	if err := r.db.SelectContext(ctx, &items, q, entityName, entityID, limit); err != nil {
		return nil, err
	}
	return items, nil
}
