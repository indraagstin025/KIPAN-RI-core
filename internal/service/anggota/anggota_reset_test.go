package anggota

// Uji Batch 1 — T1: reset password akun USER anggota.

import (
	"context"
	"testing"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/service/testutil"
)

func resetFixture() (*domain.Anggota, *domain.User, *testutil.FakeAnggotaRepo, *testutil.FakeMemberUserRepo) {
	uid := "user-kader-1"
	prov, kab := 32, 3273
	member := &domain.Anggota{
		ID: 21, NIA: "KIPAN-IND-3273-2026-000021", NamaLengkap: "Kader Uji",
		ProvinsiID: prov, KabupatenID: kab, UserID: &uid,
	}
	user := &domain.User{ID: uid, Email: "kader@example.com", Role: domain.RoleUser, Status: domain.UserStatusAktif}
	anggotaRepo := &testutil.FakeAnggotaRepo{ByID: map[int]*domain.Anggota{member.ID: member}}
	userRepo := &testutil.FakeMemberUserRepo{ByID: map[string]*domain.User{uid: user}}
	return member, user, anggotaRepo, userRepo
}

func superActor32() domain.ActorContext {
	prov, kab := 32, 3273
	return domain.ActorContext{UserID: "admin-1", Name: "Admin", Role: domain.RoleSuperAdmin, ProvinsiID: &prov, KabupatenID: &kab}
}

func TestResetMemberPasswordSukses(t *testing.T) {
	member, user, anggotaRepo, userRepo := resetFixture()
	outbox := &testutil.FakeOutboxRepo{}
	svc := NewAnggotaService(nil, AnggotaDeps{AnggotaRepo: anggotaRepo, UserRepo: userRepo, OutboxRepo: outbox})

	pw, err := svc.ResetMemberPassword(context.Background(), member.ID, superActor32(), domain.AuditContext{})
	if err != nil {
		t.Fatalf("reset gagal: %v", err)
	}
	if pw != "" {
		t.Fatalf("tidak boleh mengembalikan password plaintext, dapat %q", pw)
	}
	if user.PasswordHash == "" {
		t.Fatal("hash password baru harus tersimpan")
	}
	if len(userRepo.Revoked) != 1 || userRepo.Revoked[0] != user.ID {
		t.Fatalf("seluruh sesi anggota harus dicabut, dapat %v", userRepo.Revoked)
	}
	// Opsi A: kirim tautan set-password via antrian.
	if len(outbox.Enqueued) != 1 || outbox.Enqueued[0].Jenis != domain.EmailOutboxSetPassword {
		t.Fatalf("harap 1 outbox SET_PASSWORD, dapat %+v", outbox.Enqueued)
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
	if userRepo.LastHash != "" {
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
	if userRepo.LastHash != "" {
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
