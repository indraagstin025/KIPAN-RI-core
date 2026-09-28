package service

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/kipan-indonesia/sim-kipan-core/config"
	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/crypto"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/generator"
)

// PendaftaranService berisi business logic pendaftaran calon anggota.
//
// Aturan otorisasi (RULES 5, 6, 7): setiap aksi admin menerima actor
// (identitas server-side dari JWT) dan menegakkan jurisdiction di sini.
// Handler tidak boleh meneruskan role/wilayah dari client.
type PendaftaranService interface {
	ValidateSubmitRequest(req domain.PendaftaranSubmitRequest) error
	BuildRegistrationNumber(ctx context.Context, year, month int) (string, error)
	GenerateBlindIndex(nik string) (string, error)
	EncryptNIK(nik string) (string, error)
	CreateRegistration(ctx context.Context, req domain.PendaftaranSubmitRequest, audit domain.AuditContext) (*domain.PendaftaranCreateResult, error)
	// GetTracking adalah jalur publik: kembalikan DTO minimal tanpa PII.
	GetTracking(ctx context.Context, nomor string) (*domain.PendaftaranTrackingResponse, error)
	// VerifyKTA memverifikasi keaslian KTA: selalu 200 + verdict (tanpa
	// oracle bedakan NIA tak dikenal vs signature salah).
	VerifyKTA(ctx context.Context, nia, sig string) (*domain.KTAVerificationResponse, error)
	// ListQueue adalah antrean admin terfilter jurisdiction aktor.
	ListQueue(ctx context.Context, actor domain.ActorContext, status string, page, limit int) ([]domain.PendaftaranQueueItem, int, error)
	// RequestRevisionToken menerbitkan token revisi untuk status PERBAIKAN.
	RequestRevisionToken(ctx context.Context, req domain.RevisionTokenRequest, audit domain.AuditContext) (*domain.RevisionTokenResponse, error)
	// SubmitRevision memproses revisi mandiri applicant bertoken.
	SubmitRevision(ctx context.Context, nomor string, req domain.RevisionSubmitRequest, audit domain.AuditContext) error
	// GetDetail adalah jalur admin: tolak objek di luar wilayah aktor.
	GetDetail(ctx context.Context, id int, actor domain.ActorContext) (*domain.Pendaftaran, error)
	ProcessApproval(ctx context.Context, id int, action domain.PendaftaranApprovalAction, catatan string, actor domain.ActorContext, audit domain.AuditContext) error
}

type pendaftaranService struct {
	cfg         *config.Config
	repo        repository.PendaftaranRepository
	anggotaRepo repository.AnggotaRepository
	auditRepo   repository.AuditLogRepository
	storageSvc  ObjectVerifier
	wilayahRepo repository.WilayahRepository
}

func NewPendaftaranService(cfg *config.Config, repo repository.PendaftaranRepository, anggotaRepo repository.AnggotaRepository, auditRepo repository.AuditLogRepository, storageSvc ObjectVerifier, wilayahRepo repository.WilayahRepository) PendaftaranService {
	return &pendaftaranService{cfg: cfg, repo: repo, anggotaRepo: anggotaRepo, auditRepo: auditRepo, storageSvc: storageSvc, wilayahRepo: wilayahRepo}
}

var nipPattern = regexp.MustCompile(`^\d{16}$`)

// phonePattern selaras dengan rule id_phone di pkg/validator: prefix
// 0/62/+62, operator 8, digit kedua bukan 0, total 9-14 digit.
var phonePattern = regexp.MustCompile(`^(\+62|62|0)8[1-9][0-9]{6,10}$`)

