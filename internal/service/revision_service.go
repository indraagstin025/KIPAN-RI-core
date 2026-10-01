package service

// RevisionService menangani alur revisi mandiri applicant (R3: pecahan dari
// god-service pendaftaran). Dependensi minimal: repo + storage + audit.

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/kipan-indonesia/sim-kipan-core/config"
	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/crypto"
)

type RevisionService interface {
	RequestRevisionToken(ctx context.Context, req domain.RevisionTokenRequest, audit domain.AuditContext) (*domain.RevisionTokenResponse, error)
	SubmitRevision(ctx context.Context, nomor string, req domain.RevisionSubmitRequest, audit domain.AuditContext) error
}

// RevisionDeps adalah dependensi service revisi (R1: pola deps).
type RevisionDeps struct {
	Repo       repository.PendaftaranRepository
	StorageSvc ObjectVerifier
	AuditRepo  repository.AuditLogRepository
}

type revisionSvc struct {
	cfg        *config.Config
	repo       repository.PendaftaranRepository
	storageSvc ObjectVerifier
	auditRepo  repository.AuditLogRepository
}

func NewRevisionService(cfg *config.Config, deps RevisionDeps) RevisionService {
	return &revisionSvc{cfg: cfg, repo: deps.Repo, storageSvc: deps.StorageSvc, auditRepo: deps.AuditRepo}
}

// RevisionTokenTTL adalah masa berlaku token revisi applicant.
const RevisionTokenTTL = 24 * time.Hour

