package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kipan-indonesia/sim-kipan-core/config"
	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/crypto"
)

var errFakeUnimpl = errors.New("fake: tidak diimplementasikan")

const revealTestKey = "aa00112233445566778899aabbccddeeffaa00112233445566778899aabbccdd"

// fakePendaftaranRepo melayani GetByID canned untuk uji RevealNIK.
type fakePendaftaranRepo struct {
	item *domain.Pendaftaran
}

func (f *fakePendaftaranRepo) Create(context.Context, *domain.Pendaftaran) error {
	return errFakeUnimpl
}
func (f *fakePendaftaranRepo) CreateWithHistory(context.Context, *domain.Pendaftaran, string, string) error {
	return errFakeUnimpl
}
func (f *fakePendaftaranRepo) NextRegistrationSequence(context.Context, int, int) (int, error) {
	return 0, errFakeUnimpl
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
	return nil, errFakeUnimpl
}
func (f *fakePendaftaranRepo) CountQueue(context.Context, *int, *int, string) (int, error) {
	return 0, errFakeUnimpl
}
func (f *fakePendaftaranRepo) UpdateStatus(context.Context, int, domain.PendaftaranStatus, string) error {
	return errFakeUnimpl
}
func (f *fakePendaftaranRepo) UpdateStatusWithHistory(context.Context, int, domain.PendaftaranStatus, string, *string, *string, *string, string) error {
	return errFakeUnimpl
}
func (f *fakePendaftaranRepo) AppendHistory(context.Context, int, string, *string, *string, *string, string) error {
	return errFakeUnimpl
}
func (f *fakePendaftaranRepo) IssueMember(context.Context, int, int, string) (*domain.Anggota, error) {
	return nil, errFakeUnimpl
}
func (f *fakePendaftaranRepo) SetRevisiToken(context.Context, int, string, time.Time) error {
	return errFakeUnimpl
}
func (f *fakePendaftaranRepo) SubmitRevisionTx(context.Context, int, string, map[string]string, string) error {
	return errFakeUnimpl
}

var _ repository.PendaftaranRepository = (*fakePendaftaranRepo)(nil)

func TestRevealNIKSuccess(t *testing.T) {
	enc, err := crypto.EncryptAESGCM("3201010101010001", revealTestKey)
	if err != nil {
		t.Fatalf("enkripsi uji gagal: %v", err)
	}
	prov, kab := 2, 3
	repo := &fakePendaftaranRepo{item: &domain.Pendaftaran{
		ID: 9, NIKEncrypted: enc, ProvinsiID: prov, KabupatenID: kab,
		Status: domain.PendaftaranStatusDiajukan,
	}}
	cfg := &config.Config{}
	cfg.Crypto.AESMasterKey = revealTestKey
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
		Status: domain.PendaftaranStatusDiajukan,
	}}
	cfg := &config.Config{}
	cfg.Crypto.AESMasterKey = revealTestKey
	svc := NewVerificationService(cfg, VerificationDeps{Repo: repo})
	actor := domain.ActorContext{UserID: "u1", Role: domain.RoleAdminKabupaten, ProvinsiID: &prov, KabupatenID: &otherKab}

	if _, err := svc.RevealNIK(context.Background(), 9, actor, domain.AuditContext{}); err == nil {
		t.Fatal("reveal lintas wilayah DITERIMA")
	}
}
