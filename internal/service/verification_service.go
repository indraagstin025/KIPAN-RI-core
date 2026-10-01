package service

// VerificationService menangani verifikasi admin + KTA + NIK reveal (R3:
// pecahan dari god-service pendaftaran). Dependensi minimal: repo +
// anggota + audit + KTA.

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/kipan-indonesia/sim-kipan-core/config"
	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/crypto"
)

type VerificationService interface {
	ProcessApproval(ctx context.Context, id int, action domain.PendaftaranApprovalAction, catatan string, actor domain.ActorContext, audit domain.AuditContext) error
	VerifyKTA(ctx context.Context, nia, sig string) (*domain.KTAVerificationResponse, error)
	RevealNIK(ctx context.Context, id int, actor domain.ActorContext, audit domain.AuditContext) (string, error)
}

// VerificationDeps adalah dependensi service verifikasi (R1: pola deps).
type VerificationDeps struct {
	Repo        repository.PendaftaranRepository
	AnggotaRepo repository.AnggotaRepository
	AuditRepo   repository.AuditLogRepository
	KTASvc      KTAService
}

type verificationSvc struct {
	cfg         *config.Config
	repo        repository.PendaftaranRepository
	anggotaRepo repository.AnggotaRepository
	auditRepo   repository.AuditLogRepository
	ktaSvc      KTAService
}

func NewVerificationService(cfg *config.Config, deps VerificationDeps) VerificationService {
	return &verificationSvc{
		cfg: cfg, repo: deps.Repo, anggotaRepo: deps.AnggotaRepo,
		auditRepo: deps.AuditRepo, ktaSvc: deps.KTASvc,
	}
}

// ProcessApproval memvalidasi otorisasi + jurisdiction + transisi status,
// lalu mengeksekusi secara atomik beserta riwayat beraktor dan audit trail.
// actor WAJIB berasal dari JWT terverifikasi (RULES 6), bukan dari client.
func (s *verificationSvc) ProcessApproval(ctx context.Context, id int, action domain.PendaftaranApprovalAction, catatan string, actor domain.ActorContext, audit domain.AuditContext) error {
	if id <= 0 {
		return domain.NewValidationError("ID pendaftaran tidak valid")
	}
	if s.repo == nil {
		return unavailable("pendaftaran")
	}
	if action == "" {
		return domain.NewValidationError("Aksi verifikasi wajib dipilih")
	}

	item, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if !actor.CanAccessWilayah(item.ProvinsiID, item.KabupatenID) {
		return domain.NewForbiddenError("Pendaftaran di luar wilayah kerja Anda")
	}

	targetStatus, ok := mapStatusForAction(action)
	if !ok {
		return domain.NewValidationError("Aksi tidak valid untuk proses pendaftaran")
	}
	if !domain.IsAllowedTransition(item.Status, targetStatus, action) {
		return domain.NewValidationError("Transisi status tidak sah untuk aksi yang diminta")
	}

	note := strings.TrimSpace(catatan)
	actorID, actorName, actorRole := actor.UserID, actor.Name, string(actor.Role)
	meta := fmt.Sprintf(`{"from":%q,"to":%q}`, string(item.Status), string(targetStatus))

	if action == domain.PendaftaranActionSetujui {
		if s.cfg == nil || strings.TrimSpace(s.cfg.Crypto.KTASigningKey) == "" {
			return domain.NewValidationError("KTA_SIGNING_KEY belum dikonfigurasi")
		}
		member, err := s.repo.IssueMember(ctx, id, time.Now().Year(), s.cfg.Crypto.KTASigningKey)
		if err != nil {
			// Jalur heal: approve sebelumnya berhasil terbitkan anggota
			// tetapi gagal di PDF (fail-closed) — coba selesaikan PDF-nya
			// alih-alih gagal dengan "sudah memiliki anggota".
			var appErr *domain.AppError
			if errors.As(err, &appErr) && appErr.Code == 409 && s.anggotaRepo != nil {
				return s.healKTADocument(ctx, id, actorID, actorName, actorRole, audit, meta)
			}
			return err
		}
		// PDF KTA server-side, fail-closed: gagal render/upload = approve
		// gagal, admin retry (idempoten via jalur heal di atas).
		if s.ktaSvc != nil {
			if _, err := s.ktaSvc.IssueKTADocument(ctx, member, member.KTAQRHashValue(), audit); err != nil {
				return err
			}
		}
		s.auditEvent(ctx, audit, &actorID, actorName, actorRole,
			"pendaftaran", strconv.Itoa(id), string(action), &meta)
		return nil
	}

	if err := s.repo.UpdateStatusWithHistory(ctx, id, targetStatus, string(action), &actorID, &actorName, &actorRole, note); err != nil {
		return err
	}
	s.auditEvent(ctx, audit, &actorID, actorName, actorRole,
		"pendaftaran", strconv.Itoa(id), string(action), &meta)
	return nil
}

// healKTADocument menyelesaikan PDF KTA untuk approve yang sebelumnya gagal
// di tengah jalan (anggota sudah terbit, PDF belum). Idempoten: bila PDF
// sudah ada, IssueKTADocument mengembalikan key lama.
func (s *verificationSvc) healKTADocument(ctx context.Context, id int, actorID, actorName, actorRole string, audit domain.AuditContext, meta string) error {
	if s.anggotaRepo == nil || s.ktaSvc == nil {
		return domain.NewConflictError("Pendaftaran sudah memiliki anggota")
	}
	item, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if item.AnggotaID == nil {
		return domain.NewConflictError("Pendaftaran sudah memiliki anggota")
	}
	member, err := s.anggotaRepo.GetByID(ctx, *item.AnggotaID)
	if err != nil {
		return err
	}
	if _, err := s.ktaSvc.IssueKTADocument(ctx, member, member.KTAQRHashValue(), audit); err != nil {
		return err
	}
	s.auditEvent(ctx, audit, &actorID, actorName, actorRole,
		"pendaftaran", strconv.Itoa(id), string(domain.PendaftaranActionSetujui), &meta)
	return nil
}

