package service

import (
	"context"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
)

// DashboardService menyajikan agregat dashboard ter-scope wilayah.
type DashboardService interface {
	Load(ctx context.Context, actor domain.ActorContext) (*domain.DashboardData, error)
}

type dashboardSvc struct {
	repo repository.DashboardRepository
}

func NewDashboardService(repo repository.DashboardRepository) DashboardService {
	return &dashboardSvc{repo: repo}
}

func (s *dashboardSvc) Load(ctx context.Context, actor domain.ActorContext) (*domain.DashboardData, error) {
	if s.repo == nil {
		return nil, unavailable("dashboard")
	}
	prov, kab, err := actor.Scope()
	if err != nil {
		return nil, err
	}
	return s.repo.Load(ctx, prov, kab)
}
