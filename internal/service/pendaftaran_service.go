package service

// pendaftaran_service.go menyusun layanan pendaftaran dari dua service fokus
// (Submit & Query) yang berbagi dependensi via `pendaftaranBase`, plus helper
// validasi murni. Handler cukup bergantung pada agregat `PendaftaranService`.

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/kipan-indonesia/sim-kipan-core/config"
	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/gateway"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
	"github.com/kipan-indonesia/sim-kipan-core/internal/service/dokumen"
	"github.com/kipan-indonesia/sim-kipan-core/internal/service/notify"
	"github.com/kipan-indonesia/sim-kipan-core/internal/service/svcutil"
)

// PendaftaranService (agregat) menggabungkan Submit & Query (Fase A4, SRP).
type PendaftaranService interface {
	PendaftaranSubmitService
	PendaftaranQueryService
}

// PendaftaranDeps adalah dependensi service pendaftaran inti (R1+R3).
// Field nil-able; service memeriksa nil dan gagal fail-closed per fitur.
type PendaftaranDeps struct {
	Repo        repository.PendaftaranCoreRepository
	AnggotaRepo repository.AnggotaRepository
	AuditRepo   repository.AuditLogRepository
	StorageSvc  dokumen.ObjectVerifier
	WilayahRepo repository.WilayahRepository
	NotifRepo   repository.NotificationRepository
	OTPSvc      notify.OTPService
	WAGateway   gateway.WAGateway
	ListRepo    repository.ListKeysetRepository
	OutboxRepo  repository.EmailOutboxRepository
}

// pendaftaranBase menampung dependensi bersama + helper lintas sub-service.
type pendaftaranBase struct {
	cfg         *config.Config
	repo        repository.PendaftaranCoreRepository
	anggotaRepo repository.AnggotaRepository
	auditRepo   repository.AuditLogRepository
	storageSvc  dokumen.ObjectVerifier
	wilayahRepo repository.WilayahRepository
	notifRepo   repository.NotificationRepository
	otpSvc      notify.OTPService
	waGateway   gateway.WAGateway
	listRepo    repository.ListKeysetRepository
	outboxRepo  repository.EmailOutboxRepository
}

// pendaftaranAggregate menyalurkan method ke sub-service terkait.
type pendaftaranAggregate struct {
	PendaftaranSubmitService
	PendaftaranQueryService
}

// NewPendaftaranService membangun base bersama + dua sub-service fokus.
func NewPendaftaranService(cfg *config.Config, deps PendaftaranDeps) PendaftaranService {
	base := &pendaftaranBase{
		cfg:         cfg,
		repo:        deps.Repo,
		anggotaRepo: deps.AnggotaRepo,
		auditRepo:   deps.AuditRepo,
		storageSvc:  deps.StorageSvc,
		wilayahRepo: deps.WilayahRepo,
		notifRepo:   deps.NotifRepo,
		otpSvc:      deps.OTPSvc,
		waGateway:   deps.WAGateway,
		listRepo:    deps.ListRepo,
		outboxRepo:  deps.OutboxRepo,
	}
	return &pendaftaranAggregate{
		PendaftaranSubmitService: &pendaftaranSubmitSvc{pendaftaranBase: base},
		PendaftaranQueryService:  &pendaftaranQuerySvc{pendaftaranBase: base},
	}
}

