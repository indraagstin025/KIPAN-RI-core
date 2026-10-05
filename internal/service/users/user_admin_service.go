package users

import (
	"context"
	"strings"

	"github.com/alexedwards/argon2id"
	"github.com/google/uuid"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
	"github.com/kipan-indonesia/sim-kipan-core/internal/service/svcutil"
)

// UserMutationResult hasil mutasi akun admin. Password hanya terisi saat
// akun baru dibuat atau password di-reset (ditampilkan SEKALI).
type UserMutationResult struct {
	User     *domain.User `json:"user"`
	Password string       `json:"password,omitempty"`
}

// UserAdminService manajemen akun admin (Super Admin saja).
type UserAdminService interface {
	List(ctx context.Context, actor domain.ActorContext, role, status, search string, page, limit int) ([]domain.User, int, error)
	Counts(ctx context.Context, actor domain.ActorContext) (*domain.AdminUserCounts, error)
	Create(ctx context.Context, actor domain.ActorContext, audit domain.AuditContext, in domain.UserCreateRequest) (*UserMutationResult, error)
	Update(ctx context.Context, actor domain.ActorContext, audit domain.AuditContext, id string, in domain.UserUpdateRequest) (*UserMutationResult, error)
	Delete(ctx context.Context, actor domain.ActorContext, audit domain.AuditContext, id string) error
}

// UserAdminDeps dependensi service manajemen pengguna.
type UserAdminDeps struct {
	UserRepo    repository.UserAccountRepository
	AdminRepo   repository.UserAdminRepository
	WilayahRepo repository.WilayahRepository
	AuditRepo   repository.AuditLogRepository
}

type userAdminSvc struct {
	userRepo    repository.UserAccountRepository
	adminRepo   repository.UserAdminRepository
	wilayahRepo repository.WilayahRepository
	auditRepo   repository.AuditLogRepository
}

func NewUserAdminService(deps UserAdminDeps) UserAdminService {
	return &userAdminSvc{
		userRepo: deps.UserRepo, adminRepo: deps.AdminRepo,
		wilayahRepo: deps.WilayahRepo, auditRepo: deps.AuditRepo,
	}
}

func (s *userAdminSvc) guard(actor domain.ActorContext) error {
	if actor.Role != domain.RoleSuperAdmin && actor.Role != domain.RoleAdminNasional {
		return domain.NewForbiddenError("Manajemen Pengguna hanya untuk Super Admin / Admin Nasional")
	}
	if s.adminRepo == nil || s.userRepo == nil {
		return svcutil.Unavailable("pengguna")
	}
	return nil
}

// regionalRestricted true bila aktor hanya boleh mengelola akun Provinsi/Kab.
func regionalRestricted(actor domain.ActorContext) bool {
	return actor.Role == domain.RoleAdminNasional
}

// assertManageableRole memastikan aktor berwenang atas role target.
func assertManageableRole(actor domain.ActorContext, target domain.Role) error {
	if regionalRestricted(actor) &&
		target != domain.RoleAdminProvinsi && target != domain.RoleAdminKabupaten {
		return domain.NewForbiddenError("Admin Nasional hanya dapat mengelola akun Provinsi/Kabupaten")
	}
	return nil
}

// listRoles menentukan filter role untuk daftar akun sesuai peran aktor.
func listRoles(actor domain.ActorContext, requested string) ([]string, error) {
	rq := strings.ToUpper(strings.TrimSpace(requested))
	if actor.Role == domain.RoleSuperAdmin {
		if rq == "" {
			return nil, nil
		}
		return []string{rq}, nil
	}
	switch rq {
	case "":
		return []string{string(domain.RoleAdminProvinsi), string(domain.RoleAdminKabupaten)}, nil
	case string(domain.RoleAdminProvinsi), string(domain.RoleAdminKabupaten):
		return []string{rq}, nil
	default:
		return nil, domain.NewForbiddenError("Admin Nasional hanya dapat melihat akun Provinsi/Kabupaten")
	}
}

func normAdminRole(v string) (domain.Role, bool) {
	r := domain.Role(strings.ToUpper(strings.TrimSpace(v)))
	switch r {
	case domain.RoleSuperAdmin, domain.RoleAdminNasional, domain.RoleAdminProvinsi, domain.RoleAdminKabupaten:
		return r, true
	}
	return "", false
}

func normUserStatus(v string) (domain.UserStatus, bool) {
	switch strings.TrimSpace(v) {
	case "", "Aktif":
		return domain.UserStatusAktif, true
	case "Nonaktif":
		return domain.UserStatusNonaktif, true
	}
	return "", false
}

func validAdminEmail(e string) bool {
	if len(e) < 5 || len(e) > 255 || strings.ContainsAny(e, " \t<>") {
		return false
	}
	at := strings.IndexByte(e, '@')
	return at > 0 && at < len(e)-3
}

