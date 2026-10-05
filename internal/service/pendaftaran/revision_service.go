package pendaftaran

// RevisionService menangani alur revisi mandiri applicant (R3: pecahan dari
// god-service pendaftaran). Dependensi minimal: repo + storage + audit.

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/kipan-indonesia/sim-kipan-core/config"
	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/gateway"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
	"github.com/kipan-indonesia/sim-kipan-core/internal/service/dokumen"
	"github.com/kipan-indonesia/sim-kipan-core/internal/service/mail"
	"github.com/kipan-indonesia/sim-kipan-core/internal/service/svcutil"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/crypto"
)

type RevisionService interface {
	RequestRevisionToken(ctx context.Context, req domain.RevisionTokenRequest, audit domain.AuditContext) (*domain.RevisionTokenResponse, error)
	SubmitRevision(ctx context.Context, nomor string, req domain.RevisionSubmitRequest, audit domain.AuditContext) error
}

// RevisionDeps adalah dependensi service revisi (R1: pola deps).
type RevisionDeps struct {
	Repo       repository.PendaftaranRevisionRepository
	StorageSvc dokumen.ObjectVerifier
	AuditRepo  repository.AuditLogRepository
	Mail       gateway.MailSender
}

type revisionSvc struct {
	cfg        *config.Config
	repo       repository.PendaftaranRevisionRepository
	storageSvc dokumen.ObjectVerifier
	auditRepo  repository.AuditLogRepository
	mail       gateway.MailSender
}

func NewRevisionService(cfg *config.Config, deps RevisionDeps) RevisionService {
	return &revisionSvc{
		cfg: cfg, repo: deps.Repo, storageSvc: deps.StorageSvc,
		auditRepo: deps.AuditRepo, mail: deps.Mail,
	}
}

// RevisionTokenTTL adalah masa berlaku token revisi applicant.
const RevisionTokenTTL = 24 * time.Hour

// MatchOwnerProof mencocokkan bukti pemilik (email DAN whatsapp) terhadap
// data terdaftar. Pure function agar unit-testable. Kedua sisi
// dinormalisasi; bukti kosong selalu gagal.
func MatchOwnerProof(storedEmail, storedWA, proofEmail, proofWA string) bool {
	pe, pw := svcutil.NormalizeEmail(proofEmail), svcutil.NormalizeWA(proofWA)
	if pe == "" || pw == "" {
		return false
	}
	return svcutil.NormalizeEmail(storedEmail) == pe && svcutil.NormalizeWA(storedWA) == pw
}

// RequestRevisionToken menerbitkan token revisi satu-permintaan untuk
// pendaftaran berstatus PERBAIKAN. Wajib bukti pemilik (email DAN whatsapp
// terdaftar — BE-001): nomor saja tidak cukup karena sekuensial dan
// statusnya publik. Token mentah dikembalikan sekali; yang disimpan hanya
// hash SHA-256 + expiry.
//
// RequestRevisionToken menerbitkan token revisi satu-permintaan untuk
// pendaftaran berstatus PERBAIKAN. Wajib bukti pemilik (email DAN whatsapp
// terdaftar — BE-001): nomor saja tidak cukup karena sekuensial dan
// statusnya publik. Token mentah TIDAK lagi dikembalikan lewat respons API
// (Batch 3, Opsi B): dikirim ke EMAIL terdaftar; yang disimpan hanya hash
// SHA-256 + expiry.
func (s *revisionSvc) RequestRevisionToken(ctx context.Context, req domain.RevisionTokenRequest, audit domain.AuditContext) (*domain.RevisionTokenResponse, error) {
	nr, err := svcutil.NormalizeNomor(req.Nomor)
	if err != nil {
		return nil, err
	}
	if s.repo == nil {
		return nil, svcutil.Unavailable("pendaftaran")
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

	// Opsi B: kirim token ke email terdaftar (kanal terverifikasi).
	s.sendRevisionTokenEmail(item.NamaLengkap, item.Email, nr, raw)

	s.auditEvent(ctx, audit, nil, "Pendaftar "+nr, "PUBLIK",
		"pendaftaran", strconv.Itoa(item.ID), "REVISI_TOKEN", nil)
	return &domain.RevisionTokenResponse{Token: raw, ExpiresAt: expiresAt}, nil
}

// sendRevisionTokenEmail mengirim token ke email pendaftar (best-effort).
func (s *revisionSvc) sendRevisionTokenEmail(nama, email, nomor, token string) {
	if s.mail == nil || strings.TrimSpace(email) == "" {
		return
	}
	content := mail.RevisionTokenEmail(nama, nomor, token, svcutil.PublicURLFrom(s.cfg))
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := s.mail.Send(ctx, email, content.Subject, content.TextBody, content.HTMLBody); err != nil {
		log.Warn().Err(err).Str("nomor", nomor).Msg("Gagal mengirim token revisi via email")
	}
}

// SubmitRevision memproses revisi mandiri applicant: token valid +
// belum kedaluwarsa + state PERBAIKAN (satu UPDATE atomik, error generik
// tanpa oracle), dokumen baru tervalidasi + terverifikasi storage,
// status kembali DRAFT, token hangus sekali pakai.
func (s *revisionSvc) SubmitRevision(ctx context.Context, nomor string, req domain.RevisionSubmitRequest, audit domain.AuditContext) error {
	nr, err := svcutil.NormalizeNomor(nomor)
	if err != nil {
		return err
	}
	token := strings.TrimSpace(req.Token)
	if token == "" || len(token) > 256 {
		return domain.NewValidationError("Token revisi wajib diisi")
	}
	if s.repo == nil {
		return svcutil.Unavailable("pendaftaran")
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
		// Batch Kader/Pengurus: foto, KTP, CV, surat pernyataan, surat sehat
		// selalu wajib (selaras submit); SK wajib hanya untuk jalur
		// PENGURUS mengikuti tipe tersimpan (kader tidak melampirkan SK).
		skWajib := item.Tipe == domain.TipePendaftaranPengurus
		if strings.TrimSpace(k) == "" && (name == "foto_key" || name == "ktp_key" ||
			name == "cv_key" || name == "surat_pernyataan_key" || name == "surat_sehat_key" ||
			(name == "sk_key" && skWajib)) {
			if name == "sk_key" {
				return domain.NewValidationError("Pendaftaran Pengurus wajib melampirkan SK")
			}
			return domain.NewValidationError("Dokumen " + name + " wajib ada")
		}
		keys[name] = k
	}

	// L7: hasil merge akhir tidak boleh memakai satu key di >1 slot
	// (mis. revisi menunjuk foto ke key KTP yang sudah ada).
	if err := checkDuplicateDocKeys([]docSlot{
		{"Foto", keys["foto_key"]},
		{"KTP", keys["ktp_key"]},
		{"CV", keys["cv_key"]},
		{"SK", keys["sk_key"]},
		{"Surat pernyataan", keys["surat_pernyataan_key"]},
		{"Surat sehat", keys["surat_sehat_key"]},
	}); err != nil {
		return err
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
	svcutil.WriteAudit(ctx, s.auditRepo, audit, actorID, actorName, actorRole, entity, entityID, action, metadata)
}