// objectKeyPattern allowlist pola S3 object key (anti path-traversal dan
// key lintas namespace).
var objectKeyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9/_\.\-]{0,254}$`)

// Batas panjang field selaras kolom database (anti-DoS + 422, bukan 500).
const (
	maxEmailLen         = 255
	maxWhatsappLen      = 25
	maxKecamatanDesaLen = 100
	maxMotivasiLen      = 1000
	minMotivasiLen      = 20
	maxBebasLen         = 100
	minPendaftarAge     = 16
	maxPendaftarAge     = 30
)

// docSlot adalah pasangan label + key satu slot dokumen.
type docSlot struct {
	label string
	key   string
}

// docKeysOf mengekstrak 6 slot dokumen dari request submit.
func docKeysOf(req domain.PendaftaranSubmitRequest) []docSlot {
	return []docSlot{
		{"Foto", req.FotoKey},
		{"KTP", req.KTPKey},
		{"CV", req.CVKey},
		{"SK", req.SKKey},
		{"Surat pernyataan", req.SuratPernyataanKey},
		{"Surat sehat", req.SuratSehatKey},
	}
}

// checkDuplicateDocKeys menolak satu object key yang dipakai di >1 slot.
func checkDuplicateDocKeys(slots []docSlot) error {
	seen := make(map[string]string, len(slots))
	for _, f := range slots {
		k := strings.TrimSpace(f.key)
		if k == "" {
			continue
		}
		if prev, ok := seen[k]; ok {
			return domain.NewValidationError("Dokumen " + prev + " dan " + f.label + " tidak boleh memakai berkas yang sama")
		}
		seen[k] = f.label
	}
	return nil
}

// checkPersyaratan memvalidasi checklist persyaratan pendaftar.
func checkPersyaratan(items []string) error {
	if len(items) > 20 {
		return domain.NewValidationError("Persyaratan maksimal 20 item")
	}
	for _, it := range items {
		t := strings.TrimSpace(it)
		if t == "" || len([]rune(t)) > maxBebasLen {
			return domain.NewValidationError("Item persyaratan wajib 1-100 karakter")
		}
		if containsAngleBracket(t) {
			return domain.NewValidationError("Item persyaratan tidak boleh mengandung karakter < atau >")
		}
	}
	return nil
}

// marshalPersyaratan menyeragamkan checklist (trim) lalu mengenkode ke JSON array.
func marshalPersyaratan(items []string) (string, error) {
	clean := make([]string, 0, len(items))
	for _, it := range items {
		if t := strings.TrimSpace(it); t != "" {
			clean = append(clean, t)
		}
	}
	if clean == nil {
		clean = []string{}
	}
	raw, err := json.Marshal(clean)
	if err != nil {
		return "", domain.NewValidationError("Checklist persyaratan tidak valid")
	}
	return string(raw), nil
}

// normalizeTipePendaftaran menyeragamkan pilihan jalur (case-insensitive).
func normalizeTipePendaftaran(raw string) (domain.TipePendaftaran, error) {
	t := domain.TipePendaftaran(strings.ToUpper(strings.TrimSpace(raw)))
	if !t.IsValid() {
		return "", domain.NewValidationError("Tipe pendaftaran harus KADER atau PENGURUS")
	}
	return t, nil
}

// containsAngleBracket menolak < > pada field plain-text (anti stored-XSS).
func containsAngleBracket(s string) bool {
	return strings.ContainsAny(s, "<>")
}

// isPlausibleNIKDate memeriksa kewarasan segmen tanggal NIK (digit 7-12).
func isPlausibleNIKDate(nik string) bool {
	if len(nik) != 16 {
		return false
	}
	dd := int(nik[6]-'0')*10 + int(nik[7]-'0')
	mm := int(nik[8]-'0')*10 + int(nik[9]-'0')
	if mm < 1 || mm > 12 {
		return false
	}
	if (dd >= 1 && dd <= 31) || (dd >= 41 && dd <= 71) {
		return true
	}
	return false
}

// checkPendaftarAge menolak tanggal masa depan dan umur di luar 16-30 tahun.
func checkPendaftarAge(dob, now time.Time) error {
	if dob.After(now) {
		return domain.NewValidationError("Tanggal lahir tidak boleh di masa depan")
	}
	age := now.Year() - dob.Year()
	if now.YearDay() < dob.YearDay() {
		age--
	}
	if age < minPendaftarAge || age > maxPendaftarAge {
		return domain.NewValidationError("Usia pendaftar harus 16-30 tahun")
	}
	return nil
}

// checkObjectKey memvalidasi pola S3 object key (allowlist anti traversal).
func checkObjectKey(label, key string, required bool) error {
	k := strings.TrimSpace(key)
	if k == "" {
		if required {
			return domain.NewValidationError(label + " wajib diunggah")
		}
		return nil
	}
	if len(k) > 255 || !objectKeyPattern.MatchString(k) || strings.Contains(k, "..") {
		return domain.NewValidationError("Object key " + label + " tidak valid")
	}
	return nil
}

// verifyOneDocument memverifikasi satu object via storage (R3: fungsi paket
// agar dipakai inti + revisi). Storage mati: 503 di production, lewati di non-prod.
func verifyOneDocument(ctx context.Context, cfg *config.Config, storageSvc dokumen.ObjectVerifier, key, category string) error {
	if storageSvc == nil || !storageSvc.Configured() {
		if svcutil.DegradedSkip(cfg, "storage(verifikasi-dokumen:"+category+")") {
			return nil
		}
		return svcutil.Unavailable("verifikasi dokumen")
	}
	return storageSvc.VerifySubmittedObject(ctx, key, category)
}

// verifySubmittedDocuments memverifikasi setiap dokumen yang diklaim ter-upload.
func (b *pendaftaranBase) verifySubmittedDocuments(ctx context.Context, req domain.PendaftaranSubmitRequest) error {
	docs := []struct {
		category string
		key      string
		required bool
	}{
		{"foto", req.FotoKey, true},
		{"ktp", req.KTPKey, true},
		{"cv", req.CVKey, true},
		{"sk", req.SKKey, false},
		{"surat_pernyataan", req.SuratPernyataanKey, true},
		{"surat_sehat", req.SuratSehatKey, true},
	}
	for _, d := range docs {
		k := strings.TrimSpace(d.key)
		if k == "" {
			continue // required sudah ditegakkan ValidateSubmitRequest
		}
		if err := verifyOneDocument(ctx, b.cfg, b.storageSvc, k, d.category); err != nil {
			return err
		}
	}
	return nil
}

// validateWilayah memastikan provinsi/kabupaten ada, aktif, dan berelasi benar.
func (b *pendaftaranBase) validateWilayah(ctx context.Context, provinsiID, kabupatenID int) error {
	if b.wilayahRepo == nil {
		if svcutil.DegradedSkip(b.cfg, "wilayahRepo") {
			return nil
		}
		return svcutil.Unavailable("validasi wilayah")
	}
	ok, err := b.wilayahRepo.ExistsProvinsi(ctx, provinsiID)
	if err != nil {
		return err
	}
	if !ok {
		return domain.NewValidationError("Provinsi tidak valid atau tidak aktif")
	}
	ok, err = b.wilayahRepo.KabupatenInProvinsi(ctx, kabupatenID, provinsiID)
	if err != nil {
		return err
	}
	if !ok {
		return domain.NewValidationError("Kabupaten tidak valid untuk provinsi tersebut")
	}
	return nil
}

// notifyAdmins menyebarkan notifikasi ke admin terkait (best-effort).
func (b *pendaftaranBase) notifyAdmins(ctx context.Context, title, message string, notifType domain.NotificationType, link string, provID, kabID int) {
	if b.notifRepo == nil {
		return
	}
	if err := b.notifRepo.NotifyAdmins(ctx, title, message, notifType, link, provID, kabID); err != nil {
		log.Warn().Err(err).Msg("gagal menyebar notifikasi pendaftaran")
	}
}

// publicURL mengembalikan base URL frontend untuk tautan pesan.
func (b *pendaftaranBase) publicURL() string {
	return svcutil.PublicURLFrom(b.cfg)
}

// registrantWAMessage menyusun teks WhatsApp berisi nomor pendaftaran + tautan lacak.
func registrantWAMessage(nama, nomor, publicURL string) string {
	return "Halo " + nama + ",\n\n" +
		"Pendaftaran KIPAN Anda telah kami terima.\n" +
		"Nomor Pendaftaran: " + nomor + "\n\n" +
		"Lacak status: " + publicURL + "/lacak?nomor=" + nomor + "\n" +
		"Simpan nomor ini untuk revisi berkas dan verifikasi KTA.\n\n" +
		"— Sistem Informasi KIPAN RI"
}

// notifyRegistrantWA mengirim nomor REG via WhatsApp (async best-effort).
func (b *pendaftaranBase) notifyRegistrantWA(whatsapp, nama, nomor string) {
	if b.waGateway == nil || strings.TrimSpace(whatsapp) == "" {
		return
	}
	text := registrantWAMessage(nama, nomor, b.publicURL())
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()
		if err := b.waGateway.SendMessage(ctx, whatsapp, text); err != nil {
			log.Warn().Err(err).Str("nomor", nomor).
				Msg("Gagal mengirim nomor pendaftaran via WhatsApp")
		}
	}()
}

// auditEvent mendelegasikan ke writeAudit terpusat (R2).
func (b *pendaftaranBase) auditEvent(
	ctx context.Context,
	audit domain.AuditContext,
	actorID *string,
	actorName, actorRole, entity, entityID, action string,
	metadata *string,
) {
	svcutil.WriteAudit(ctx, b.auditRepo, audit, actorID, actorName, actorRole, entity, entityID, action, metadata)
}

// aesKey/blindIndexKey membaca kunci kripto dari config (R3: fungsi paket).
func aesKey(cfg *config.Config) (string, error) {
	if cfg == nil || strings.TrimSpace(cfg.Crypto.AESMasterKey) == "" {
		return "", domain.NewValidationError("AES_MASTER_KEY belum dikonfigurasi")
	}
	return cfg.Crypto.AESMasterKey, nil
}

func blindIndexKey(cfg *config.Config) (string, error) {
	if cfg == nil || strings.TrimSpace(cfg.Crypto.BlindIndexKey) == "" {
		return "", domain.NewValidationError("BLIND_INDEX_KEY belum dikonfigurasi")
	}
	return cfg.Crypto.BlindIndexKey, nil
}
