package pendaftaran

import (
	"context"
	"testing"
	"time"

	"github.com/kipan-indonesia/sim-kipan-core/config"
	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
	"github.com/kipan-indonesia/sim-kipan-core/internal/service/testutil"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/crypto"
)

// fakePendaftaranRepo melayani GetByID canned untuk uji RevealNIK.
type fakePendaftaranRepo struct {
	item *domain.Pendaftaran
}

func (f *fakePendaftaranRepo) Create(context.Context, *domain.Pendaftaran) error {
	return testutil.ErrFakeUnimpl
}
func (f *fakePendaftaranRepo) CreateWithHistory(context.Context, *domain.Pendaftaran, string, string) error {
	return testutil.ErrFakeUnimpl
}
func (f *fakePendaftaranRepo) NextRegistrationSequence(context.Context, int, int) (int, error) {
	return 0, testutil.ErrFakeUnimpl
}
func (f *fakePendaftaranRepo) GetByID(context.Context, int) (*domain.Pendaftaran, error) {
	if f.item == nil {
		return nil, domain.ErrNotFound
	}
	return f.item, nil
}
func (f *fakePendaftaranRepo) GetByNomorPendaftaran(context.Context, string) (*domain.Pendaftaran, error) {
	return nil, domain.ErrNotFound
}
func (f *fakePendaftaranRepo) GetByNikHash(context.Context, string) (*domain.Pendaftaran, error) {
	return nil, domain.ErrNotFound
}
func (f *fakePendaftaranRepo) ListHistory(context.Context, int) ([]domain.PendaftaranRiwayat, error) {
	return []domain.PendaftaranRiwayat{}, nil
}
func (f *fakePendaftaranRepo) ListQueue(context.Context, *int, *int, string, int, int) ([]domain.PendaftaranQueueItem, error) {
	return nil, testutil.ErrFakeUnimpl
}
func (f *fakePendaftaranRepo) CountQueue(context.Context, *int, *int, string) (int, error) {
	return 0, testutil.ErrFakeUnimpl
}
func (f *fakePendaftaranRepo) UpdateStatus(context.Context, int, domain.PendaftaranStatus, string) error {
	return testutil.ErrFakeUnimpl
}
func (f *fakePendaftaranRepo) UpdateStatusWithHistory(context.Context, int, domain.PendaftaranStatus, string, *string, *string, *string, string) error {
	return testutil.ErrFakeUnimpl
}
func (f *fakePendaftaranRepo) AppendHistory(context.Context, int, string, *string, *string, *string, string) error {
	return testutil.ErrFakeUnimpl
}
func (f *fakePendaftaranRepo) IssueMember(context.Context, int, int, string) (*domain.Anggota, error) {
	return nil, testutil.ErrFakeUnimpl
}
func (f *fakePendaftaranRepo) SetRevisiToken(context.Context, int, string, time.Time) error {
	return testutil.ErrFakeUnimpl
}
func (f *fakePendaftaranRepo) SubmitRevisionTx(context.Context, int, string, map[string]string, string) error {
	return testutil.ErrFakeUnimpl
}
func (f *fakePendaftaranRepo) ExpireStaleDrafts(context.Context, int) error { return nil }

var _ repository.PendaftaranRepository = (*fakePendaftaranRepo)(nil)

func TestRevealNIKSuccess(t *testing.T) {
	enc, err := crypto.EncryptAESGCM("3201010101010001", testutil.RevealTestKey)
	if err != nil {
		t.Fatalf("enkripsi uji gagal: %v", err)
	}
	prov, kab := 2, 3
	repo := &fakePendaftaranRepo{item: &domain.Pendaftaran{
		ID: 9, NIKEncrypted: enc, ProvinsiID: prov, KabupatenID: kab,
		Status: domain.PendaftaranStatusDraft,
	}}
	cfg := &config.Config{}
	cfg.Crypto.AESMasterKey = testutil.RevealTestKey
	svc := NewVerificationService(cfg, VerificationDeps{Repo: repo})
	actor := domain.ActorContext{UserID: "u1", Role: domain.RoleAdminKabupaten, ProvinsiID: &prov, KabupatenID: &kab}

	nik, err := svc.RevealNIK(context.Background(), 9, actor, domain.AuditContext{})
	if err != nil {
		t.Fatalf("RevealNIK gagal: %v", err)
	}
	if nik != "3201010101010001" {
		t.Fatalf("NIK salah: %q", nik)
	}
}

func TestRevealNIKLintasWilayahDitolak(t *testing.T) {
	prov, kab := 2, 3
	otherKab := 99
	repo := &fakePendaftaranRepo{item: &domain.Pendaftaran{
		ID: 9, NIKEncrypted: "x", ProvinsiID: prov, KabupatenID: kab,
		Status: domain.PendaftaranStatusDraft,
	}}
	cfg := &config.Config{}
	cfg.Crypto.AESMasterKey = testutil.RevealTestKey
	svc := NewVerificationService(cfg, VerificationDeps{Repo: repo})
	actor := domain.ActorContext{UserID: "u1", Role: domain.RoleAdminKabupaten, ProvinsiID: &prov, KabupatenID: &otherKab}

	if _, err := svc.RevealNIK(context.Background(), 9, actor, domain.AuditContext{}); err == nil {
		t.Fatal("reveal lintas wilayah DITERIMA")
	}
}
