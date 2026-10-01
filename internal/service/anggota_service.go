package service

// AnggotaService melayani daftar kader resmi untuk admin (terfilter
// jurisdiction, proyeksi non-PII) dan cek publik minimal pengganti
// cek-anggota lama (tanpa NIK, alamat, kontak, maupun object key).

import (
	"context"
	"strings"

	"github.com/kipan-indonesia/sim-kipan-core/config"
	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
)

type AnggotaService interface {
	// ListAnggota mengembalikan daftar sesuai jurisdiction aktor.
	ListAnggota(ctx context.Context, actor domain.ActorContext, status, search string, page, limit int) ([]domain.AnggotaListItem, int, error)
	// GetAnggotaDetail melayani admin: tolak objek di luar wilayah aktor.
	GetAnggotaDetail(ctx context.Context, id int, actor domain.ActorContext) (*domain.Anggota, error)
	// GetPublicAnggota adalah jalur publik by NIA (tanpa pencarian NIK:
	// NIK mentah dari publik adalah oracle PII — ditolak by design).
	GetPublicAnggota(ctx context.Context, nia string) (*domain.AnggotaPublicInfo, error)
}

type AnggotaDeps struct {
	AnggotaRepo repository.AnggotaRepository
	WilayahRepo repository.WilayahRepository
}

type anggotaService struct {
	cfg         *config.Config
	anggotaRepo repository.AnggotaRepository
	wilayahRepo repository.WilayahRepository
}

func NewAnggotaService(cfg *config.Config, deps AnggotaDeps) AnggotaService {
	return &anggotaService{
		cfg:         cfg,
		anggotaRepo: deps.AnggotaRepo,
		wilayahRepo: deps.WilayahRepo,
	}
}

// normalizeNIA menyeragamkan NIA (trim + uppercase) dan menolak format
// asing (422). Menerima kode provinsi huruf (warisan JB) maupun BPS
// numerik (baru 32) karena sequence migrasi data mencakup keduanya.
// Pencarian NIK publik sengaja tidak didukung (oracle PII): gunakan NIA.
func normalizeNIA(nia string) (string, error) {
	code := strings.ToUpper(strings.TrimSpace(nia))
	if code == "" || len(code) > 50 || !strings.HasPrefix(code, "KIPAN-") {
		return "", domain.NewValidationError("NIA tidak valid (gunakan Nomor Induk Anggota, cth KIPAN-32-3273-2026-00001)")
	}
	return code, nil
}

func (s *anggotaService) scopeOf(actor domain.ActorContext) (provID, kabID *int, err error) {
	switch actor.Role {
	case domain.RoleSuperAdmin, domain.RoleAdminNasional:
		return nil, nil, nil
	case domain.RoleAdminProvinsi:
		if actor.ProvinsiID == nil {
			return nil, nil, domain.NewForbiddenError("Akun Admin Provinsi belum terhubung ke wilayah")
		}
		return actor.ProvinsiID, nil, nil
	case domain.RoleAdminKabupaten:
		if actor.KabupatenID == nil {
			return nil, nil, domain.NewForbiddenError("Akun Admin Kabupaten belum terhubung ke wilayah")
		}
		return actor.ProvinsiID, actor.KabupatenID, nil
	default:
		return nil, nil, domain.NewForbiddenError("Role tidak diizinkan mengakses data anggota")
	}
}

func (s *anggotaService) ListAnggota(ctx context.Context, actor domain.ActorContext, status, search string, page, limit int) ([]domain.AnggotaListItem, int, error) {
	if s.anggotaRepo == nil {
		return nil, 0, unavailable("anggota")
	}
	st := strings.TrimSpace(status)
	if st != "" {
		allowed := map[string]bool{
			string(domain.AnggotaStatusAktif):         true,
			string(domain.AnggotaStatusNonaktif):      true,
			string(domain.AnggotaStatusDemisioner):    true,
			string(domain.AnggotaStatusDiberhentikan): true,
			string(domain.AnggotaStatusMeninggal):     true,
		}
		if !allowed[st] {
			return nil, 0, domain.NewValidationError("Filter status tidak valid")
		}
	}
	if len([]rune(strings.TrimSpace(search))) > 100 {
		return nil, 0, domain.NewValidationError("Kata kunci pencarian maksimal 100 karakter")
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

	provID, kabID, err := s.scopeOf(actor)
	if err != nil {
		return nil, 0, err
	}
	items, err := s.anggotaRepo.ListAnggota(ctx, provID, kabID, st, strings.TrimSpace(search), limit, offset)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.anggotaRepo.CountAnggota(ctx, provID, kabID, st, strings.TrimSpace(search))
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *anggotaService) GetAnggotaDetail(ctx context.Context, id int, actor domain.ActorContext) (*domain.Anggota, error) {
	if id <= 0 {
		return nil, domain.NewValidationError("ID anggota tidak valid")
	}
	if s.anggotaRepo == nil {
		return nil, unavailable("anggota")
	}
	item, err := s.anggotaRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !actor.CanAccessWilayah(item.ProvinsiID, item.KabupatenID) {
		return nil, domain.NewForbiddenError("Data anggota di luar wilayah kerja Anda")
	}
	return item, nil
}

func (s *anggotaService) GetPublicAnggota(ctx context.Context, nia string) (*domain.AnggotaPublicInfo, error) {
	code, err := normalizeNIA(nia)
	if err != nil {
		return nil, err
	}
	if s.anggotaRepo == nil {
		return nil, unavailable("anggota")
	}
	item, err := s.anggotaRepo.GetByNIA(ctx, code)
	if err != nil {
		return nil, err
	}
	var provNama, kabNama string
	if s.wilayahRepo != nil {
		if prov, kab, err := s.wilayahRepo.GetNames(ctx, item.ProvinsiID, item.KabupatenID); err == nil {
			provNama, kabNama = prov, kab
		}
	}
	return &domain.AnggotaPublicInfo{
		NIA:           item.NIA,
		NamaLengkap:   item.NamaLengkap,
		Status:        string(item.Status),
		ProvinsiNama:  provNama,
		KabupatenNama: kabNama,
	}, nil
}
