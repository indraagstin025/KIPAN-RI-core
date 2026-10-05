package notify

import (
	"context"
	"testing"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
)

func TestNotificationNilRepoUnavailable(t *testing.T) {
	svc := NewNotificationService(nil, nil)
	actor := domain.ActorContext{UserID: "u1", Role: domain.RoleAdminKabupaten}
	if _, err := svc.ListMine(context.Background(), actor, 10); err == nil {
		t.Fatal("harap 503 tanpa repo")
	}
	if err := svc.MarkRead(context.Background(), 1, actor); err == nil {
		t.Fatal("harap 503 tanpa repo")
	}
}

func TestNotificationInvalidInput(t *testing.T) {
	svc := NewNotificationService(nil, &stubNotifRepo{})
	if err := svc.MarkRead(context.Background(), 0, domain.ActorContext{UserID: "u1"}); err == nil {
		t.Fatal("ID 0 harus ditolak")
	}
	if _, err := svc.ListMine(context.Background(), domain.ActorContext{}, 10); err == nil {
		t.Fatal("aktor tanpa UserID harus ditolak")
	}
}

type stubNotifRepo struct{}

func (s *stubNotifRepo) NotifyAdmins(_ context.Context, _ string, _ string, _ domain.NotificationType, _ string, _ int, _ int) error {
	return nil
}

func (s *stubNotifRepo) ListMine(_ context.Context, _ string, _ int) ([]domain.Notification, error) {
	return []domain.Notification{}, nil
}

func (s *stubNotifRepo) MarkRead(_ context.Context, _ int64, _ string) error {
	return nil
}

var _ repository.NotificationRepository = (*stubNotifRepo)(nil)