// normalizeEmail menyeragamkan email untuk perbandingan bukti pemilik.
func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// normalizeWA menyeragamkan nomor WA ke digit inti: buang non-digit lalu
// buang prefix negara/ trunk (62/0) sehingga 08xx, 62xxx, +62xxx setara.
func normalizeWA(wa string) string {
	var digits strings.Builder
	for _, r := range wa {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	d := digits.String()
	d = strings.TrimPrefix(d, "62")
	d = strings.TrimPrefix(d, "0")
	return d
}

// MatchOwnerProof mencocokkan bukti pemilik (email DAN whatsapp) terhadap
// data terdaftar. Pure function agar unit-testable. Kedua sisi
// dinormalisasi; bukti kosong selalu gagal.
func MatchOwnerProof(storedEmail, storedWA, proofEmail, proofWA string) bool {
	pe, pw := normalizeEmail(proofEmail), normalizeWA(proofWA)
	if pe == "" || pw == "" {
		return false
	}
	return normalizeEmail(storedEmail) == pe && normalizeWA(storedWA) == pw
}

// RequestRevisionToken menerbitkan token revisi satu-permintaan untuk
// pendaftaran berstatus PERBAIKAN. Wajib bukti pemilik (email DAN whatsapp
// terdaftar — BE-001): nomor saja tidak cukup karena sekuensial dan
// statusnya publik. Token mentah dikembalikan sekali; yang disimpan hanya
// hash SHA-256 + expiry.
//
// PENERIMAAN RISIKO SEMENTARA (dicatat di laporan PR Fase 2): token
// dikembalikan di respons, bukan kanal terverifikasi. Diterima karena
// bukti ganda + limiter per-nomor + 24 jam + sekali pakai; pengiriman
// WA/email tetap wajib sebelum produksi (Fase 5).
func (s *revisionSvc) RequestRevisionToken(ctx context.Context, req domain.RevisionTokenRequest, audit domain.AuditContext) (*domain.RevisionTokenResponse, error) {
	nr := strings.TrimSpace(req.Nomor)
	if nr == "" || len(nr) > 30 {
		return nil, domain.NewValidationError("Nomor pendaftaran tidak valid")
	}
	if s.repo == nil {
		return nil, unavailable("pendaftaran")
	}
	item, err := s.repo.GetByNomorPendaftaran(ctx, nr)
	if err != nil {
		return nil, err
	}
	if item.Status != domain.PendaftaranStatusPerbaikan {
		return nil, domain.NewValidationError("Pendaftaran tidak dalam status revisi (PERBAIKAN)")
	}
	if !MatchOwnerProof(item.Email, item.Whatsapp, req.Email, req.Whatsapp) {
		// 403 generik: tanpa bocorkan field mana yang salah. Bukti PII
		// pemohon TIDAK masuk audit (hanya fakta kegagalan).
		denyMeta := `{"reason":"owner_mismatch"}`
		s.auditEvent(ctx, audit, nil, "Pendaftar "+nr, "PUBLIK",
			"pendaftaran", strconv.Itoa(item.ID), "REVISI_TOKEN_DENIED", &denyMeta)
		return nil, domain.NewForbiddenError("Anda tidak berhak meminta token revisi ini")
	}

	raw, err := crypto.GenerateSecureToken(32)
	if err != nil {
		return nil, fmt.Errorf("gagal menerbitkan token revisi: %w", err)
	}
	expiresAt := time.Now().Add(RevisionTokenTTL)
	if err := s.repo.SetRevisiToken(ctx, item.ID, crypto.HashToken(raw), expiresAt); err != nil {
		return nil, err
	}
	s.auditEvent(ctx, audit, nil, "Pendaftar "+nr, "PUBLIK",
		"pendaftaran", strconv.Itoa(item.ID), "REVISI_TOKEN", nil)
	return &domain.RevisionTokenResponse{Token: raw, ExpiresAt: expiresAt}, nil
}

// SubmitRevision memproses revisi mandiri applicant: token valid +
// belum kedaluwarsa + state PERBAIKAN (satu UPDATE atomik, error generik
// tanpa oracle), dokumen baru tervalidasi + terverifikasi storage,
// status kembali DIAJUKAN, token hangus sekali pakai.
func (s *revisionSvc) SubmitRevision(ctx context.Context, nomor string, req domain.RevisionSubmitRequest, audit domain.AuditContext) error {
	nr := strings.TrimSpace(nomor)
	if nr == "" || len(nr) > 30 {
		return domain.NewValidationError("Nomor pendaftaran tidak valid")
	}
	token := strings.TrimSpace(req.Token)
	if token == "" || len(token) > 256 {
		return domain.NewValidationError("Token revisi wajib diisi")
	}
	if s.repo == nil {
		return unavailable("pendaftaran")
	}
	item, err := s.repo.GetByNomorPendaftaran(ctx, nr)
	if err != nil {
		return err
	}

	// Merge dokumen: field kosong = pertahankan yang lama. Field baru
	// wajib lolos pola key + verifikasi storage (bila dikonfigurasi).
	keys := map[string]string{}
	inputs := map[string]struct {
		val      string
		category string
		current  string
	}{
		"foto_key":             {req.FotoKey, "foto", item.FotoKey},
		"ktp_key":              {req.KTPKey, "ktp", item.KTPKey},
		"cv_key":               {req.CVKey, "cv", item.CVKey},
		"sk_key":               {req.SKKey, "sk", item.SKKey},
		"surat_pernyataan_key": {req.SuratPernyataanKey, "surat_pernyataan", item.SuratPernyataanKey},
		"surat_sehat_key":      {req.SuratSehatKey, "surat_sehat", item.SuratSehatKey},
	}
	for name, in := range inputs {
		k := strings.TrimSpace(in.val)
		if k == "" {
			k = in.current
		} else {
			if err := checkObjectKey(name, k, false); err != nil {
				return err
			}
			if err := verifyOneDocument(ctx, s.cfg, s.storageSvc, k, in.category); err != nil {
				return err
			}
		}
		// Hanya foto + KTP yang wajib (selaras submit); dokumen opsional
		// boleh tetap kosong bila tidak pernah diunggah.
		if strings.TrimSpace(k) == "" && (name == "foto_key" || name == "ktp_key") {
			return domain.NewValidationError("Dokumen " + name + " wajib ada")
		}
		keys[name] = k
	}

	// Perbandingan hash dilakukan di SQL dalam UPDATE atomik yang sama:
	// token 256-bit + rate limit membuat brute force infeasible, sementara
	// single-statement memberi satu error generik (tanpa oracle bedakan
	// token salah vs kedaluwarsa vs state salah).
	if err := s.repo.SubmitRevisionTx(ctx, item.ID, crypto.HashToken(token), keys, strings.TrimSpace(req.Catatan)); err != nil {
		return err
	}
	s.auditEvent(ctx, audit, nil, "Pendaftar "+nr, "PUBLIK",
		"pendaftaran", strconv.Itoa(item.ID), "REVISI", nil)
	return nil
}

// auditEvent mendelegasikan ke writeAudit terpusat (R2).
func (s *revisionSvc) auditEvent(
	ctx context.Context,
	audit domain.AuditContext,
	actorID *string,
	actorName, actorRole, entity, entityID, action string,
	metadata *string,
) {
	writeAudit(ctx, s.auditRepo, audit, actorID, actorName, actorRole, entity, entityID, action, metadata)
}