// objectKeyPattern allowlist pola S3 object key (anti path-traversal dan
// key lintas namespace). Verifikasi keberadaan object (HeadObject) menyusul
// di batch storage presign.
var objectKeyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9/_\.\-]{0,254}$`)

// Batas panjang field selaras kolom database (anti-DoS + 422, bukan 500).
const (
	maxNamaLen          = 150
	maxTempatLahirLen   = 100
	maxAlamatLen        = 2000
	maxEmailLen         = 255
	maxWhatsappLen      = 25
	maxKecamatanDesaLen = 100
	maxKodePosLen       = 10
	maxMotivasiLen      = 1000
	maxBebasLen         = 100
	minPendaftarAge     = 17
	maxPendaftarAge     = 100
)

// ValidateSubmitRequest memastikan payload pendaftaran aman dan valid sebelum masuk DB.
// Single source of truth validasi server-side (DTO sengaja tanpa tag agar tidak divergen).
func (s *pendaftaranService) ValidateSubmitRequest(req domain.PendaftaranSubmitRequest) error {
	nama := strings.TrimSpace(req.NamaLengkap)
	if len([]rune(nama)) < 3 || len([]rune(nama)) > maxNamaLen {
		return domain.NewValidationError("Nama lengkap wajib 3-150 karakter")
	}
	if containsAngleBracket(nama) {
		return domain.NewValidationError("Nama lengkap tidak boleh mengandung karakter < atau >")
	}
	nik := strings.TrimSpace(req.NIK)
	if !nipPattern.MatchString(nik) {
		return domain.NewValidationError("NIK harus berisi 16 digit angka")
	}
	if !isPlausibleNIKDate(nik) {
		return domain.NewValidationError("Segmen tanggal lahir pada NIK tidak valid")
	}
	tempat := strings.TrimSpace(req.TempatLahir)
	if tempat == "" || len([]rune(tempat)) > maxTempatLahirLen {
		return domain.NewValidationError("Tempat lahir wajib diisi (maks 100 karakter)")
	}
	if containsAngleBracket(tempat) {
		return domain.NewValidationError("Tempat lahir tidak boleh mengandung karakter < atau >")
	}
	dob, err := time.Parse(time.RFC3339, strings.TrimSpace(req.TanggalLahir))
	if err != nil {
		return domain.NewValidationError("Format tanggal lahir tidak valid")
	}
	if err := checkPendaftarAge(dob, time.Now()); err != nil {
		return err
	}
	if strings.TrimSpace(req.JenisKelamin) == "" || (req.JenisKelamin != "L" && req.JenisKelamin != "P") {
		return domain.NewValidationError("Jenis kelamin harus L atau P")
	}
	alamat := strings.TrimSpace(req.Alamat)
	if len([]rune(alamat)) < 10 || len([]rune(alamat)) > maxAlamatLen {
		return domain.NewValidationError("Alamat wajib 10-2000 karakter")
	}
	if containsAngleBracket(alamat) {
		return domain.NewValidationError("Alamat tidak boleh mengandung karakter < atau >")
	}
	if req.ProvinsiID <= 0 || req.KabupatenID <= 0 {
		return domain.NewValidationError("Wilayah provinsi dan kabupaten wajib dipilih")
	}
	email := strings.TrimSpace(req.Email)
	if len(email) < 5 || len(email) > maxEmailLen {
		return domain.NewValidationError("Email wajib 5-255 karakter")
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return domain.NewValidationError("Format email tidak valid")
	}
	wa := strings.TrimSpace(req.Whatsapp)
	if len(wa) > maxWhatsappLen || !phonePattern.MatchString(wa) {
		return domain.NewValidationError("Nomor WhatsApp tidak valid (contoh: 081234567890)")
	}
	if err := checkObjectKey("Foto", req.FotoKey, true); err != nil {
		return err
	}
	if err := checkObjectKey("KTP", req.KTPKey, true); err != nil {
		return err
	}
	for _, f := range []struct {
		label string
		key   string
	}{
		{"CV", req.CVKey},
		{"SK", req.SKKey},
		{"Surat pernyataan", req.SuratPernyataanKey},
		{"Surat sehat", req.SuratSehatKey},
	} {
		if err := checkObjectKey(f.label, f.key, false); err != nil {
			return err
		}
	}
	if len([]rune(req.Motivasi)) > maxMotivasiLen {
		return domain.NewValidationError("Motivasi maksimal 1000 karakter")
	}
	if containsAngleBracket(req.Motivasi) {
		return domain.NewValidationError("Motivasi tidak boleh mengandung karakter < atau >")
	}
	for _, f := range []struct {
		label string
		val   string
		limit int
	}{
		{"Agama", req.Agama, maxBebasLen},
		{"Pendidikan", req.Pendidikan, maxBebasLen},
		{"Pekerjaan", req.Pekerjaan, maxBebasLen},
		{"Status pribadi", req.StatusPribadi, maxBebasLen},
		{"Kecamatan", req.Kecamatan, maxKecamatanDesaLen},
		{"Desa", req.Desa, maxKecamatanDesaLen},
	} {
		if len([]rune(strings.TrimSpace(f.val))) > f.limit {
			return domain.NewValidationError(f.label + " melebihi batas karakter")
		}
	}
	if len(strings.TrimSpace(req.KodePos)) > maxKodePosLen {
		return domain.NewValidationError("Kode pos maksimal 10 karakter")
	}
	return nil
}

// containsAngleBracket menolak < > pada field plain-text (anti stored-XSS;
// nama/alamat yang sah tidak pernah mengandung angle bracket).
func containsAngleBracket(s string) bool {
	return strings.ContainsAny(s, "<>")
}

// isPlausibleNIKDate memeriksa kewarasan segmen tanggal NIK (digit 7-12 =
// DDMMYY; hari 01-31 atau 41-71 untuk perempuan, bulan 01-12). NIK tidak
// punya checksum resmi sehingga ini batas maksimal yang bisa divalidasi.
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

// checkPendaftarAge menolak tanggal masa depan dan umur di luar 17-100 tahun.
func checkPendaftarAge(dob, now time.Time) error {
	if dob.After(now) {
		return domain.NewValidationError("Tanggal lahir tidak boleh di masa depan")
	}
	age := now.Year() - dob.Year()
	if now.YearDay() < dob.YearDay() {
		age--
	}
	if age < minPendaftarAge || age > maxPendaftarAge {
		return domain.NewValidationError("Usia pendaftar harus 17-100 tahun")
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

// BuildRegistrationNumber mengalokasikan sequence periode dari database lalu
// memformat nomor REG-YYYYMM-XXXXX. Sequence dari DB menjamin atomisitas.
func (s *pendaftaranService) BuildRegistrationNumber(ctx context.Context, year, month int) (string, error) {
	if year <= 0 || month < 1 || month > 12 {
		return "", domain.NewValidationError("Periode pendaftaran tidak valid")
	}
	if s.repo == nil {
		return "", domain.NewValidationError("Repository pendaftaran belum tersedia")
	}
	seq, err := s.repo.NextRegistrationSequence(ctx, year, month)
	if err != nil {
		return "", err
	}
	return generator.GenerateRegistrationNumber(year, month, seq)
}

// GenerateBlindIndex mengubah NIK ternormalisasi menjadi blind index
// HMAC-SHA256 untuk deteksi duplikasi aman (RULES 10).
func (s *pendaftaranService) GenerateBlindIndex(nik string) (string, error) {
	key, err := s.getBlindIndexKey()
	if err != nil {
		return "", err
	}
	return crypto.BlindIndex(strings.TrimSpace(nik), key)
}

// EncryptNIK mengenkripsi NIK ternormalisasi agar tidak pernah disimpan plaintext.
func (s *pendaftaranService) EncryptNIK(nik string) (string, error) {
	key, err := s.getAESKey()
	if err != nil {
		return "", err
	}
	return crypto.EncryptAESGCM(strings.TrimSpace(nik), key)
}

// CreateRegistration menyimpan pendaftaran baru: validasi → cek duplikat NIK
// di DUA tabel (pendaftaran + anggota) → enkripsi → alokasi nomor → insert
// atomik beserta riwayat SUBMIT. Duplikat dikembalikan sebagai 409.
func (s *pendaftaranService) CreateRegistration(ctx context.Context, req domain.PendaftaranSubmitRequest, audit domain.AuditContext) (*domain.PendaftaranCreateResult, error) {
	if err := s.ValidateSubmitRequest(req); err != nil {
		return nil, err
	}
	if s.repo == nil {
		return nil, domain.NewValidationError("Repository pendaftaran belum tersedia")
	}

	nik := strings.TrimSpace(req.NIK)
	blindIndex, err := s.GenerateBlindIndex(nik)
	if err != nil {
		return nil, err
	}
	if _, err := s.repo.GetByNikHash(ctx, blindIndex); err == nil {
		return nil, domain.NewConflictError("NIK sudah terdaftar")
	} else if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	if s.anggotaRepo != nil {
		exists, err := s.anggotaRepo.ExistsByNikHash(ctx, blindIndex)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, domain.NewConflictError("NIK sudah terdaftar sebagai anggota")
		}
	}
	nikEncrypted, err := s.EncryptNIK(nik)
	if err != nil {
		return nil, err
	}

	// Validasi master wilayah server-side (RULES 6): provinsi ada + aktif,
	// kabupaten milik provinsi tersebut + aktif.
	if err := s.validateWilayah(ctx, req.ProvinsiID, req.KabupatenID); err != nil {
		return nil, err
	}

	// Verifikasi keberadaan + integritas dokumen di storage SEBELUM nomor
	// dialokasikan (RULES 14): jangan bakar sequence untuk submit cacat.
	if err := s.verifySubmittedDocuments(ctx, req); err != nil {
		return nil, err
	}

	dob, err := time.Parse(time.RFC3339, strings.TrimSpace(req.TanggalLahir))
	if err != nil {
		return nil, domain.NewValidationError("Format tanggal lahir tidak valid")
	}
	now := time.Now()

	// Retry sekali untuk perebutan nomor (UNIQUE backstop). 409 final tetap
	// dikembalikan apa adanya (bisa jadi balapan NIK — tetap konflik valid).
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		seq, err := s.repo.NextRegistrationSequence(ctx, now.Year(), int(now.Month()))
		if err != nil {
			return nil, err
		}
		number, err := generator.GenerateRegistrationNumber(now.Year(), int(now.Month()), seq)
		if err != nil {
			return nil, err
		}
		entity := &domain.Pendaftaran{
			NomorPendaftaran:   number,
			NamaLengkap:        strings.TrimSpace(req.NamaLengkap),
			NIKHash:            blindIndex,
			NIKEncrypted:       nikEncrypted,
			TempatLahir:        strings.TrimSpace(req.TempatLahir),
			TanggalLahir:       dob,
			JenisKelamin:       strings.TrimSpace(req.JenisKelamin),
			Agama:              strings.TrimSpace(req.Agama),
			Pendidikan:         strings.TrimSpace(req.Pendidikan),
			Pekerjaan:          strings.TrimSpace(req.Pekerjaan),
			StatusPribadi:      strings.TrimSpace(req.StatusPribadi),
			Alamat:             strings.TrimSpace(req.Alamat),
			ProvinsiID:         req.ProvinsiID,
			KabupatenID:        req.KabupatenID,
			Kecamatan:          strings.TrimSpace(req.Kecamatan),
			Desa:               strings.TrimSpace(req.Desa),
			KodePos:            strings.TrimSpace(req.KodePos),
			Email:              strings.TrimSpace(req.Email),
			Whatsapp:           strings.TrimSpace(req.Whatsapp),
			Motivasi:           strings.TrimSpace(req.Motivasi),
			FotoKey:            strings.TrimSpace(req.FotoKey),
			KTPKey:             strings.TrimSpace(req.KTPKey),
			CVKey:              strings.TrimSpace(req.CVKey),
			SKKey:              strings.TrimSpace(req.SKKey),
			SuratPernyataanKey: strings.TrimSpace(req.SuratPernyataanKey),
			SuratSehatKey:      strings.TrimSpace(req.SuratSehatKey),
			Status:             domain.PendaftaranStatusDiajukan,
		}
		if err := s.repo.CreateWithHistory(ctx, entity, "SUBMIT", "Pendaftaran mandiri diterima"); err != nil {
			var appErr *domain.AppError
			if errors.As(err, &appErr) && appErr.Code == 409 && attempt == 0 {
				lastErr = err
				continue
			}
			return nil, err
		}
		// Jejak audit: aktor adalah pendaftar publik (tanpa akun).
		s.auditEvent(ctx, audit, nil, entity.NamaLengkap, "PUBLIK",
			"pendaftaran", strconv.Itoa(entity.ID), "SUBMIT", nil)
		return &domain.PendaftaranCreateResult{
			ID:               entity.ID,
			NomorPendaftaran: number,
			Status:           string(domain.PendaftaranStatusDiajukan),
		}, nil
	}
	return nil, lastErr
}

// GetTracking melayani pelacakan publik dengan DTO minimal: nomor, status,
// dan timestamp. Tanpa nama, kontak, alamat, maupun object key (RULES 12).
func (s *pendaftaranService) GetTracking(ctx context.Context, nomor string) (*domain.PendaftaranTrackingResponse, error) {
	nr := strings.TrimSpace(nomor)
	if nr == "" || len(nr) > 30 {
		return nil, domain.NewValidationError("Nomor pendaftaran tidak valid")
	}
	if s.repo == nil {
		return nil, domain.NewValidationError("Repository pendaftaran belum tersedia")
	}
	item, err := s.repo.GetByNomorPendaftaran(ctx, nr)
	if err != nil {
		return nil, err
	}
	return &domain.PendaftaranTrackingResponse{
		NomorPendaftaran: item.NomorPendaftaran,
		Status:          string(item.Status),
		CreatedAt:       item.CreatedAt,
		UpdatedAt:       item.UpdatedAt,
	}, nil
}

// GetDetail melayani admin: tolak objek di luar wilayah kerja aktor (RULES 7).
func (s *pendaftaranService) GetDetail(ctx context.Context, id int, actor domain.ActorContext) (*domain.Pendaftaran, error) {
	if id <= 0 {
		return nil, domain.NewValidationError("ID pendaftaran tidak valid")
	}
	if s.repo == nil {
		return nil, domain.NewValidationError("Repository pendaftaran belum tersedia")
	}
	item, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !actor.CanAccessWilayah(item.ProvinsiID, item.KabupatenID) {
		return nil, domain.NewForbiddenError("Pendaftaran di luar wilayah kerja Anda")
	}
	return item, nil
}

// ProcessApproval memvalidasi otorisasi + jurisdiction + transisi status,
// lalu mengeksekusi secara atomik beserta riwayat beraktor dan audit trail.
// actor WAJIB berasal dari JWT terverifikasi (RULES 6), bukan dari client.
func (s *pendaftaranService) ProcessApproval(ctx context.Context, id int, action domain.PendaftaranApprovalAction, catatan string, actor domain.ActorContext, audit domain.AuditContext) error {
	if id <= 0 {
		return domain.NewValidationError("ID pendaftaran tidak valid")
	}
	if s.repo == nil {
		return domain.NewUnavailableError("Layanan pendaftaran sedang tidak tersedia")
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
		if _, err := s.repo.IssueMember(ctx, id, time.Now().Year(), s.cfg.Crypto.KTASigningKey); err != nil {
			return err
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

// VerifyKTA memverifikasi keaslian KTA secara kriptografis (RULES 20).
// NIA tak dikenal dan signature salah menghasilkan verdict valid=false yang
// sama (tanpa oracle). Hanya input kosong yang ditolak sebagai 422.
func (s *pendaftaranService) VerifyKTA(ctx context.Context, nia, sig string) (*domain.KTAVerificationResponse, error) {
	code := strings.TrimSpace(nia)
	signature := strings.TrimSpace(sig)
	if code == "" || len(code) > 50 || len(signature) > 128 {
		return nil, domain.NewValidationError("Parameter verifikasi KTA tidak valid")
	}
	if s.anggotaRepo == nil {
		return nil, domain.NewUnavailableError("Layanan anggota sedang tidak tersedia")
	}
	member, err := s.anggotaRepo.GetByNIA(ctx, code)
	if err != nil {
		// NIA tak dikenal = verdict tidak valid (bukan 404, anti oracle).
		return &domain.KTAVerificationResponse{NIA: code, Valid: false}, nil
	}
	keys := s.ktaVerifyKeys()
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

// verifySubmittedDocuments memverifikasi setiap dokumen yang diklaim sudah
// di-upload. Bila storage tidak dikonfigurasi: tolak di production
// (fail-closed), lewati dengan warning eksplisit di non-production agar dev
// tanpa MinIO tetap bisa berjalan (pola yang sama dengan Redis degraded).
func (s *pendaftaranService) verifySubmittedDocuments(ctx context.Context, req domain.PendaftaranSubmitRequest) error {
	docs := []struct {
		category string
		key      string
		required bool
	}{
		{"foto", req.FotoKey, true},
		{"ktp", req.KTPKey, true},
		{"cv", req.CVKey, false},
		{"sk", req.SKKey, false},
		{"surat_pernyataan", req.SuratPernyataanKey, false},
		{"surat_sehat", req.SuratSehatKey, false},
	}
	for _, d := range docs {
		k := strings.TrimSpace(d.key)
		if k == "" {
			continue // required sudah ditegakkan ValidateSubmitRequest
		}
		if err := s.verifyOneDocument(ctx, k, d.category); err != nil {
			return err
		}
	}
	return nil
}

// verifyOneDocument memverifikasi satu object via storage. Storage mati:
// 503 di production, lewati + warning di non-production.
func (s *pendaftaranService) verifyOneDocument(ctx context.Context, key, category string) error {
	if s.storageSvc == nil || !s.storageSvc.Configured() {
		if s.cfg != nil && s.cfg.App.Env == "production" {
			return domain.NewUnavailableError("Verifikasi dokumen tidak tersedia")
		}
		log.Warn().Str("category", category).
			Msg("Storage tidak dikonfigurasi — verifikasi dokumen dilewati (HANYA non-production)")
		return nil
	}
	return s.storageSvc.VerifySubmittedObject(ctx, key, category)
}

// validateWilayah memastikan provinsi/kabupaten ada, aktif, dan berelasi
// benar di master. Tanpa wilayahRepo (hanya test): tolak di production.
func (s *pendaftaranService) validateWilayah(ctx context.Context, provinsiID, kabupatenID int) error {
	if s.wilayahRepo == nil {
		if s.cfg != nil && s.cfg.App.Env == "production" {
			return domain.NewUnavailableError("Validasi wilayah tidak tersedia")
		}
		return nil
	}
	ok, err := s.wilayahRepo.ExistsProvinsi(ctx, provinsiID)
	if err != nil {
		return err
	}
	if !ok {
		return domain.NewValidationError("Provinsi tidak valid atau tidak aktif")
	}
	ok, err = s.wilayahRepo.KabupatenInProvinsi(ctx, kabupatenID, provinsiID)
	if err != nil {
		return err
	}
	if !ok {
		return domain.NewValidationError("Kabupaten tidak valid untuk provinsi tersebut")
	}
	return nil
}

// ktaVerifyKeys mengembalikan kunci verifikasi KTA: aktif dulu, lalu kunci
// rotasi sebelumnya (verify-only). Urutan penting untuk short-circuit.
func (s *pendaftaranService) ktaVerifyKeys() []string {
	if s.cfg == nil {
		return nil
	}
	keys := make([]string, 0, 2)
	if k := strings.TrimSpace(s.cfg.Crypto.KTASigningKey); k != "" {
		keys = append(keys, k)
	}
	if k := strings.TrimSpace(s.cfg.Crypto.KTASigningKeyPrev); k != "" {
		keys = append(keys, k)
	}
	return keys
}

// ListQueue mengembalikan antrean sesuai jurisdiction aktor + filter
// status allowlist + pagination bounded. Scope nasional = tanpa filter.
func (s *pendaftaranService) ListQueue(ctx context.Context, actor domain.ActorContext, status string, page, limit int) ([]domain.PendaftaranQueueItem, int, error) {
	if s.repo == nil {
		return nil, 0, domain.NewValidationError("Repository pendaftaran belum tersedia")
	}
	st := strings.TrimSpace(status)
	if st != "" {
		allowed := map[string]bool{
			string(domain.PendaftaranStatusDiajukan):     true,
			string(domain.PendaftaranStatusDiverifikasi): true,
			string(domain.PendaftaranStatusPerbaikan):    true,
			string(domain.PendaftaranStatusDisetujui):    true,
			string(domain.PendaftaranStatusDitolak):      true,
		}
		if !allowed[st] {
			return nil, 0, domain.NewValidationError("Filter status tidak valid")
		}
	}
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}
	offset := (page - 1) * limit

	var provID, kabID *int
	switch actor.Role {
	case domain.RoleSuperAdmin, domain.RoleAdminNasional:
		// tanpa filter
	case domain.RoleAdminProvinsi:
		if actor.ProvinsiID == nil {
			return nil, 0, domain.NewForbiddenError("Akun Admin Provinsi belum terhubung ke wilayah")
		}
		provID = actor.ProvinsiID
	case domain.RoleAdminKabupaten:
		if actor.KabupatenID == nil {
			return nil, 0, domain.NewForbiddenError("Akun Admin Kabupaten belum terhubung ke wilayah")
		}
		provID, kabID = actor.ProvinsiID, actor.KabupatenID
	default:
		return nil, 0, domain.NewForbiddenError("Role tidak diizinkan mengakses antrean")
	}

	items, err := s.repo.ListQueue(ctx, provID, kabID, st, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.repo.CountQueue(ctx, provID, kabID, st)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
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
func (s *pendaftaranService) RequestRevisionToken(ctx context.Context, req domain.RevisionTokenRequest, audit domain.AuditContext) (*domain.RevisionTokenResponse, error) {
	nr := strings.TrimSpace(req.Nomor)
	if nr == "" || len(nr) > 30 {
		return nil, domain.NewValidationError("Nomor pendaftaran tidak valid")
	}
	if s.repo == nil {
		return nil, domain.NewValidationError("Repository pendaftaran belum tersedia")
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
func (s *pendaftaranService) SubmitRevision(ctx context.Context, nomor string, req domain.RevisionSubmitRequest, audit domain.AuditContext) error {
	nr := strings.TrimSpace(nomor)
	if nr == "" || len(nr) > 30 {
		return domain.NewValidationError("Nomor pendaftaran tidak valid")
	}
	token := strings.TrimSpace(req.Token)
	if token == "" || len(token) > 256 {
		return domain.NewValidationError("Token revisi wajib diisi")
	}
	if s.repo == nil {
		return domain.NewUnavailableError("Layanan pendaftaran sedang tidak tersedia")
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
			if err := s.verifyOneDocument(ctx, k, in.category); err != nil {
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

// auditEvent mencatat jejak audit best-effort (RULES 21): gagal tulis tidak
// menggagalkan operasi utama. PII tidak pernah masuk metadata.
func (s *pendaftaranService) auditEvent(
	ctx context.Context,
	audit domain.AuditContext,
	actorID *string,
	actorName, actorRole, entity, entityID, action string,
	metadata *string,
) {
	if s.auditRepo == nil {
		return
	}
	e := &domain.ActivityLog{
		ActorID:    actorID,
		ActorName:  actorName,
		ActorRole:  actorRole,
		IPAddress:  audit.IP,
		UserAgent:  audit.UserAgent,
		EntityName: entity,
		EntityID:   entityID,
		Action:     action,
		Metadata:   metadata,
		RequestID:  audit.RequestID,
	}
	if err := s.auditRepo.Create(ctx, e); err != nil {
		log.Warn().
			Err(err).
			Str("action", action).
			Str("entity_id", entityID).
			Msg("Gagal mencatat audit trail pendaftaran")
	}
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

func (s *pendaftaranService) getAESKey() (string, error) {
	if s.cfg == nil || strings.TrimSpace(s.cfg.Crypto.AESMasterKey) == "" {
		return "", domain.NewValidationError("AES_MASTER_KEY belum dikonfigurasi")
	}
	return s.cfg.Crypto.AESMasterKey, nil
}

func (s *pendaftaranService) getBlindIndexKey() (string, error) {
	if s.cfg == nil || strings.TrimSpace(s.cfg.Crypto.BlindIndexKey) == "" {
		return "", domain.NewValidationError("BLIND_INDEX_KEY belum dikonfigurasi")
	}
	return s.cfg.Crypto.BlindIndexKey, nil
}