// resolveScope menegakkan konsistensi role ↔ wilayah (server-side).
func (s *userAdminSvc) resolveScope(ctx context.Context, role domain.Role, prov, kab *int) (*int, *int, error) {
	switch role {
	case domain.RoleSuperAdmin, domain.RoleAdminNasional:
		return nil, nil, nil
	case domain.RoleAdminProvinsi:
		if prov == nil || *prov <= 0 {
			return nil, nil, domain.NewValidationError("Provinsi wajib dipilih untuk Admin Provinsi")
		}
		if s.wilayahRepo == nil {
			return nil, nil, svcutil.Unavailable("wilayah")
		}
		ok, err := s.wilayahRepo.ExistsProvinsi(ctx, *prov)
		if err != nil {
			return nil, nil, err
		}
		if !ok {
			return nil, nil, domain.NewValidationError("Provinsi tidak valid / tidak aktif")
		}
		return prov, nil, nil
	case domain.RoleAdminKabupaten:
		if prov == nil || kab == nil || *prov <= 0 || *kab <= 0 {
			return nil, nil, domain.NewValidationError("Provinsi & Kabupaten/Kota wajib dipilih untuk Admin Kabupaten")
		}
		if s.wilayahRepo == nil {
			return nil, nil, svcutil.Unavailable("wilayah")
		}
		ok, err := s.wilayahRepo.KabupatenInProvinsi(ctx, *kab, *prov)
		if err != nil {
			return nil, nil, err
		}
		if !ok {
			return nil, nil, domain.NewValidationError("Kabupaten/Kota tidak valid untuk provinsi tersebut")
		}
		return prov, kab, nil
	}
	return nil, nil, domain.NewValidationError("Role admin tidak valid")
}

func (s *userAdminSvc) List(ctx context.Context, actor domain.ActorContext, role, status, search string, page, limit int) ([]domain.User, int, error) {
	if err := s.guard(actor); err != nil {
		return nil, 0, err
	}
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}
	roles, err := listRoles(actor, role)
	if err != nil {
		return nil, 0, err
	}
	return s.adminRepo.ListAdmin(ctx, roles, status, search, limit, (page-1)*limit)
}

func (s *userAdminSvc) Counts(ctx context.Context, actor domain.ActorContext) (*domain.AdminUserCounts, error) {
	if err := s.guard(actor); err != nil {
		return nil, err
	}
	return s.adminRepo.Counts(ctx)
}

