package pendaftaran

// pendaftaran_submit_service.go — service fokus: submit/pendaftaran baru
// (validasi, enkripsi NIK, alokasi nomor, simpan) (Fase A4, SRP).

import (
	"context"
	"errors"
	"net/mail"
	"strconv"
	"strings"
	"time"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/service/svcutil"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/crypto"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/generator"
)

// PendaftaranSubmitService menangani pembuatan pendaftaran baru & util kripto.
type PendaftaranSubmitService interface {
	ValidateSubmitRequest(req domain.PendaftaranSubmitRequest) error
	BuildRegistrationNumber(ctx context.Context, year, month int) (string, error)
	GenerateBlindIndex(nik string) (string, error)
	EncryptNIK(nik string) (string, error)
	CreateRegistration(ctx context.Context, req domain.PendaftaranSubmitRequest, audit domain.AuditContext) (*domain.PendaftaranCreateResult, error)
}

type pendaftaranSubmitSvc struct{ *pendaftaranBase }

// ValidateSubmitRequest memastikan payload pendaftaran aman dan valid sebelum masuk DB.
func (s *pendaftaranSubmitSvc) ValidateSubmitRequest(req domain.PendaftaranSubmitRequest) error {
	nama := strings.TrimSpace(req.NamaLengkap)
	if len([]rune(nama)) < 3 || len([]rune(nama)) > svcutil.MaxNamaLen {
		return domain.NewValidationError("Nama lengkap wajib 3-150 karakter")
	}
	if svcutil.ContainsAngleBracket(nama) {
		return domain.NewValidationError("Nama lengkap tidak boleh mengandung karakter < atau >")
	}
	nik := strings.TrimSpace(req.NIK)
	if !svcutil.NipPattern.MatchString(nik) {
		return domain.NewValidationError("NIK harus berisi 16 digit angka")
	}
	if !svcutil.IsPlausibleNIKDate(nik) {
		return domain.NewValidationError("Segmen tanggal lahir pada NIK tidak valid")
	}
	tempat := strings.TrimSpace(req.TempatLahir)
	if tempat == "" || len([]rune(tempat)) > svcutil.MaxTempatLahirLen {
		return domain.NewValidationError("Tempat lahir wajib diisi (maks 100 karakter)")
	}
	if svcutil.ContainsAngleBracket(tempat) {
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
	// Pendaftaran HANYA untuk Kader (Pengurus diangkat via SK).
	if tipe, err := normalizeTipePendaftaran(req.TipePendaftaran); err != nil {
		return err
	} else if tipe != domain.TipePendaftaranKader {
		return domain.NewValidationError("Pendaftaran hanya untuk Kader. Pengurus diangkat via Surat Keputusan oleh Admin Kabupaten/Kota.")
	}
	alamat := strings.TrimSpace(req.Alamat)
	if len([]rune(alamat)) < 10 || len([]rune(alamat)) > svcutil.MaxAlamatLen {
		return domain.NewValidationError("Alamat wajib 10-2000 karakter")
	}
	if svcutil.ContainsAngleBracket(alamat) {
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
	if len(wa) > maxWhatsappLen || !svcutil.PhonePattern.MatchString(wa) {
		return domain.NewValidationError("Nomor WhatsApp tidak valid (contoh: 081234567890)")
	}
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
	if svcutil.ContainsAngleBracket(motivasi) {
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
		if svcutil.ContainsAngleBracket(v) {
			return domain.NewValidationError(f.label + " tidak boleh mengandung karakter < atau >")
		}
	}
	if !svcutil.KodePosPattern.MatchString(strings.TrimSpace(req.KodePos)) {
		return domain.NewValidationError("Kode pos wajib 5 digit angka")
	}
	// L7: satu object key tidak boleh dipakai di >1 slot dokumen.
	return checkDuplicateDocKeys(docKeysOf(req))
}

// BuildRegistrationNumber mengalokasikan sequence periode dari database lalu
// memformat nomor REG-YYYYMM-XXXXX.
func (s *pendaftaranSubmitSvc) BuildRegistrationNumber(ctx context.Context, year, month int) (string, error) {
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

// GenerateBlindIndex mengubah NIK ternormalisasi menjadi blind index HMAC-SHA256.
func (s *pendaftaranSubmitSvc) GenerateBlindIndex(nik string) (string, error) {
	key, err := svcutil.BlindIndexKey(s.cfg)
	if err != nil {
		return "", err
	}
	return crypto.BlindIndex(strings.TrimSpace(nik), key)
}

// EncryptNIK mengenkripsi NIK ternormalisasi agar tidak pernah disimpan plaintext.
func (s *pendaftaranSubmitSvc) EncryptNIK(nik string) (string, error) {
	key, err := svcutil.AesKey(s.cfg)
	if err != nil {
		return "", err
	}
	return crypto.EncryptAESGCM(strings.TrimSpace(nik), key)
}

// CreateRegistration menyimpan pendaftaran baru: validasi → bukti OTP WA →
// cek duplikat NIK (pendaftaran + anggota) → enkripsi → alokasi nomor →
// insert atomik beserta riwayat SUBMIT. Duplikat = 409.
func (s *pendaftaranSubmitSvc) CreateRegistration(ctx context.Context, req domain.PendaftaranSubmitRequest, audit domain.AuditContext) (*domain.PendaftaranCreateResult, error) {
	if err := s.ValidateSubmitRequest(req); err != nil {
		return nil, err
	}
	// Tipe selalu KADER (Validate sudah menolak non-KADER).
	tipe := domain.TipePendaftaranKader
	if s.repo == nil {
		return nil, domain.NewValidationError("Repository pendaftaran belum tersedia")
	}

	nik := strings.TrimSpace(req.NIK)
	blindIndex, err := s.GenerateBlindIndex(nik)
	if err != nil {
		return nil, err
	}
	// Hardening anti-oracle: NIK terdaftar (pendaftar/anggota) → pesan 409 sama.
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

	// Validasi master wilayah server-side (RULES 6).
	if err := s.validateWilayah(ctx, req.ProvinsiID, req.KabupatenID); err != nil {
		return nil, err
	}

	// Verifikasi keberadaan + integritas dokumen SEBELUM nomor dialokasikan (RULES 14).
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
	// Token OTP dikonsumsi SETELAH seluruh validasi lolos (tepat sebelum
	// nomor dialokasikan). Sekali pakai via script Lua atomik.
	if s.otpSvc == nil {
		return nil, svcutil.Unavailable("verifikasi OTP")
	}
	if err := s.otpSvc.VerifyAndConsume(ctx, req.Whatsapp, req.WaOTPToken); err != nil {
		return nil, err
	}
	now := time.Now()

	// Retry sekali untuk perebutan nomor (UNIQUE backstop).
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
		// Notifikasi fan-out server-side (best-effort).
		s.notifyAdmins(ctx, "Pendaftaran Baru",
			entity.NamaLengkap+" mendaftar dan menunggu verifikasi ("+number+").",
			domain.NotifTypePendaftaran, "#admin?page=verifikasi",
			entity.ProvinsiID, entity.KabupatenID)
		// Kirim nomor REG via WhatsApp (async best-effort).
		s.notifyRegistrantWA(entity.Whatsapp, entity.NamaLengkap, number)
		return &domain.PendaftaranCreateResult{
			ID:               entity.ID,
			NomorPendaftaran: number,
			Status:           string(domain.PendaftaranStatusDraft),
		}, nil
	}
	return nil, lastErr
}
