package service

import (
	"context"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
)

// WilayahService melayani daftar master wilayah untuk dropdown publik
// (form pendaftaran frontend) dan test lintas-wilayah. Read-only.
type WilayahService interface {
	ListProvinsi(ctx context.Context) ([]domain.WilayahProvinsi, error)
	ListKabupaten(ctx context.Context, provinsiID int) ([]domain.WilayahKabupaten, error)
}

type wilayahSvc struct {
	repo repository.WilayahRepository
}

func NewWilayahService(repo repository.WilayahRepository) WilayahService {
	return &wilayahSvc{repo: repo}
}

func (s *wilayahSvc) ListProvinsi(ctx context.Context) ([]domain.WilayahProvinsi, error) {
	if s.repo == nil {
		return nil, unavailable("wilayah")
	}
	return s.repo.ListProvinsi(ctx)
}

func (s *wilayahSvc) ListKabupaten(ctx context.Context, provinsiID int) ([]domain.WilayahKabupaten, error) {
	if provinsiID <= 0 {
		return nil, domain.NewValidationError("ID provinsi tidak valid")
	}
	if s.repo == nil {
		return nil, unavailable("wilayah")
	}
	ok, err := s.repo.ExistsProvinsi(ctx, provinsiID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain.NewNotFoundError("Provinsi")
	}
	return s.repo.ListKabupaten(ctx, provinsiID)
}
