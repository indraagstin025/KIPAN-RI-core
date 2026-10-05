package service

// NotificationService melayani baca notifikasi milik sendiri (pengganti
// aman endpoint notifikasi lama yang membaca via ?userId= — IDOR).

import (
	"context"

	"github.com/kipan-indonesia/sim-kipan-core/config"
	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
	"github.com/kipan-indonesia/sim-kipan-core/internal/service/svcutil"
)

type NotificationService interface {
	// ListMine mengambil notifikasi milik aktor (klaim JWT), terbaru dulu.
	ListMine(ctx context.Context, actor domain.ActorContext, limit int) ([]domain.Notification, error)
	// MarkRead menandai satu notifikasi milik aktor sebagai dibaca.
	MarkRead(ctx context.Context, id int64, actor domain.ActorContext) error
}

type notificationService struct {
	cfg  *config.Config
	repo repository.NotificationRepository
}

func NewNotificationService(cfg *config.Config, repo repository.NotificationRepository) NotificationService {
	return &notificationService{cfg: cfg, repo: repo}
}

func (s *notificationService) ListMine(ctx context.Context, actor domain.ActorContext, limit int) ([]domain.Notification, error) {
	if s.repo == nil {
		return nil, svcutil.Unavailable("notifikasi")
	}
	if actor.UserID == "" {
		return nil, domain.NewForbiddenError("Identitas pengguna tidak valid")
	}
	return s.repo.ListMine(ctx, actor.UserID, limit)
}

func (s *notificationService) MarkRead(ctx context.Context, id int64, actor domain.ActorContext) error {
	if id <= 0 {
		return domain.NewValidationError("ID notifikasi tidak valid")
	}
	if s.repo == nil {
		return svcutil.Unavailable("notifikasi")
	}
	if actor.UserID == "" {
		return domain.NewForbiddenError("Identitas pengguna tidak valid")
	}
	return s.repo.MarkRead(ctx, id, actor.UserID)
}
