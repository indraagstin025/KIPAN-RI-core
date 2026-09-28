package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/kipan-indonesia/sim-kipan-core/config"
	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/kta"
)

// KTADocumentStore adalah kapabilitas storage yang dibutuhkan KTA.
type KTADocumentStore interface {
	Configured() bool
	PutKTADocument(ctx context.Context, nia string, pdf []byte) (string, error)
	PresignKTADocument(ctx context.Context, key string) (string, error)
}

// KTAService mengorkestrasi dokumen KTA: render server-side saat approve
// (fail-closed: gagal render = approve gagal, admin retry) dan unduhan
// teraudit untuk admin dalam yurisdiksi.
type KTAService interface {
	// IssueKTADocument merender + menyimpan PDF KTA anggota. Idempoten:
	// bila kta_pdf_key sudah terisi, kembalikan key lama (jalur heal
	// retry approve yang sebelumnya gagal di tengah jalan).
	IssueKTADocument(ctx context.Context, member *domain.Anggota, ktaSig string, audit domain.AuditContext) (string, error)
	// GetKTADocumentURL mengembalikan tiket baca PDF untuk admin
	// dalam yurisdiksi anggota + audit VIEW.
	GetKTADocumentURL(ctx context.Context, anggotaID int, actor domain.ActorContext, audit domain.AuditContext) (string, error)
}

type ktaService struct {
	cfg         *config.Config
	anggotaRepo repository.AnggotaRepository
	docStore    KTADocumentStore
	auditRepo   repository.AuditLogRepository
}

// KTADeps adalah dependensi service KTA (R1: konsisten dengan pola deps).
type KTADeps struct {
	AnggotaRepo repository.AnggotaRepository
	DocStore    KTADocumentStore
	AuditRepo   repository.AuditLogRepository
}

func NewKTAService(cfg *config.Config, deps KTADeps) KTAService {
	return &ktaService{cfg: cfg, anggotaRepo: deps.AnggotaRepo, docStore: deps.DocStore, auditRepo: deps.AuditRepo}
}

func (s *ktaService) verifyBaseURL() string {
	if s.cfg != nil && strings.TrimSpace(s.cfg.App.KTAVerifyBaseURL) != "" {
		return s.cfg.App.KTAVerifyBaseURL
	}
	return "https://kipan.id"
}

func (s *ktaService) IssueKTADocument(ctx context.Context, member *domain.Anggota, ktaSig string, audit domain.AuditContext) (string, error) {
	if member == nil {
		return "", domain.NewValidationError("Data anggota tidak valid")
	}
	// Jalur heal: PDF sudah pernah terbit → pakai ulang, jangan render ulang.
	if member.KTAPDFKey != nil && strings.TrimSpace(*member.KTAPDFKey) != "" {
		return *member.KTAPDFKey, nil
	}
	if s.anggotaRepo == nil {
		return "", domain.NewUnavailableError("Layanan anggota sedang tidak tersedia")
	}
	if s.docStore == nil || !s.docStore.Configured() {
		if s.cfg != nil && s.cfg.App.Env == "production" {
			return "", domain.NewUnavailableError("Layanan storage belum dikonfigurasi")
		}
		// Dev tanpa storage: lewati PDF (approve tetap sah), dengan
		// peringatan eksplisit — pola degraded yang sama dengan
		// verifikasi dokumen submit.
		log.Warn().Str("nia", member.NIA).
			Msg("Storage tidak dikonfigurasi — PDF KTA dilewati (HANYA non-production)")
		return "", nil
	}

	verifyURL := fmt.Sprintf("%s/v/%s?sig=%s", s.verifyBaseURL(), member.NIA, ktaSig)
	qrPNG, err := kta.QRBytes(verifyURL)
	if err != nil {
		return "", fmt.Errorf("gagal membuat QR KTA: %w", err)
	}
	pdf, err := kta.RenderPDF(kta.CardData{
		NIA:           member.NIA,
		NamaLengkap:   member.NamaLengkap,
		Status:        string(member.Status),
		TanggalAngkat: member.TanggalAngkat,
		VerifyURL:     verifyURL,
	}, qrPNG)
	if err != nil {
		return "", err
	}
	key, err := s.docStore.PutKTADocument(ctx, member.NIA, pdf)
	if err != nil {
		return "", err
	}
	if err := s.anggotaRepo.SetKTAPDFKey(ctx, member.ID, key); err != nil {
		return "", fmt.Errorf("gagal mencatat key PDF KTA: %w", err)
	}
	member.KTAPDFKey = &key
	return key, nil
}

func (s *ktaService) GetKTADocumentURL(ctx context.Context, anggotaID int, actor domain.ActorContext, audit domain.AuditContext) (string, error) {
	if anggotaID <= 0 {
		return "", domain.NewValidationError("ID anggota tidak valid")
	}
	if s.anggotaRepo == nil {
		return "", domain.NewUnavailableError("Layanan anggota sedang tidak tersedia")
	}
	member, err := s.anggotaRepo.GetByID(ctx, anggotaID)
	if err != nil {
		return "", err
	}
	if !actor.CanAccessWilayah(member.ProvinsiID, member.KabupatenID) {
		return "", domain.NewForbiddenError("Anggota di luar wilayah kerja Anda")
	}
	if member.KTAPDFKey == nil || strings.TrimSpace(*member.KTAPDFKey) == "" {
		return "", domain.NewNotFoundError("Dokumen KTA")
	}
	if s.docStore == nil || !s.docStore.Configured() {
		return "", domain.NewUnavailableError("Layanan storage belum dikonfigurasi")
	}
	url, err := s.docStore.PresignKTADocument(ctx, *member.KTAPDFKey)
	if err != nil {
		return "", err
	}
	if s.auditRepo != nil {
		actorID := actor.UserID
		meta := `{"event":"kta_download"}`
		_ = s.auditRepo.Create(ctx, &domain.ActivityLog{
			ActorID:    &actorID,
			ActorName:  actor.Name,
			ActorRole:  string(actor.Role),
			IPAddress:  audit.IP,
			UserAgent:  audit.UserAgent,
			EntityName: "anggota",
			EntityID:   strconv.Itoa(anggotaID),
			Action:     "VIEW",
			Metadata:   &meta,
			RequestID:  audit.RequestID,
		})
	}
	return url, nil
}
