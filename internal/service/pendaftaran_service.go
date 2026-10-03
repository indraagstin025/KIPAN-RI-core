package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/mail"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/kipan-indonesia/sim-kipan-core/config"
	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/gateway"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/crypto"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/generator"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/keyset"
	"github.com/rs/zerolog/log"
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
	// ListQueueCursor varian keyset (tanpa COUNT) untuk antrean besar.
	ListQueueCursor(ctx context.Context, actor domain.ActorContext, status, cursor string, limit int) ([]domain.PendaftaranQueueItem, string, error)
	// GetDetail adalah jalur admin: tolak objek di luar wilayah aktor.
	GetDetail(ctx context.Context, id int, actor domain.ActorContext) (*domain.PendaftaranAdminDetail, error)
	// Verifikasi, revisi, dan KTA/NIK pindah ke RevisionService &
	// VerificationService (R3: pecah god-service).
}

// PendaftaranDeps adalah dependensi service pendaftaran inti (R1+R3).
// Field nil-able; service memeriksa nil dan gagal fail-closed per fitur.
type PendaftaranDeps struct {
	Repo        repository.PendaftaranCoreRepository
	AnggotaRepo repository.AnggotaRepository
	AuditRepo   repository.AuditLogRepository
	StorageSvc  ObjectVerifier
	WilayahRepo repository.WilayahRepository
	NotifRepo   repository.NotificationRepository
	OTPSvc      OTPService
	WAGateway   gateway.WAGateway
	ListRepo    repository.ListKeysetRepository
}

type pendaftaranService struct {
	cfg         *config.Config
	repo        repository.PendaftaranCoreRepository
	anggotaRepo repository.AnggotaRepository
	auditRepo   repository.AuditLogRepository
	storageSvc  ObjectVerifier
	wilayahRepo repository.WilayahRepository
	notifRepo   repository.NotificationRepository
	otpSvc      OTPService
	waGateway   gateway.WAGateway
	listRepo    repository.ListKeysetRepository
}

