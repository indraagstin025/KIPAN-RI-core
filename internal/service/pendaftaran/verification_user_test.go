package pendaftaran

// Uji Batch 2: penerbitan akun USER saat approve + KTA mandiri.

import (
	"context"
	"testing"
	"time"

	"github.com/kipan-indonesia/sim-kipan-core/config"
	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
	"github.com/kipan-indonesia/sim-kipan-core/internal/service/kta"
	"github.com/kipan-indonesia/sim-kipan-core/internal/service/testutil"
)

// fakeApproveRepo melayani GetByID + IssueMember untuk jalur SETUJI.
type fakeApproveRepo struct {
	repository.PendaftaranRepository
	item   *domain.Pendaftaran
	member *domain.Anggota
}

func (f *fakeApproveRepo) GetByID(_ context.Context, _ int) (*domain.Pendaftaran, error) {
	if f.item == nil {
		return nil, domain.ErrNotFound
	}
	return f.item, nil
}

func (f *fakeApproveRepo) IssueMember(_ context.Context, _ int, _ int, _ string) (*domain.Anggota, error) {
	if f.member == nil {
		return nil, domain.ErrNotFound
	}
	return f.member, nil
}

// fakeApproveKTASvc melewati render PDF (bukan fokus uji ini).
type fakeApproveKTASvc struct {
	kta.KTAService
}

func (f *fakeApproveKTASvc) IssueKTADocument(_ context.Context, _ *domain.Anggota, _ string, _ domain.AuditContext) (string, error) {
	return "kta/test.pdf", nil
}

const testKTASigningKey = "aa00112233445566778899aabbccddeeffaa00112233445566778899aabbccdd"

func approveFixture() (*domain.Pendaftaran, *domain.Anggota) {
	item := &domain.Pendaftaran{
		ID:          11,
		Status:      domain.PendaftaranStatusDiverifikasi,
		NamaLengkap: "Calon Kader",
		Email:       "calon@example.com",
		ProvinsiID:  32,
		KabupatenID: 3273,
	}
	member := &domain.Anggota{
		ID:            21,
		NIA:           "KIPAN-IND-3273-2026-000021",
		NamaLengkap:   "Calon Kader",
		Email:         "calon@example.com",
		ProvinsiID:    32,
		KabupatenID:   3273,
		TanggalAngkat: time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC),
	}
	return item, member
}

func approveSvc(repo *fakeApproveRepo, users *testutil.FakeMemberUserRepo, anggota *testutil.FakeAnggotaRepo, outbox repository.EmailOutboxRepository) VerificationService {
	cfg := &config.Config{}
	cfg.Crypto.KTASigningKey = testKTASigningKey
	return NewVerificationService(cfg, VerificationDeps{
		Repo: repo, AnggotaRepo: anggota, UserRepo: users, KTASvc: &fakeApproveKTASvc{},
		OutboxRepo: outbox,
	})
}

func TestApproveCreatesUserAccount(t *testing.T) {
	item, member := approveFixture()
	repo := &fakeApproveRepo{item: item, member: member}
	users := &testutil.FakeMemberUserRepo{}
	anggota := &testutil.FakeAnggotaRepo{ByID: map[int]*domain.Anggota{member.ID: member}}
	outbox := &testutil.FakeOutboxRepo{}
	svc := approveSvc(repo, users, anggota, outbox)

	res, err := svc.ProcessApproval(context.Background(), item.ID, domain.PendaftaranActionSetujui, "", testutil.SuperActor(), domain.AuditContext{})
	if err != nil {
		t.Fatalf("approve gagal: %v", err)
	}
	if res == nil || res.NIA != member.NIA {
		t.Fatalf("harap NIA %q, dapat %+v", member.NIA, res)
	}

	u, err := users.GetByEmail(context.Background(), member.Email)
	if err != nil {
		t.Fatalf("akun USER tidak tercipta: %v", err)
	}
	if u.Role != domain.RoleUser || u.TipeUser != domain.UserTipeKader || u.Status != domain.UserStatusAktif {
		t.Fatalf("akun salah: %+v", u)
	}
	if u.PasswordHash == "" {
		t.Fatal("harap hash password tersimpan (walau tak ditampilkan)")
	}
	if got := anggota.Links[member.ID]; got != u.ID {
		t.Fatalf("anggota tidak terhubung ke akun: %q", got)
	}
	// Opsi A: kredensial via antrian SET_PASSWORD (bukan plaintext).
	if len(outbox.Enqueued) != 1 || outbox.Enqueued[0].Jenis != domain.EmailOutboxSetPassword {
		t.Fatalf("harap 1 outbox SET_PASSWORD, dapat %+v", outbox.Enqueued)
	}
	if outbox.Enqueued[0].UserID == nil || *outbox.Enqueued[0].UserID != u.ID {
		t.Fatalf("outbox harus menunjuk user_id akun baru")
	}
}

