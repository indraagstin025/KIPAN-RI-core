package service

// Uji Batch 2: penerbitan akun USER saat approve + KTA mandiri.

import (
	"context"
	"testing"
	"time"

	"github.com/kipan-indonesia/sim-kipan-core/config"
	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
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

// fakeMemberUserRepo menyimpan user in-memory keyed by email.
type fakeMemberUserRepo struct {
	repository.UserRepository
	byEmail  map[string]*domain.User
	byID     map[string]*domain.User
	created  []*domain.User
	revoked  []string
	lastHash string
}

func (f *fakeMemberUserRepo) GetByID(_ context.Context, id string) (*domain.User, error) {
	if u, ok := f.byID[id]; ok && u != nil {
		return u, nil
	}
	return nil, domain.ErrUserNotFound
}

func (f *fakeMemberUserRepo) UpdatePassword(_ context.Context, id string, hash string) error {
	f.lastHash = hash
	if u, ok := f.byID[id]; ok && u != nil {
		u.PasswordHash = hash
	}
	return nil
}

func (f *fakeMemberUserRepo) RevokeAllUserTokens(_ context.Context, id string) error {
	f.revoked = append(f.revoked, id)
	return nil
}

func (f *fakeMemberUserRepo) GetByEmail(_ context.Context, email string) (*domain.User, error) {
	for addr, u := range f.byEmail {
		if addr == email {
			return u, nil
		}
	}
	return nil, domain.ErrUserNotFound
}

func (f *fakeMemberUserRepo) Create(_ context.Context, u *domain.User) error {
	if f.byEmail == nil {
		f.byEmail = map[string]*domain.User{}
	}
	if _, ok := f.byEmail[u.Email]; ok {
		return domain.NewConflictError("email sudah ada")
	}
	f.byEmail[u.Email] = u
	f.created = append(f.created, u)
	return nil
}

// fakeApproveKTASvc melewati render PDF (bukan fokus uji ini).
type fakeApproveKTASvc struct {
	KTAService
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

// fakeOutboxRepo merekam email yang di-enqueue (outbox).
type fakeOutboxRepo struct {
	repository.EmailOutboxRepository
	enqueued []*domain.EmailOutbox
}

func (f *fakeOutboxRepo) Enqueue(_ context.Context, it *domain.EmailOutbox) error {
	f.enqueued = append(f.enqueued, it)
	return nil
}

func approveSvc(repo *fakeApproveRepo, users *fakeMemberUserRepo, anggota *fakeAnggotaRepo, outbox repository.EmailOutboxRepository) VerificationService {
	cfg := &config.Config{}
	cfg.Crypto.KTASigningKey = testKTASigningKey
	return NewVerificationService(cfg, VerificationDeps{
		Repo: repo, AnggotaRepo: anggota, UserRepo: users, KTASvc: &fakeApproveKTASvc{},
		OutboxRepo: outbox,
	})
}

func superActor() domain.ActorContext {
	prov, kab := 32, 3273
	return domain.ActorContext{UserID: "admin-1", Name: "Admin", Role: domain.RoleSuperAdmin, ProvinsiID: &prov, KabupatenID: &kab}
}

func TestApproveCreatesUserAccount(t *testing.T) {
	item, member := approveFixture()
	repo := &fakeApproveRepo{item: item, member: member}
	users := &fakeMemberUserRepo{}
	anggota := &fakeAnggotaRepo{byID: map[int]*domain.Anggota{member.ID: member}}
	outbox := &fakeOutboxRepo{}
	svc := approveSvc(repo, users, anggota, outbox)

	res, err := svc.ProcessApproval(context.Background(), item.ID, domain.PendaftaranActionSetujui, "", superActor(), domain.AuditContext{})
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
	if got := anggota.links[member.ID]; got != u.ID {
		t.Fatalf("anggota tidak terhubung ke akun: %q", got)
	}
	// Opsi A: kredensial via antrian SET_PASSWORD (bukan plaintext).
	if len(outbox.enqueued) != 1 || outbox.enqueued[0].Jenis != domain.EmailOutboxSetPassword {
		t.Fatalf("harap 1 outbox SET_PASSWORD, dapat %+v", outbox.enqueued)
	}
	if outbox.enqueued[0].UserID == nil || *outbox.enqueued[0].UserID != u.ID {
		t.Fatalf("outbox harus menunjuk user_id akun baru")
	}
}

func TestApprovePengurusCreatesPengurusUser(t *testing.T) {
	item, member := approveFixture()
	member.Tipe = domain.TipePendaftaranPengurus
	repo := &fakeApproveRepo{item: item, member: member}
	users := &fakeMemberUserRepo{}
	anggota := &fakeAnggotaRepo{byID: map[int]*domain.Anggota{member.ID: member}}
	outbox := &fakeOutboxRepo{}
	svc := approveSvc(repo, users, anggota, outbox)

	res, err := svc.ProcessApproval(context.Background(), item.ID, domain.PendaftaranActionSetujui, "", superActor(), domain.AuditContext{})
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
	if len(outbox.enqueued) != 1 || outbox.enqueued[0].Jenis != domain.EmailOutboxSetPassword {
		t.Fatalf("harap outbox SET_PASSWORD, dapat %+v", outbox.enqueued)
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
		svc := approveSvc(repo, &fakeMemberUserRepo{}, &fakeAnggotaRepo{}, &fakeOutboxRepo{})
		_, err := svc.ProcessApproval(context.Background(), item.ID, action, "   ", superActor(), domain.AuditContext{})
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
	users := &fakeMemberUserRepo{byEmail: map[string]*domain.User{member.Email: admin}}
	anggota := &fakeAnggotaRepo{byID: map[int]*domain.Anggota{member.ID: member}}
	svc := approveSvc(repo, users, anggota, &fakeOutboxRepo{})

	_, err := svc.ProcessApproval(context.Background(), item.ID, domain.PendaftaranActionSetujui, "", superActor(), domain.AuditContext{})
	appErr, ok := err.(*domain.AppError)
	if !ok || appErr.Code != 409 {
		t.Fatalf("harap 409 saat email milik akun non-USER, dapat %v", err)
	}
	if len(anggota.links) != 0 {
		t.Fatalf("anggota tidak boleh ditautkan ke akun non-USER: %v", anggota.links)
	}
}

func TestApproveLinksExistingUser(t *testing.T) {
	item, member := approveFixture()
	existing := &domain.User{ID: "user-lama", Email: member.Email, Role: domain.RoleUser, TipeUser: domain.UserTipeKader}
	repo := &fakeApproveRepo{item: item, member: member}
	users := &fakeMemberUserRepo{byEmail: map[string]*domain.User{member.Email: existing}}
	anggota := &fakeAnggotaRepo{byID: map[int]*domain.Anggota{member.ID: member}}
	outbox := &fakeOutboxRepo{}
	svc := approveSvc(repo, users, anggota, outbox)

	res, err := svc.ProcessApproval(context.Background(), item.ID, domain.PendaftaranActionSetujui, "", superActor(), domain.AuditContext{})
	if err != nil {
		t.Fatalf("approve gagal: %v", err)
	}
	if res == nil {
		t.Fatal("hasil nil")
	}
	if len(users.created) != 0 {
		t.Fatalf("tidak boleh buat user baru, tercipta %d", len(users.created))
	}
	if got := anggota.links[member.ID]; got != existing.ID {
		t.Fatalf("anggota harus terhubung ke akun existing: %q", got)
	}
	// Akun tertaut: outbox AKUN_TERHUBUNG (tanpa set-password).
	if len(outbox.enqueued) != 1 || outbox.enqueued[0].Jenis != domain.EmailOutboxAkunTerhubung {
		t.Fatalf("harap 1 outbox AKUN_TERHUBUNG, dapat %+v", outbox.enqueued)
	}
}

func TestApproveTanpaUserRepoGagalFailClosed(t *testing.T) {
	item, member := approveFixture()
	repo := &fakeApproveRepo{item: item, member: member}
	cfg := &config.Config{}
	cfg.Crypto.KTASigningKey = testKTASigningKey
	svc := NewVerificationService(cfg, VerificationDeps{
		Repo: repo, AnggotaRepo: &fakeAnggotaRepo{}, KTASvc: &fakeApproveKTASvc{},
	})

	_, err := svc.ProcessApproval(context.Background(), item.ID, domain.PendaftaranActionSetujui, "", superActor(), domain.AuditContext{})
	if err == nil {
		t.Fatal("tanpa UserRepo approve harus gagal (fail-closed), bukan skip diam-diam")
	}
}

// stubDocStore menerbitkan URL baca palsu untuk uji KTA mandiri.
type stubDocStore struct{}

func (stubDocStore) Configured() bool { return true }
func (stubDocStore) PutKTADocument(_ context.Context, _ string, _ []byte) (string, error) {
	return "kta/x.pdf", nil
}
func (stubDocStore) PresignKTADocument(_ context.Context, _ string) (string, error) {
	return "https://s3.example/kta.pdf", nil
}

func TestGetMyKTA(t *testing.T) {
	uid := "user-kader-1"
	key := "kta/KIPAN-1.pdf"
	member := &domain.Anggota{ID: 31, NIA: "KIPAN-IND-3273-2026-000031", NamaLengkap: "Kader", UserID: &uid, KTAPDFKey: &key}
	cfg := &config.Config{}
	svc := NewKTAService(cfg, KTADeps{
		AnggotaRepo: &fakeAnggotaRepo{byNIA: map[string]*domain.Anggota{member.NIA: member}},
		DocStore:    stubDocStore{},
	})

	url, err := svc.GetMyKTADocumentURL(context.Background(), uid, domain.AuditContext{})
	if err != nil {
		t.Fatalf("KTA sendiri gagal: %v", err)
	}
	if url == "" {
		t.Fatal("harap URL presign dikembalikan")
	}

	if _, err := svc.GetMyKTADocumentURL(context.Background(), "user-tanpa-anggota", domain.AuditContext{}); err == nil {
		t.Fatal("user tanpa anggota harus 404")
	}
}