func (s *userAdminSvc) Create(ctx context.Context, actor domain.ActorContext, audit domain.AuditContext, in domain.UserCreateRequest) (*UserMutationResult, error) {
	if err := s.guard(actor); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(in.Name)
	if len(name) < 2 || len(name) > 150 {
		return nil, domain.NewValidationError("Nama wajib 2-150 karakter")
	}
	email := strings.ToLower(strings.TrimSpace(in.Email))
	if !validAdminEmail(email) {
		return nil, domain.NewValidationError("Format email tidak valid")
	}
	role, ok := normAdminRole(in.Role)
	if !ok {
		return nil, domain.NewValidationError("Role admin tidak valid")
	}
	if err := assertManageableRole(actor, role); err != nil {
		return nil, err
	}
	status, ok := normUserStatus(in.Status)
	if !ok {
		return nil, domain.NewValidationError("Status akun tidak valid")
	}
	prov, kab, err := s.resolveScope(ctx, role, in.ProvinsiID, in.KabupatenID)
	if err != nil {
		return nil, err
	}

	password, err := svcutil.GenerateMemberPassword(16)
	if err != nil {
		return nil, err
	}
	hash, err := argon2id.CreateHash(password, svcutil.Argon2Params)
	if err != nil {
		return nil, err
	}
	u := &domain.User{
		ID: uuid.NewString(), Email: email, PasswordHash: hash, Name: name,
		Role: role, TipeUser: domain.UserTipeAdmin, Status: status,
		ProvinsiID: prov, KabupatenID: kab,
	}
	if err := s.adminRepo.CreateAdmin(ctx, u); err != nil {
		return nil, err
	}
	meta := `{"event":"admin_user_create","role":"` + string(role) + `"}`
	s.audit(ctx, audit, actor, "users", u.ID, "CREATE", &meta)
	created, err := s.userRepo.GetByID(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	return &UserMutationResult{User: created, Password: password}, nil
}

func (s *userAdminSvc) Update(ctx context.Context, actor domain.ActorContext, audit domain.AuditContext, id string, in domain.UserUpdateRequest) (*UserMutationResult, error) {
	if err := s.guard(actor); err != nil {
		return nil, err
	}
	if strings.TrimSpace(id) == "" {
		return nil, domain.NewValidationError("ID pengguna tidak valid")
	}
	existing, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(in.Name)
	if len(name) < 2 || len(name) > 150 {
		return nil, domain.NewValidationError("Nama wajib 2-150 karakter")
	}
	email := strings.ToLower(strings.TrimSpace(in.Email))
	if !validAdminEmail(email) {
		return nil, domain.NewValidationError("Format email tidak valid")
	}
	role, ok := normAdminRole(in.Role)
	if !ok {
		return nil, domain.NewValidationError("Role admin tidak valid")
	}
	if err := assertManageableRole(actor, existing.Role); err != nil {
		return nil, err
	}
	if err := assertManageableRole(actor, role); err != nil {
		return nil, err
	}
	status, ok := normUserStatus(in.Status)
	if !ok {
		return nil, domain.NewValidationError("Status akun tidak valid")
	}
	prov, kab, err := s.resolveScope(ctx, role, in.ProvinsiID, in.KabupatenID)
	if err != nil {
		return nil, err
	}

	// Cegah lockout: tidak boleh menonaktifkan / menurunkan role akun sendiri.
	if id == actor.UserID {
		if status != domain.UserStatusAktif {
			return nil, domain.NewForbiddenError("Tidak dapat menonaktifkan akun Anda sendiri")
		}
		if role != existing.Role {
			return nil, domain.NewForbiddenError("Tidak dapat mengubah role akun Anda sendiri")
		}
	}
	// Cegah kehilangan Super Admin terakhir.
	if existing.Role == domain.RoleSuperAdmin && (role != domain.RoleSuperAdmin || status != domain.UserStatusAktif) {
		n, err := s.adminRepo.CountActiveSuperAdmins(ctx, id)
		if err != nil {
			return nil, err
		}
		if n == 0 {
			return nil, domain.NewConflictError("Tidak dapat menonaktifkan/menurunkan Super Admin aktif terakhir")
		}
	}

	if err := s.adminRepo.UpdateAdminProfile(ctx, id, name, email, role, prov, kab, status); err != nil {
		return nil, err
	}

	res := &UserMutationResult{}
	// Password: minimal salah satu dari password eksplisit / reset.
	if in.ResetPassword || strings.TrimSpace(in.Password) != "" {
		pw := in.Password
		if in.ResetPassword || pw == "" {
			pw, err = svcutil.GenerateMemberPassword(16)
			if err != nil {
				return nil, err
			}
			res.Password = pw
		} else if len(pw) < 8 {
			return nil, domain.NewValidationError("Password minimal 8 karakter")
		}
		hash, err := argon2id.CreateHash(pw, svcutil.Argon2Params)
		if err != nil {
			return nil, err
		}
		if err := s.userRepo.UpdatePassword(ctx, id, hash); err != nil {
			return nil, err
		}
		_ = s.userRepo.RevokeAllUserTokens(ctx, id)
	}
	// Cabut sesi saat role berubah atau status jadi Nonaktif.
	if existing.Role != role || (status == domain.UserStatusNonaktif && existing.Status != domain.UserStatusNonaktif) {
		_ = s.userRepo.RevokeAllUserTokens(ctx, id)
	}

	meta := `{"event":"admin_user_update","role":"` + string(role) + `","status":"` + string(status) + `"}`
	s.audit(ctx, audit, actor, "users", id, "UPDATE", &meta)
	updated, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	res.User = updated
	return res, nil
}

func (s *userAdminSvc) Delete(ctx context.Context, actor domain.ActorContext, audit domain.AuditContext, id string) error {
	if err := s.guard(actor); err != nil {
		return err
	}
	if strings.TrimSpace(id) == "" {
		return domain.NewValidationError("ID pengguna tidak valid")
	}
	if id == actor.UserID {
		return domain.NewForbiddenError("Tidak dapat menghapus akun Anda sendiri")
	}
	existing, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if err := assertManageableRole(actor, existing.Role); err != nil {
		return err
	}
	if existing.Role == domain.RoleSuperAdmin {
		n, err := s.adminRepo.CountActiveSuperAdmins(ctx, id)
		if err != nil {
			return err
		}
		if n == 0 {
			return domain.NewConflictError("Tidak dapat menghapus Super Admin aktif terakhir")
		}
	}
	if err := s.adminRepo.SoftDeleteAdmin(ctx, id); err != nil {
		return err
	}
	_ = s.userRepo.RevokeAllUserTokens(ctx, id)
	meta := `{"event":"admin_user_delete"}`
	s.audit(ctx, audit, actor, "users", id, "DELETE", &meta)
	return nil
}

func (s *userAdminSvc) audit(ctx context.Context, tr domain.AuditContext, actor domain.ActorContext, entity, entityID, action string, metadata *string) {
	id := actor.UserID
	svcutil.WriteAudit(ctx, s.auditRepo, tr, &id, actor.Name, string(actor.Role), entity, entityID, action, metadata)
}