func NewPendaftaranService(cfg *config.Config, deps PendaftaranDeps) PendaftaranService {
	return &pendaftaranService{
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

// kodePosPattern mewajibkan 5 digit (format Indonesia).
var kodePosPattern = regexp.MustCompile(`^\d{5}$`)

// nomorPattern menerima REG-YYYYMM-XXXX (warisan KIPAN_INDONESIA, 4 digit)
// dan REG-YYYYMM-XXXXX (baru, 5 digit) serta lebih dari 5 digit bila suatu
// periode melampaui 99.999 pendaftaran (generator memakai lebar minimum 5).
var nomorPattern = regexp.MustCompile(`^REG-\d{6}-\d{4,}$`)

// normalizeNomor menyeragamkan nomor registrasi (trim + uppercase) dan
// menolak format di luar dua varian yang dikenal (422, bukan 404 agar
// pesan selaras UI lama "Format nomor pendaftaran tidak valid").
func normalizeNomor(nomor string) (string, error) {
	nr := strings.ToUpper(strings.TrimSpace(nomor))
	if nr == "" || len(nr) > 30 || !nomorPattern.MatchString(nr) {
		return "", domain.NewValidationError("Nomor pendaftaran tidak valid")
	}
	return nr, nil
}

// Batas panjang field selaras kolom database (anti-DoS + 422, bukan 500).
const (
	maxNamaLen          = 150
	maxTempatLahirLen   = 100
	maxAlamatLen        = 2000
	maxEmailLen         = 255
	maxWhatsappLen      = 25
	maxKecamatanDesaLen = 100
	maxMotivasiLen      = 1000
	minMotivasiLen      = 20
	maxBebasLen         = 100
	minPendaftarAge     = 16
	maxPendaftarAge     = 30
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
	// Pendaftaran HANYA untuk Kader. Jalur Pengurus via submit DITOLAK
	// (pengurus lahir dari pengangkatan via SK, bukan pendaftaran mandiri).
	// TipePendaftaran tetap diterima untuk kompatibilitas, tetapi selain
	// KADER ditolak eksplisit agar klien lama mendapat pesan jelas (bukan
	// perilaku diam-diam).
	if tipe, err := normalizeTipePendaftaran(req.TipePendaftaran); err != nil {
		return err
	} else if tipe != domain.TipePendaftaranKader {
		return domain.NewValidationError("Pendaftaran hanya untuk Kader. Pengurus diangkat via Surat Keputusan oleh Admin Kabupaten/Kota.")
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
	// Batch 3: token bukti OTP wajib ada (validitas + sekali pakai
	// ditegakkan di CreateRegistration via OTPService).
	if t := strings.TrimSpace(req.WaOTPToken); t == "" || len(t) > 256 {
		return domain.NewValidationError("Verifikasi WhatsApp wajib diselesaikan sebelum submit")
	}
	if err := checkObjectKey("Foto", req.FotoKey, true); err != nil {
		return err
	}
	if err := checkObjectKey("KTP", req.KTPKey, true); err != nil {
		return err
	}
	for _, f := range []struct {
		label    string
		key      string
		required bool
	}{
		{"CV", req.CVKey, true},
		{"SK", req.SKKey, false},
		{"Surat pernyataan", req.SuratPernyataanKey, true},
		{"Surat sehat", req.SuratSehatKey, true},
	} {
		if err := checkObjectKey(f.label, f.key, f.required); err != nil {
			return err
		}
	}
	motivasi := strings.TrimSpace(req.Motivasi)
	if len([]rune(motivasi)) < minMotivasiLen || len([]rune(motivasi)) > maxMotivasiLen {
		return domain.NewValidationError("Motivasi wajib 20-1000 karakter")
	}
	if containsAngleBracket(motivasi) {
		return domain.NewValidationError("Motivasi tidak boleh mengandung karakter < atau >")
	}
	if err := checkPersyaratan(req.Persyaratan); err != nil {
		return err
	}
	for _, f := range []struct {
		label    string
		val      string
		limit    int
		required bool
	}{
		{"Agama", req.Agama, maxBebasLen, true},
		{"Pendidikan", req.Pendidikan, maxBebasLen, true},
		{"Pekerjaan", req.Pekerjaan, maxBebasLen, true},
		{"Status pribadi", req.StatusPribadi, maxBebasLen, false},
		{"Kecamatan", req.Kecamatan, maxKecamatanDesaLen, true},
		{"Desa", req.Desa, maxKecamatanDesaLen, true},
	} {
		v := strings.TrimSpace(f.val)
		if v == "" {
			if f.required {
				return domain.NewValidationError(f.label + " wajib diisi")
			}
			continue
		}
		if len([]rune(v)) > f.limit {
			return domain.NewValidationError(f.label + " melebihi batas karakter")
		}
		if containsAngleBracket(v) {
			return domain.NewValidationError(f.label + " tidak boleh mengandung karakter < atau >")
		}
	}
	if !kodePosPattern.MatchString(strings.TrimSpace(req.KodePos)) {
		return domain.NewValidationError("Kode pos wajib 5 digit angka")
	}
	// L7: satu object key tidak boleh dipakai di >1 slot dokumen
	// (anti satu-berkas-untuk-semua). Berlaku untuk submit.
	return checkDuplicateDocKeys(docKeysOf(req))
}

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
// Slot kosong (dokumen opsional tak diunggah) diabaikan.
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

// checkPersyaratan memvalidasi checklist persyaratan pendaftar (selaras
// form KIPAN_INDONESIA): opsional, maks 20 item, tiap item 1-100 karakter
// tanpa angle bracket (anti stored-XSS).
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

// marshalPersyaratan menyeragamkan checklist (trim) lalu mengenkode ke
// JSON array string untuk kolom persyaratan_checklist. Gagal encode yang
// praktis mustahil dipetakan ke 422 agar tidak menjadi 500.
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

// normalizeTipePendaftaran menyeragamkan pilihan jalur (case-insensitive)
// dan menolak nilai di luar KADER/PENGURUS. Dipakai validasi agar klien
// lama mendapat pesan jelas; submit baru wajib KADER (ditolak di Validate).
func normalizeTipePendaftaran(raw string) (domain.TipePendaftaran, error) {
	t := domain.TipePendaftaran(strings.ToUpper(strings.TrimSpace(raw)))
	if !t.IsValid() {
		return "", domain.NewValidationError("Tipe pendaftaran harus KADER atau PENGURUS")
	}
	return t, nil
}

// containsAngleBracket menolak < > pada field plain-text (anti stored-XSS);
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

// checkPendaftarAge menolak tanggal masa depan dan umur di luar 16-30 tahun
// (selaras KIPAN_INDONESIA: POST /api/pendaftaran menolak umur <16/>30).
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

// CreateRegistration menyimpan pendaftaran baru: validasi → bukti OTP WA
// → cek duplikat NIK di DUA tabel (pendaftaran + anggota) → enkripsi →
// alokasi nomor → insert atomik beserta riwayat SUBMIT.
// Duplikat dikembalikan sebagai 409.
func (s *pendaftaranService) CreateRegistration(ctx context.Context, req domain.PendaftaranSubmitRequest, audit domain.AuditContext) (*domain.PendaftaranCreateResult, error) {
	if err := s.ValidateSubmitRequest(req); err != nil {
		return nil, err
	}
	// Tipe selalu KADER (Validate sudah menolak non-KADER); SKKey masuk
	// diabaikan (pengurus lahir dari pengangkatan, bukan submit).
	tipe := domain.TipePendaftaranKader
	if s.repo == nil {
		return nil, domain.NewValidationError("Repository pendaftaran belum tersedia")
	}

	nik := strings.TrimSpace(req.NIK)
	blindIndex, err := s.GenerateBlindIndex(nik)
	if err != nil {
		return nil, err
	}
	// Hardening anti-oracle: NIK terdaftar sebagai pendaftar maupun sebagai
	// anggota menghasilkan pesan 409 yang SAMA agar endpoint publik tidak
	// membocorkan status keanggotaan seseorang.
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
			return nil, domain.NewConflictError("NIK sudah terdaftar")
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
	persyaratanJSON, err := marshalPersyaratan(req.Persyaratan)
	if err != nil {
		return nil, err
	}
	// Batch 3: token OTP dikonsumsi SETELAH seluruh validasi lolos (tepat
	// sebelum nomor dialokasikan) agar submit yang gagal validasi bisa
	// diperbaiki tanpa minta kode baru. Sekali pakai via script Lua atomik.
	if s.otpSvc == nil {
		return nil, unavailable("verifikasi OTP")
	}
	if err := s.otpSvc.VerifyAndConsume(ctx, req.Whatsapp, req.WaOTPToken); err != nil {
		return nil, err
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
			NomorPendaftaran:     number,
			NamaLengkap:          strings.TrimSpace(req.NamaLengkap),
			NIKHash:              blindIndex,
			NIKEncrypted:         nikEncrypted,
			TempatLahir:          strings.TrimSpace(req.TempatLahir),
			TanggalLahir:         dob,
			JenisKelamin:         strings.TrimSpace(req.JenisKelamin),
			Agama:                strings.TrimSpace(req.Agama),
			Pendidikan:           strings.TrimSpace(req.Pendidikan),
			Pekerjaan:            strings.TrimSpace(req.Pekerjaan),
			StatusPribadi:        strings.TrimSpace(req.StatusPribadi),
			Alamat:               strings.TrimSpace(req.Alamat),
			ProvinsiID:           req.ProvinsiID,
			KabupatenID:          req.KabupatenID,
			Kecamatan:            strings.TrimSpace(req.Kecamatan),
			Desa:                 strings.TrimSpace(req.Desa),
			KodePos:              strings.TrimSpace(req.KodePos),
			Email:                strings.TrimSpace(req.Email),
			Whatsapp:             strings.TrimSpace(req.Whatsapp),
			Motivasi:             strings.TrimSpace(req.Motivasi),
			PersyaratanChecklist: persyaratanJSON,
			FotoKey:              strings.TrimSpace(req.FotoKey),
			KTPKey:               strings.TrimSpace(req.KTPKey),
			CVKey:                strings.TrimSpace(req.CVKey),
			SKKey:                "",
			SuratPernyataanKey:   strings.TrimSpace(req.SuratPernyataanKey),
			SuratSehatKey:        strings.TrimSpace(req.SuratSehatKey),
			Status:               domain.PendaftaranStatusDraft,
			Tipe:                 tipe,
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
		// Notifikasi fan-out server-side (best-effort, tanpa gagalkan submit).
		s.notifyAdmins(ctx, "Pendaftaran Baru",
			entity.NamaLengkap+" mendaftar dan menunggu verifikasi ("+number+").",
			domain.NotifTypePendaftaran, "#admin?page=verifikasi",
			entity.ProvinsiID, entity.KabupatenID)
		// Batch 2: kirim nomor REG via WhatsApp (async best-effort).
		s.notifyRegistrantWA(entity.Whatsapp, entity.NamaLengkap, number)
		return &domain.PendaftaranCreateResult{
			ID:               entity.ID,
			NomorPendaftaran: number,
			Status:           string(domain.PendaftaranStatusDraft),
		}, nil
	}
	return nil, lastErr
}

// GetTracking melayani pelacakan publik MINIMAL: nomor + status + timestamp.
// Field kaya (nama, wilayah, catatan, timeline) SENGAJA tidak disertakan pada
// jalur publik karena nomor REG sekuensial dapat dienumerasi (SEC-TRACK-PII).
// Data kaya akan dilayani lewat jalur berpruf pemilik (menyusul).
func (s *pendaftaranService) GetTracking(ctx context.Context, nomor string) (*domain.PendaftaranTrackingResponse, error) {
	nr, err := normalizeNomor(nomor)
	if err != nil {
		return nil, err
	}
	if s.repo == nil {
		return nil, domain.NewValidationError("Repository pendaftaran belum tersedia")
	}
	item, err := s.repo.GetByNomorPendaftaran(ctx, nr)
	if err != nil {
		return nil, err
	}
	// Endpoint publik GET tidak boleh menulis. Kedaluwarsa dihitung read-only:
	// DRAFT > 30 hari ditampilkan sebagai KEDALUWARSA (persist dilakukan admin).
	status := item.Status
	if status == domain.PendaftaranStatusDraft && time.Since(item.CreatedAt) > 30*24*time.Hour {
		status = domain.PendaftaranStatusKedaluwarsa
	}
	return &domain.PendaftaranTrackingResponse{
		NomorPendaftaran: item.NomorPendaftaran,
		Status:           string(status),
		StatusLabel:      domain.TrackingStatusLabel(status),
		CreatedAt:        item.CreatedAt,
		UpdatedAt:        item.UpdatedAt,
	}, nil
}

// GetDetail melayani admin: tolak objek di luar wilayah kerja aktor (RULES 7).
// Nama wilayah di-resolve dari master yang sama dengan dropdown (fail-open:
// lookup gagal = nama kosong, detail tetap kembali).
func (s *pendaftaranService) GetDetail(ctx context.Context, id int, actor domain.ActorContext) (*domain.PendaftaranAdminDetail, error) {
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
	out := &domain.PendaftaranAdminDetail{Pendaftaran: *item}
	if s.wilayahRepo != nil {
		if prov, kab, err := s.wilayahRepo.GetNames(ctx, item.ProvinsiID, item.KabupatenID); err == nil {
			out.ProvinsiNama, out.KabupatenNama = prov, kab
		} else {
			log.Warn().Err(err).Int("pendaftaran_id", id).Msg("GetDetail: gagal resolve nama wilayah")
		}
	}
	return out, nil
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
	// Lazy expiry: tandai DRAFT yang sudah lewat 30 hari menjadi KEDALUWARSA.
	_ = s.repo.ExpireStaleDrafts(ctx, 30)
	st := strings.TrimSpace(status)
	if st != "" && !domain.PendaftaranStatus(st).IsValid() {
		return nil, 0, domain.NewValidationError("Filter status tidak valid")
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

	provID, kabID, err := actor.Scope()
	if err != nil {
		return nil, 0, err
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

// ListQueueCursor varian keyset (tanpa COUNT + tanpa OFFSET besar).
func (s *pendaftaranService) ListQueueCursor(ctx context.Context, actor domain.ActorContext, status, cursor string, limit int) ([]domain.PendaftaranQueueItem, string, error) {
	if s.listRepo == nil {
		return nil, "", unavailable("pendaftaran")
	}
	_ = s.repo.ExpireStaleDrafts(ctx, 30)
	st := strings.TrimSpace(status)
	if st != "" && !domain.PendaftaranStatus(st).IsValid() {
		return nil, "", domain.NewValidationError("Filter status tidak valid")
	}
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}
	provID, kabID, err := actor.Scope()
	if err != nil {
		return nil, "", err
	}
	at := time.Now().UTC().Add(time.Hour)
	id := maxInt4
	if strings.TrimSpace(cursor) != "" {
		var err error
		at, id, err = keyset.Decode(cursor)
		if err != nil {
			return nil, "", domain.NewValidationError("Cursor tidak valid")
		}
	}
	items, err := s.listRepo.ListQueueKeyset(ctx, provID, kabID, st, at, id, limit+1)
	if err != nil {
		return nil, "", err
	}
	next := ""
	if len(items) > limit {
		items = items[:limit]
		last := items[len(items)-1]
		next = keyset.Encode(last.CreatedAt, last.ID)
	}
	return items, next, nil
}

// gagal kirim hanya dicatat warn, tidak menggagalkan alur utama.
func (s *pendaftaranService) notifyAdmins(ctx context.Context, title, message string, notifType domain.NotificationType, link string, provID, kabID int) {
	if s.notifRepo == nil {
		return
	}
	if err := s.notifRepo.NotifyAdmins(ctx, title, message, notifType, link, provID, kabID); err != nil {
		log.Warn().Err(err).Msg("gagal menyebar notifikasi pendaftaran")
	}
}

// publicURL mengembalikan base URL frontend untuk tautan pesan.
func (s *pendaftaranService) publicURL() string {
	return publicURLFrom(s.cfg)
}

// registrantWAMessage menyusun teks WhatsApp berisi nomor pendaftaran +
// tautan lacak.
func registrantWAMessage(nama, nomor, publicURL string) string {
	return "Halo " + nama + ",\n\n" +
		"Pendaftaran KIPAN Anda telah kami terima.\n" +
		"Nomor Pendaftaran: " + nomor + "\n\n" +
		"Lacak status: " + publicURL + "/lacak?nomor=" + nomor + "\n" +
		"Simpan nomor ini untuk revisi berkas dan verifikasi KTA.\n\n" +
		"— Sistem Informasi KIPAN RI"
}

// notifyRegistrantWA mengirim nomor REG via WhatsApp secara async & best-effort:
// submit tetap sukses walau gateway gagal/lambat (nomor sudah tampil di layar
// sukses + tersimpan di browser). Memakai context terpisah karena context
// request sudah selesai saat goroutine berjalan.
func (s *pendaftaranService) notifyRegistrantWA(whatsapp, nama, nomor string) {
	if s.waGateway == nil || strings.TrimSpace(whatsapp) == "" {
		return
	}
	text := registrantWAMessage(nama, nomor, s.publicURL())
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()
		if err := s.waGateway.SendMessage(ctx, whatsapp, text); err != nil {
			log.Warn().Err(err).Str("nomor", nomor).
				Msg("Gagal mengirim nomor pendaftaran via WhatsApp")
		}
	}()
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
