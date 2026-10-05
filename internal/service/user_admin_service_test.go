package service

import (
	"context"
	"testing"
	"time"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
	"github.com/kipan-indonesia/sim-kipan-core/internal/service/testutil"
)

// fakeUserAdmin memenuhi UserRepository + UserAdminRepository sekaligus
// (in-memory) agar jalur Create→GetByID konsisten dalam satu struct.
type fakeUserAdmin struct {
	repository.UserRepository
	repository.UserAdminRepository
	byID         map[string]*domain.User
	revoked      []string
	lastHash     string
	deleted      string
	activeSupers int
	forceConfirm bool
}

func newFakeUserAdmin() *fakeUserAdmin { return &fakeUserAdmin{byID: map[string]*domain.User{}} }

func (f *fakeUserAdmin) GetByID(_ context.Context, id string) (*domain.User, error) {
	if u, ok := f.byID[id]; ok && u != nil && u.DeletedAt == nil {
		return u, nil
	}
	return nil, domain.ErrUserNotFound
}
func (f *fakeUserAdmin) UpdatePassword(_ context.Context, id, hash string) error {
	f.lastHash = hash
	if u := f.byID[id]; u != nil {
		u.PasswordHash = hash
	}
	return nil
}
func (f *fakeUserAdmin) RevokeAllUserTokens(_ context.Context, id string) error {
	f.revoked = append(f.revoked, id)
	return nil
}
func (f *fakeUserAdmin) CreateAdmin(_ context.Context, u *domain.User) error {
	if f.forceConfirm {
		return domain.NewConflictError("Email sudah dipakai akun lain")
	}
	f.byID[u.ID] = u
	return nil
}
func (f *fakeUserAdmin) UpdateAdminProfile(_ context.Context, id, name, email string, role domain.Role, prov, kab *int, status domain.UserStatus) error {
	if f.forceConfirm {
		return domain.NewConflictError("Email sudah dipakai akun lain")
	}
	u := f.byID[id]
	if u == nil {
		return domain.ErrUserNotFound
	}
	u.Name, u.Email, u.Role, u.ProvinsiID, u.KabupatenID, u.Status = name, email, role, prov, kab, status
	return nil
}
func (f *fakeUserAdmin) SoftDeleteAdmin(_ context.Context, id string) error {
	f.deleted = id
	if u := f.byID[id]; u != nil {
		now := time.Now()
		u.DeletedAt = &now
	}
	return nil
}
func (f *fakeUserAdmin) ListAdmin(context.Context, []string, string, string, int, int) ([]domain.User, int, error) {
	return nil, 0, nil
}
func (f *fakeUserAdmin) Counts(context.Context) (*domain.AdminUserCounts, error) {
	return &domain.AdminUserCounts{Total: len(f.byID)}, nil
}
func (f *fakeUserAdmin) CountActiveSuperAdmins(context.Context, string) (int, error) {
	return f.activeSupers, nil
}

func userAdminSvcFor(f *fakeUserAdmin) UserAdminService {
	return NewUserAdminService(UserAdminDeps{UserRepo: f, AdminRepo: f, WilayahRepo: &testutil.FakeWilayahRepo{}})
}

func TestUserAdminCreate(t *testing.T) {
	ctx := context.Background()
	f := newFakeUserAdmin()
	svc := userAdminSvcFor(f)

	// Super → tanpa wilayah, email dinormalisasi, tipe ADMIN, password sekali.
	res, err := svc.Create(ctx, testutil.SuperActor(), domain.AuditContext{},
		domain.UserCreateRequest{Name: "Budi", Email: "Budi@KIPAN.id", Role: "ADMIN_NASIONAL"})
	if err != nil {
		t.Fatalf("Create gagal: %v", err)
	}
	if res.Password == "" || res.User.TipeUser != domain.UserTipeAdmin || res.User.Email != "budi@kipan.id" {
		t.Fatalf("hasil tak sesuai: %+v", res)
	}

	// Provinsi tanpa provinsi → validasi.
	if _, err := svc.Create(ctx, testutil.SuperActor(), domain.AuditContext{},
		domain.UserCreateRequest{Name: "Ani", Email: "ani@kipan.id", Role: "ADMIN_PROVINSI"}); err == nil {
		t.Fatal("Admin Provinsi tanpa provinsi seharusnya ditolak")
	}
	// Email buruk → validasi.
	if _, err := svc.Create(ctx, testutil.SuperActor(), domain.AuditContext{},
		domain.UserCreateRequest{Name: "Ani", Email: "bukan-email", Role: "ADMIN_NASIONAL"}); err == nil {
		t.Fatal("email buruk seharusnya ditolak")
	}
	// Admin Nasional TIDAK boleh membuat akun Nasional/Super.
	if _, err := svc.Create(ctx, testutil.NasActor(), domain.AuditContext{},
		domain.UserCreateRequest{Name: "X", Email: "x@kipan.id", Role: "ADMIN_NASIONAL"}); err == nil {
		t.Fatal("Nasional tidak boleh membuat akun Nasional")
	}
	// Admin Nasional BOLEH membuat akun Provinsi/Kabupaten.
	prov := 32
	if _, err := svc.Create(ctx, testutil.NasActor(), domain.AuditContext{},
		domain.UserCreateRequest{Name: "Prov", Email: "prov@kipan.id", Role: "ADMIN_PROVINSI", ProvinsiID: &prov}); err != nil {
		t.Fatalf("Nasional harus boleh membuat akun Provinsi: %v", err)
	}
	// Provinsi tidak boleh mengelola pengguna.
	if _, err := svc.Create(ctx, testutil.ProvActor(), domain.AuditContext{},
		domain.UserCreateRequest{Name: "Y", Email: "y@kipan.id", Role: "ADMIN_KABUPATEN"}); err == nil {
		t.Fatal("Admin Provinsi tidak boleh mengelola pengguna")
	}
}

