package service

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/kipan-indonesia/sim-kipan-core/config"
	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/crypto"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/generator"
)

// PendaftaranService berisi business logic pendaftaran calon anggota.
type PendaftaranService interface {
	ValidateSubmitRequest(req domain.PendaftaranSubmitRequest) error
	BuildRegistrationNumber(year int) (string, error)
	GenerateBlindIndex(nik string) (string, error)
	EncryptNIK(nik string) (string, error)
	CreateRegistration(ctx context.Context, req domain.PendaftaranSubmitRequest) (*domain.PendaftaranCreateResult, error)
	ProcessApproval(ctx context.Context, id int, action domain.PendaftaranApprovalAction, catatan string) error
}

type pendaftaranService struct {
	cfg  *config.Config
	repo repository.PendaftaranRepository
}

func NewPendaftaranService(cfg *config.Config, repo ...repository.PendaftaranRepository) PendaftaranService {
	var selected repository.PendaftaranRepository
	if len(repo) > 0 {
		selected = repo[0]
	}
	return &pendaftaranService{cfg: cfg, repo: selected}
}

var nipPattern = regexp.MustCompile(`^\d{16}$`)
var phonePattern = regexp.MustCompile(`^(08|\+62)\d{8,13}$`)

// ValidateSubmitRequest memastikan payload pendaftaran aman dan valid sebelum masuk DB.
func (s *pendaftaranService) ValidateSubmitRequest(req domain.PendaftaranSubmitRequest) error {
	if strings.TrimSpace(req.NamaLengkap) == "" {
		return domain.NewValidationError("Nama lengkap wajib diisi")
	}
	if strings.TrimSpace(req.NIK) == "" || !nipPattern.MatchString(req.NIK) {
		return domain.NewValidationError("NIK harus berisi 16 digit angka")
	}
	if strings.TrimSpace(req.TempatLahir) == "" {
		return domain.NewValidationError("Tempat lahir wajib diisi")
	}
	if _, err := time.Parse(time.RFC3339, req.TanggalLahir); err != nil {
		return domain.NewValidationError("Format tanggal lahir tidak valid")
	}
	if strings.TrimSpace(req.JenisKelamin) == "" || (req.JenisKelamin != "L" && req.JenisKelamin != "P") {
		return domain.NewValidationError("Jenis kelamin harus L atau P")
	}
	if strings.TrimSpace(req.Alamat) == "" {
		return domain.NewValidationError("Alamat wajib diisi")
	}
	if req.ProvinsiID <= 0 || req.KabupatenID <= 0 {
		return domain.NewValidationError("Wilayah provinsi dan kabupaten wajib dipilih")
	}
	if strings.TrimSpace(req.Email) == "" || !strings.Contains(req.Email, "@") {
		return domain.NewValidationError("Email wajib valid")
	}
	if strings.TrimSpace(req.Whatsapp) == "" || !phonePattern.MatchString(req.Whatsapp) {
		return domain.NewValidationError("Nomor WhatsApp tidak valid")
	}
	if strings.TrimSpace(req.FotoKey) == "" || strings.TrimSpace(req.KTPKey) == "" {
		return domain.NewValidationError("Foto dan KTP wajib diunggah")
	}
	return nil
}

// BuildRegistrationNumber menghasilkan nomor registrasi baru sesuai format business.
func (s *pendaftaranService) BuildRegistrationNumber(year int) (string, error) {
	if year <= 0 {
		return "", domain.NewValidationError("Tahun pendaftaran tidak valid")
	}
	seq, err := nextSequenceForYear(year)
	if err != nil {
		return "", err
	}
	return generator.GenerateRegistrationNumber(year, seq)
}

func nextSequenceForYear(year int) (int, error) {
	if year <= 0 {
		return 0, domain.NewValidationError("Tahun pendaftaran tidak valid")
	}
	return 1, nil
}

// GenerateBlindIndex mengubah NIK menjadi blind index HMAC-SHA256 untuk deteksi duplikasi aman.
func (s *pendaftaranService) GenerateBlindIndex(nik string) (string, error) {
	key, err := s.getBlindIndexKey()
	if err != nil {
		return "", err
	}
	return crypto.BlindIndex(nik, key)
}

// EncryptNIK mengenkripsi NIK agar tidak pernah disimpan dalam plaintext.
func (s *pendaftaranService) EncryptNIK(nik string) (string, error) {
	key, err := s.getAESKey()
	if err != nil {
		return "", err
	}
	return crypto.EncryptAESGCM(nik, key)
}

// CreateRegistration placeholder untuk layer repository yang akan dibuat di langkah berikutnya.
func (s *pendaftaranService) CreateRegistration(ctx context.Context, req domain.PendaftaranSubmitRequest) (*domain.PendaftaranCreateResult, error) {
	if err := s.ValidateSubmitRequest(req); err != nil {
		return nil, err
	}

	number, err := s.BuildRegistrationNumber(time.Now().Year())
	if err != nil {
		return nil, err
	}

	return &domain.PendaftaranCreateResult{
		ID:               1,
		NomorPendaftaran: number,
		Status:           string(domain.PendaftaranStatusDiajukan),
	}, nil
}

// ProcessApproval memvalidasi transisi status pendaftaran sesuai state machine dan menyimpan riwayat aksi.
func (s *pendaftaranService) ProcessApproval(ctx context.Context, id int, action domain.PendaftaranApprovalAction, catatan string) error {
	if id <= 0 {
		return domain.NewValidationError("ID pendaftaran tidak valid")
	}
	if s.repo == nil {
		return domain.NewValidationError("Repository pendaftaran belum tersedia")
	}
	if action == "" {
		return domain.NewValidationError("Aksi verifikasi wajib dipilih")
	}

	item, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	targetStatus, ok := mapStatusForAction(action)
	if !ok {
		return domain.NewValidationError("Aksi tidak valid untuk proses pendaftaran")
	}
	if !domain.IsAllowedTransition(item.Status, targetStatus, action) {
		return domain.NewValidationError("Transisi status tidak sah untuk aksi yang diminta")
	}
	if action == domain.PendaftaranActionSetujui {
		_, err := s.repo.IssueMember(ctx, id, time.Now().Year())
		return err
	}

	if err := s.repo.UpdateStatus(ctx, id, targetStatus, strings.TrimSpace(catatan)); err != nil {
		return err
	}

	if err := s.repo.AppendHistory(ctx, id, string(action), nil, nil, strings.TrimSpace(catatan)); err != nil {
		return err
	}

	return nil
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
