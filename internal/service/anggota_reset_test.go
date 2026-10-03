package service

// Uji Batch 1 — T1: reset password akun USER anggota.

import (
	"context"
	"testing"

	"github.com/alexedwards/argon2id"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
)

func resetFixture() (*domain.Anggota, *domain.User, *fakeAnggotaRepo, *fakeMemberUserRepo) {
	uid := "user-kader-1"
	prov, kab := 32, 3273
	member := &domain.Anggota{
		ID: 21, NIA: "KIPAN-IND-3273-2026-000021", NamaLengkap: "Kader Uji",
		ProvinsiID: prov, KabupatenID: kab, UserID: &uid,
	}
	user := &domain.User{ID: uid, Email: "kader@example.com", Role: domain.RoleUser, Status: domain.UserStatusAktif}
	anggotaRepo := &fakeAnggotaRepo{byID: map[int]*domain.Anggota{member.ID: member}}
	userRepo := &fakeMemberUserRepo{byID: map[string]*domain.User{uid: user}}
	return member, user, anggotaRepo, userRepo
}

func superActor32() domain.ActorContext {
	prov, kab := 32, 3273
	return domain.ActorContext{UserID: "admin-1", Name: "Admin", Role: domain.RoleSuperAdmin, ProvinsiID: &prov, KabupatenID: &kab}
}

func TestResetMemberPasswordSukses(t *testing.T) {
	member, user, anggotaRepo, userRepo := resetFixture()
	svc := NewAnggotaService(nil, AnggotaDeps{AnggotaRepo: anggotaRepo, UserRepo: userRepo})

	pw, err := svc.ResetMemberPassword(context.Background(), member.ID, superActor32(), domain.AuditContext{})
	if err != nil {
		t.Fatalf("reset gagal: %v", err)
	}
	if len(pw) < 12 {
		t.Fatalf("password terlalu pendek: %d", len(pw))
	}
	match, err := argon2id.ComparePasswordAndHash(pw, user.PasswordHash)
	if err != nil || !match {
		t.Fatal("password baru tidak cocok dengan hash tersimpan")
	}
	if len(userRepo.revoked) != 1 || userRepo.revoked[0] != user.ID {
		t.Fatalf("seluruh sesi anggota harus dicabut, dapat %v", userRepo.revoked)
	}
}

func TestResetMemberPasswordDiluarWilayah(t *testing.T) {
	member, _, anggotaRepo, userRepo := resetFixture()
	svc := NewAnggotaService(nil, AnggotaDeps{AnggotaRepo: anggotaRepo, UserRepo: userRepo})

	provLain := 33
	actor := domain.ActorContext{UserID: "admin-2", Role: domain.RoleAdminProvinsi, ProvinsiID: &provLain}
	if _, err := svc.ResetMemberPassword(context.Background(), member.ID, actor, domain.AuditContext{}); err == nil {
		t.Fatal("reset lintas wilayah harus ditolak")
	}
	if userRepo.lastHash != "" {
		t.Fatal("password tidak boleh berubah saat otorisasi gagal")
	}
}

func TestResetMemberPasswordTanpaAkun(t *testing.T) {
	member, _, anggotaRepo, userRepo := resetFixture()
	member.UserID = nil
	svc := NewAnggotaService(nil, AnggotaDeps{AnggotaRepo: anggotaRepo, UserRepo: userRepo})

	_, err := svc.ResetMemberPassword(context.Background(), member.ID, superActor32(), domain.AuditContext{})
	appErr, ok := err.(*domain.AppError)
	if !ok || appErr.Code != 409 {
		t.Fatalf("harap 409 saat anggota tanpa akun, dapat %v", err)
	}
}

func TestResetMemberPasswordTolakAkunNonUser(t *testing.T) {
	member, user, anggotaRepo, userRepo := resetFixture()
	user.Role = domain.RoleAdminKabupaten
	svc := NewAnggotaService(nil, AnggotaDeps{AnggotaRepo: anggotaRepo, UserRepo: userRepo})

	_, err := svc.ResetMemberPassword(context.Background(), member.ID, superActor32(), domain.AuditContext{})
	appErr, ok := err.(*domain.AppError)
	if !ok || appErr.Code != 409 {
		t.Fatalf("harap 409 saat akun terhubung bukan USER, dapat %v", err)
	}
	if userRepo.lastHash != "" {
		t.Fatal("password akun non-USER tidak boleh diubah lewat jalur anggota")
	}
}

func TestResetMemberPasswordFailClosedTanpaUserRepo(t *testing.T) {
	member, _, anggotaRepo, _ := resetFixture()
	svc := NewAnggotaService(nil, AnggotaDeps{AnggotaRepo: anggotaRepo})

	if _, err := svc.ResetMemberPassword(context.Background(), member.ID, superActor32(), domain.AuditContext{}); err == nil {
		t.Fatal("tanpa UserRepo harus gagal fail-closed")
	}
}
