package service

import (
	"context"
	"errors"
	"net/mail"
	"regexp"
	"strconv"
	"strings"
	"time"

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
	// ListQueue adalah antrean admin terfilter jurisdiction aktor.
	ListQueue(ctx context.Context, actor domain.ActorContext, status string, page, limit int) ([]domain.PendaftaranQueueItem, int, error)
	// GetDetail adalah jalur admin: tolak objek di luar wilayah aktor.
	GetDetail(ctx context.Context, id int, actor domain.ActorContext) (*domain.Pendaftaran, error)
	// Verifikasi, revisi, dan KTA/NIK pindah ke RevisionService &
	// VerificationService (R3: pecah god-service).
}

// PendaftaranDeps adalah dependensi service pendaftaran inti (R1+R3).
// Field nil-able; service memeriksa nil dan gagal fail-closed per fitur.
type PendaftaranDeps struct {
	Repo        repository.PendaftaranRepository
	AnggotaRepo repository.AnggotaRepository
	AuditRepo   repository.AuditLogRepository
	StorageSvc  ObjectVerifier
	WilayahRepo repository.WilayahRepository
}

type pendaftaranService struct {
	cfg         *config.Config
	repo        repository.PendaftaranRepository
	anggotaRepo repository.AnggotaRepository
	auditRepo   repository.AuditLogRepository
	storageSvc  ObjectVerifier
	wilayahRepo repository.WilayahRepository
}

func NewPendaftaranService(cfg *config.Config, deps PendaftaranDeps) PendaftaranService {
	return &pendaftaranService{
		cfg:         cfg,
		repo:        deps.Repo,
		anggotaRepo: deps.AnggotaRepo,
		auditRepo:   deps.AuditRepo,
		storageSvc:  deps.StorageSvc,
		wilayahRepo: deps.WilayahRepo,
	}
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
	key, err := blindIndexKey(s.cfg)
	if err != nil {
		return "", err
	}
	return crypto.BlindIndex(strings.TrimSpace(nik), key)
}

// EncryptNIK mengenkripsi NIK ternormalisasi agar tidak pernah disimpan plaintext.
func (s *pendaftaranService) EncryptNIK(nik string) (string, error) {
	key, err := aesKey(s.cfg)
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
		if err := verifyOneDocument(ctx, s.cfg, s.storageSvc, k, d.category); err != nil {
			return err
		}
	}
	return nil
}

// verifyOneDocument memverifikasi satu object via storage (R3: fungsi paket
// agar dipakai inti + revisi). Storage mati: 503 di production, lewati +
// warning di non-production.
func verifyOneDocument(ctx context.Context, cfg *config.Config, storageSvc ObjectVerifier, key, category string) error {
	if storageSvc == nil || !storageSvc.Configured() {
		if degradedSkip(cfg, "storage(verifikasi-dokumen:"+category+")") {
			return nil
		}
		return unavailable("verifikasi dokumen")
	}
	return storageSvc.VerifySubmittedObject(ctx, key, category)
}

// validateWilayah memastikan provinsi/kabupaten ada, aktif, dan berelasi
// benar di master (R2: degradedSkip bila repo tak di-wire).
func (s *pendaftaranService) validateWilayah(ctx context.Context, provinsiID, kabupatenID int) error {
	if s.wilayahRepo == nil {
		if degradedSkip(s.cfg, "wilayahRepo") {
			return nil
		}
		return unavailable("validasi wilayah")
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

// ListQueue mengembalikan antrean sesuai jurisdiction aktor + filter
// status allowlist + pagination bounded. Scope nasional = tanpa filter.
func (s *pendaftaranService) ListQueue(ctx context.Context, actor domain.ActorContext, status string, page, limit int) ([]domain.PendaftaranQueueItem, int, error) {
	if s.repo == nil {
		return nil, 0, unavailable("pendaftaran")
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

// auditEvent mendelegasikan ke writeAudit terpusat (R2).
func (s *pendaftaranService) auditEvent(
	ctx context.Context,
	audit domain.AuditContext,
	actorID *string,
	actorName, actorRole, entity, entityID, action string,
	metadata *string,
) {
	writeAudit(ctx, s.auditRepo, audit, actorID, actorName, actorRole, entity, entityID, action, metadata)
}

// aesKey/blindIndexKey membaca kunci kripto dari config (R3: fungsi paket
// agar dipakai inti + verifikasi tanpa duplikasi method).
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