// VerifyKTA memverifikasi keaslian KTA secara kriptografis (RULES 20).
// NIA tak dikenal dan signature salah menghasilkan verdict valid=false yang
// sama (tanpa oracle). Hanya input kosong yang ditolak sebagai 422.
func (s *verificationSvc) VerifyKTA(ctx context.Context, nia, sig string) (*domain.KTAVerificationResponse, error) {
	code := strings.TrimSpace(nia)
	signature := strings.TrimSpace(sig)
	if code == "" || len(code) > 50 || len(signature) > 128 {
		return nil, domain.NewValidationError("Parameter verifikasi KTA tidak valid")
	}
	if s.anggotaRepo == nil {
		return nil, unavailable("anggota")
	}
	member, err := s.anggotaRepo.GetByNIA(ctx, code)
	if err != nil {
		// NIA tak dikenal = verdict tidak valid (bukan 404, anti oracle).
		return &domain.KTAVerificationResponse{NIA: code, Valid: false}, nil
	}
	keys := ktaVerifyKeys(s.cfg)
	if len(keys) == 0 {
		return nil, domain.NewValidationError("KTA_SIGNING_KEY belum dikonfigurasi")
	}
	// Rotasi: coba kunci aktif dulu, lalu kunci sebelumnya. Kartu lama yang
	// ditandatangani kunci prev tetap valid tanpa migrasi ulang.
	valid := false
	for _, k := range keys {
		if err := crypto.VerifyKTASignature(
			member.NIA,
			member.TanggalAngkat.Format("2006-01-02"),
			member.ID,
			signature,
			k,
		); err == nil {
			valid = true
			break
		}
	}
	if !valid {
		return &domain.KTAVerificationResponse{NIA: member.NIA, Valid: false}, nil
	}
	tgl := member.TanggalAngkat
	return &domain.KTAVerificationResponse{
		NIA:           member.NIA,
		Valid:         true,
		NamaLengkap:   member.NamaLengkap,
		Status:        string(member.Status),
		TanggalAngkat: &tgl,
	}, nil
}

// RevealNIK mendekripsi NIK khusus untuk admin verifikator dalam yurisdiksinya.
// Setiap pembukaan dicatat (actor, IP, request-ID) agar dapat diaudit (RULES 12, 21).
func (s *verificationSvc) RevealNIK(ctx context.Context, id int, actor domain.ActorContext, audit domain.AuditContext) (string, error) {
	if id <= 0 {
		return "", domain.NewValidationError("ID pendaftaran tidak valid")
	}
	if s.repo == nil {
		return "", unavailable("pendaftaran")
	}
	item, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return "", err
	}
	if !actor.CanAccessWilayah(item.ProvinsiID, item.KabupatenID) {
		return "", domain.NewForbiddenError("Pendaftaran di luar wilayah kerja Anda")
	}
	key, err := aesKey(s.cfg)
	if err != nil {
		return "", err
	}
	nik, err := crypto.DecryptAESGCM(item.NIKEncrypted, key)
	if err != nil {
		return "", domain.NewValidationError("Data NIK tidak dapat dibuka")
	}
	actorID, actorName, actorRole := actor.UserID, actor.Name, string(actor.Role)
	meta := `{"event":"nik_reveal"}`
	s.auditEvent(ctx, audit, &actorID, actorName, actorRole,
		"pendaftaran", strconv.Itoa(id), "NIK_REVEAL", &meta)
	return nik, nil
}

func mapStatusForAction(action domain.PendaftaranApprovalAction) (domain.PendaftaranStatus, bool) {
	switch action {
	case domain.PendaftaranActionVerifikasi:
		return domain.PendaftaranStatusDiverifikasi, true
	case domain.PendaftaranActionPerbaikan:
		return domain.PendaftaranStatusPerbaikan, true
	case domain.PendaftaranActionTolak:
		return domain.PendaftaranStatusDitolak, true
	case domain.PendaftaranActionSetujui:
		return domain.PendaftaranStatusDisetujui, true
	default:
		return "", false
	}
}

// ktaVerifyKeys mengembalikan kunci verifikasi KTA: aktif dulu, lalu kunci
// rotasi sebelumnya (verify-only). Urutan penting untuk short-circuit.
func ktaVerifyKeys(cfg *config.Config) []string {
	if cfg == nil {
		return nil
	}
	keys := make([]string, 0, 2)
	if k := strings.TrimSpace(cfg.Crypto.KTASigningKey); k != "" {
		keys = append(keys, k)
	}
	if k := strings.TrimSpace(cfg.Crypto.KTASigningKeyPrev); k != "" {
		keys = append(keys, k)
	}
	return keys
}

// auditEvent mendelegasikan ke writeAudit terpusat (R2).
func (s *verificationSvc) auditEvent(
	ctx context.Context,
	audit domain.AuditContext,
	actorID *string,
	actorName, actorRole, entity, entityID, action string,
	metadata *string,
) {
	writeAudit(ctx, s.auditRepo, audit, actorID, actorName, actorRole, entity, entityID, action, metadata)
}