func TestApprovePengurusCreatesPengurusUser(t *testing.T) {
	item, member := approveFixture()
	member.Tipe = domain.TipePendaftaranPengurus
	repo := &fakeApproveRepo{item: item, member: member}
	users := &testutil.FakeMemberUserRepo{}
	anggota := &testutil.FakeAnggotaRepo{ByID: map[int]*domain.Anggota{member.ID: member}}
	outbox := &testutil.FakeOutboxRepo{}
	svc := approveSvc(repo, users, anggota, outbox)

	res, err := svc.ProcessApproval(context.Background(), item.ID, domain.PendaftaranActionSetujui, "", testutil.SuperActor(), domain.AuditContext{})
	if err != nil {
		t.Fatalf("approve gagal: %v", err)
	}
	if res == nil {
		t.Fatal("hasil nil")
	}
	u, err := users.GetByEmail(context.Background(), member.Email)
	if err != nil {
		t.Fatalf("akun USER tidak tercipta: %v", err)
	}
	if u.TipeUser != domain.UserTipePengurus {
		t.Fatalf("harap tipe PENGURUS mengikuti pendaftaran, dapat %q", u.TipeUser)
	}
	if len(outbox.Enqueued) != 1 || outbox.Enqueued[0].Jenis != domain.EmailOutboxSetPassword {
		t.Fatalf("harap outbox SET_PASSWORD, dapat %+v", outbox.Enqueued)
	}
}

// T6: perbaikan/penolakan wajib disertai catatan.
func TestApprovalWajibCatatan(t *testing.T) {
	for _, action := range []domain.PendaftaranApprovalAction{
		domain.PendaftaranActionPerbaikan,
		domain.PendaftaranActionTolak,
	} {
		item, member := approveFixture()
		repo := &fakeApproveRepo{item: item, member: member}
		svc := approveSvc(repo, &testutil.FakeMemberUserRepo{}, &testutil.FakeAnggotaRepo{}, &testutil.FakeOutboxRepo{})
		_, err := svc.ProcessApproval(context.Background(), item.ID, action, "   ", testutil.SuperActor(), domain.AuditContext{})
		appErr, ok := err.(*domain.AppError)
		if !ok || appErr.Code != 422 {
			t.Fatalf("action %s tanpa catatan harus 422, dapat %v", action, err)
		}
	}
}

// #3(a): email yang sudah dipakai akun non-USER tidak boleh ditautkan.
func TestApproveRejectsNonUserEmail(t *testing.T) {
	item, member := approveFixture()
	admin := &domain.User{ID: "admin-9", Email: member.Email, Role: domain.RoleAdminKabupaten, Status: domain.UserStatusAktif}
	repo := &fakeApproveRepo{item: item, member: member}
	users := &testutil.FakeMemberUserRepo{ByEmail: map[string]*domain.User{member.Email: admin}}
	anggota := &testutil.FakeAnggotaRepo{ByID: map[int]*domain.Anggota{member.ID: member}}
	svc := approveSvc(repo, users, anggota, &testutil.FakeOutboxRepo{})

	_, err := svc.ProcessApproval(context.Background(), item.ID, domain.PendaftaranActionSetujui, "", testutil.SuperActor(), domain.AuditContext{})
	appErr, ok := err.(*domain.AppError)
	if !ok || appErr.Code != 409 {
		t.Fatalf("harap 409 saat email milik akun non-USER, dapat %v", err)
	}
	if len(anggota.Links) != 0 {
		t.Fatalf("anggota tidak boleh ditautkan ke akun non-USER: %v", anggota.Links)
	}
}

func TestApproveLinksExistingUser(t *testing.T) {
	item, member := approveFixture()
	existing := &domain.User{ID: "user-lama", Email: member.Email, Role: domain.RoleUser, TipeUser: domain.UserTipeKader}
	repo := &fakeApproveRepo{item: item, member: member}
	users := &testutil.FakeMemberUserRepo{ByEmail: map[string]*domain.User{member.Email: existing}}
	anggota := &testutil.FakeAnggotaRepo{ByID: map[int]*domain.Anggota{member.ID: member}}
	outbox := &testutil.FakeOutboxRepo{}
	svc := approveSvc(repo, users, anggota, outbox)

	res, err := svc.ProcessApproval(context.Background(), item.ID, domain.PendaftaranActionSetujui, "", testutil.SuperActor(), domain.AuditContext{})
	if err != nil {
		t.Fatalf("approve gagal: %v", err)
	}
	if res == nil {
		t.Fatal("hasil nil")
	}
	if len(users.Created) != 0 {
		t.Fatalf("tidak boleh buat user baru, tercipta %d", len(users.Created))
	}
	if got := anggota.Links[member.ID]; got != existing.ID {
		t.Fatalf("anggota harus terhubung ke akun existing: %q", got)
	}
	// Akun tertaut: outbox AKUN_TERHUBUNG (tanpa set-password).
	if len(outbox.Enqueued) != 1 || outbox.Enqueued[0].Jenis != domain.EmailOutboxAkunTerhubung {
		t.Fatalf("harap 1 outbox AKUN_TERHUBUNG, dapat %+v", outbox.Enqueued)
	}
}

func TestApproveTanpaUserRepoGagalFailClosed(t *testing.T) {
	item, member := approveFixture()
	repo := &fakeApproveRepo{item: item, member: member}
	cfg := &config.Config{}
	cfg.Crypto.KTASigningKey = testKTASigningKey
	svc := NewVerificationService(cfg, VerificationDeps{
		Repo: repo, AnggotaRepo: &testutil.FakeAnggotaRepo{}, KTASvc: &fakeApproveKTASvc{},
	})

	_, err := svc.ProcessApproval(context.Background(), item.ID, domain.PendaftaranActionSetujui, "", testutil.SuperActor(), domain.AuditContext{})
	if err == nil {
		t.Fatal("tanpa UserRepo approve harus gagal (fail-closed), bukan skip diam-diam")
	}
}
