package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
)

// UserAdminRepository operasi manajemen akun admin (Super Admin).
// Interface terpisah agar konsumen lain (auth) tak terpengaruh.
type UserAdminRepository interface {
	// ListAdmin memfilter berdasarkan daftar role (kosong = semua role admin).
	ListAdmin(ctx context.Context, roles []string, status, search string, limit, offset int) ([]domain.User, int, error)
	Counts(ctx context.Context) (*domain.AdminUserCounts, error)
	CreateAdmin(ctx context.Context, u *domain.User) error
	UpdateAdminProfile(ctx context.Context, id, name, email string, role domain.Role, provinsiID, kabupatenID *int, status domain.UserStatus) error
	SoftDeleteAdmin(ctx context.Context, id string) error
	CountActiveSuperAdmins(ctx context.Context, excludeID string) (int, error)
}

type userAdminRepo struct{ db *sqlx.DB }

func NewUserAdminRepository(db *sqlx.DB) UserAdminRepository { return &userAdminRepo{db: db} }

func (r *userAdminRepo) CreateAdmin(ctx context.Context, u *domain.User) error {
	if u.ID == "" {
		u.ID = uuid.NewString()
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO users (id, email, password_hash, name, role, tipe_user, status, provinsi_id, kabupaten_id, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		u.ID, u.Email, u.PasswordHash, u.Name, u.Role, u.TipeUser, u.Status, u.ProvinsiID, u.KabupatenID)
	return mapDBError(err, "Email sudah dipakai akun lain")
}

func (r *userAdminRepo) ListAdmin(ctx context.Context, roles []string, status, search string, limit, offset int) ([]domain.User, int, error) {
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	where := []string{"u.deleted_at IS NULL", "u.role <> 'USER'"}
	args := []interface{}{}
	clean := make([]string, 0, len(roles))
	for _, rl := range roles {
		if s := strings.TrimSpace(rl); s != "" {
			clean = append(clean, s)
		}
	}
	if len(clean) > 0 {
		ph := make([]string, len(clean))
		for i, rl := range clean {
			args = append(args, rl)
			ph[i] = fmt.Sprintf("$%d", len(args))
		}
		where = append(where, "u.role IN ("+strings.Join(ph, ",")+")")
	}
	if v := strings.TrimSpace(status); v != "" {
		args = append(args, v)
		where = append(where, fmt.Sprintf("u.status = $%d", len(args)))
	}
	if q := strings.TrimSpace(search); q != "" {
		args = append(args, "%"+q+"%")
		where = append(where, fmt.Sprintf("(u.name ILIKE $%d OR u.email ILIKE $%d)", len(args), len(args)))
	}
	base := strings.Join(where, " AND ")

	var total int
	if err := r.db.GetContext(ctx, &total, `SELECT COUNT(*) FROM users u WHERE `+base, args...); err != nil {
		return nil, 0, err
	}
	items := make([]domain.User, 0)
	listArgs := append(append([]interface{}{}, args...), limit, offset)
	q := selectUserSQL + ` WHERE ` + base +
		fmt.Sprintf(` ORDER BY u.created_at DESC LIMIT $%d OFFSET $%d`, len(args)+1, len(args)+2)
	if err := r.db.SelectContext(ctx, &items, q, listArgs...); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *userAdminRepo) Counts(ctx context.Context) (*domain.AdminUserCounts, error) {
	var out domain.AdminUserCounts
	q := `SELECT COUNT(*) AS total,
		COUNT(*) FILTER (WHERE role = 'SUPER_ADMIN') AS super,
		COUNT(*) FILTER (WHERE role = 'ADMIN_NASIONAL') AS nasional,
		COUNT(*) FILTER (WHERE role = 'ADMIN_PROVINSI') AS provinsi,
		COUNT(*) FILTER (WHERE role = 'ADMIN_KABUPATEN') AS kabupaten
		FROM users WHERE deleted_at IS NULL AND role <> 'USER'`
	if err := r.db.GetContext(ctx, &out, q); err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *userAdminRepo) UpdateAdminProfile(ctx context.Context, id, name, email string, role domain.Role, provinsiID, kabupatenID *int, status domain.UserStatus) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE users
		 SET name = $2, email = $3, role = $4, provinsi_id = $5, kabupaten_id = $6, status = $7, updated_at = CURRENT_TIMESTAMP
		 WHERE id = $1 AND deleted_at IS NULL`,
		id, strings.TrimSpace(name), strings.TrimSpace(email), role, provinsiID, kabupatenID, status)
	if err != nil {
		return mapDBError(err, "Email sudah dipakai akun lain")
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}

func (r *userAdminRepo) SoftDeleteAdmin(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE users SET deleted_at = CURRENT_TIMESTAMP, status = 'Nonaktif', updated_at = CURRENT_TIMESTAMP
		 WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}

func (r *userAdminRepo) CountActiveSuperAdmins(ctx context.Context, excludeID string) (int, error) {
	var n int
	err := r.db.GetContext(ctx, &n,
		`SELECT COUNT(*) FROM users
		 WHERE role = 'SUPER_ADMIN' AND status = 'Aktif' AND deleted_at IS NULL AND id <> $1`, excludeID)
	return n, err
}