func TestUserAdminLockoutSelf(t *testing.T) {
	ctx := context.Background()
	f := newFakeUserAdmin()
	f.byID["super-1"] = &domain.User{ID: "super-1", Name: "Me", Email: "me@kipan.id", Role: domain.RoleSuperAdmin, Status: domain.UserStatusAktif}
	actor := domain.ActorContext{UserID: "super-1", Name: "Me", Role: domain.RoleSuperAdmin}
	svc := userAdminSvcFor(f)

	if _, err := svc.Update(ctx, actor, domain.AuditContext{}, "super-1",
		domain.UserUpdateRequest{Name: "Me", Email: "me@kipan.id", Role: "SUPER_ADMIN", Status: "Nonaktif"}); err == nil {
		t.Fatal("menonaktifkan diri seharusnya ditolak")
	}
	if _, err := svc.Update(ctx, actor, domain.AuditContext{}, "super-1",
		domain.UserUpdateRequest{Name: "Me", Email: "me@kipan.id", Role: "ADMIN_NASIONAL", Status: "Aktif"}); err == nil {
		t.Fatal("mengubah role diri seharusnya ditolak")
	}
	if err := svc.Delete(ctx, actor, domain.AuditContext{}, "super-1"); err == nil {
		t.Fatal("menghapus diri seharusnya ditolak")
	}
}

func TestUserAdminLastSuper(t *testing.T) {
	ctx := context.Background()
	f := newFakeUserAdmin()
	f.byID["super-2"] = &domain.User{ID: "super-2", Name: "Other", Email: "o@kipan.id", Role: domain.RoleSuperAdmin, Status: domain.UserStatusAktif}
	f.activeSupers = 0
	actor := domain.ActorContext{UserID: "super-1", Role: domain.RoleSuperAdmin}
	svc := userAdminSvcFor(f)

	if _, err := svc.Update(ctx, actor, domain.AuditContext{}, "super-2",
		domain.UserUpdateRequest{Name: "Other", Email: "o@kipan.id", Role: "ADMIN_NASIONAL", Status: "Aktif"}); err == nil {
		t.Fatal("menurunkan Super Admin terakhir seharusnya ditolak")
	}
	if err := svc.Delete(ctx, actor, domain.AuditContext{}, "super-2"); err == nil {
		t.Fatal("menghapus Super Admin terakhir seharusnya ditolak")
	}
}

func TestUserAdminResetPasswordRevokes(t *testing.T) {
	ctx := context.Background()
	f := newFakeUserAdmin()
	f.byID["u-9"] = &domain.User{ID: "u-9", Name: "Xyz", Email: "x@kipan.id", Role: domain.RoleAdminNasional, Status: domain.UserStatusAktif}
	svc := userAdminSvcFor(f)

	res, err := svc.Update(ctx, testutil.SuperActor(), domain.AuditContext{}, "u-9",
		domain.UserUpdateRequest{Name: "Xyz", Email: "x@kipan.id", Role: "ADMIN_NASIONAL", Status: "Aktif", ResetPassword: true})
	if err != nil {
		t.Fatalf("Update reset gagal: %v", err)
	}
	if res.Password == "" {
		t.Fatal("password reset harus dikembalikan sekali")
	}
	if len(f.revoked) == 0 {
		t.Fatal("sesi harus dicabut setelah reset password")
	}
}
