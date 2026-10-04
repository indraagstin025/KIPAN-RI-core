package service

import (
	"context"
	"strings"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
)

// EmailDispatcher mengirim satu item antrian segera (render + kirim + tandai
// sent/failed). Diimplementasikan oleh *service.EmailWorker.
type EmailDispatcher interface {
	ProcessOne(ctx context.Context, it domain.EmailOutbox) error
}

// OutboxService melayani pemantauan & pengiriman antrian email (ter-scope).
type OutboxService interface {
	List(ctx context.Context, actor domain.ActorContext, jenis, status string, page, limit int) ([]domain.EmailOutbox, int, error)
	Retry(ctx context.Context, actor domain.ActorContext, id int64) error
	RetryPending(ctx context.Context, actor domain.ActorContext) (int64, error)
	RetryMany(ctx context.Context, actor domain.ActorContext, ids []int64) (int64, error)
	// SendNow mengirim SATU item antrian sekarang (sinkron), tanpa menunggu worker.
	SendNow(ctx context.Context, actor domain.ActorContext, id int64) error
}

type outboxSvc struct {
	repo       repository.EmailOutboxRepository
	dispatcher EmailDispatcher
}

func NewOutboxService(repo repository.EmailOutboxRepository, dispatcher EmailDispatcher) OutboxService {
	return &outboxSvc{repo: repo, dispatcher: dispatcher}
}

// SendNow mengirim satu item antrian saat ini juga (dipakai tombol admin).
// Item yang gagal akan ditandai failed + last_error oleh dispatcher.
func (s *outboxSvc) SendNow(ctx context.Context, actor domain.ActorContext, id int64) error {
	if s.repo == nil {
		return unavailable("antrian email")
	}
	if s.dispatcher == nil {
		return unavailable("pengiriman email")
	}
	it, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if !actor.CanAccessWilayah(derefInt(it.ProvinsiID), derefInt(it.KabupatenID)) {
		return domain.NewForbiddenError("Antrian di luar wilayah kerja Anda")
	}
	if it.Status == domain.EmailOutboxSent {
		return domain.NewConflictError("Email sudah terkirim")
	}
	if err := s.dispatcher.ProcessOne(ctx, *it); err != nil {
		return domain.NewValidationError("Gagal mengirim email. Lihat detail error pada baris antrian.")
	}
	return nil
}

func (s *outboxSvc) List(ctx context.Context, actor domain.ActorContext, jenis, status string, page, limit int) ([]domain.EmailOutbox, int, error) {
	if s.repo == nil {
		return nil, 0, unavailable("antrian email")
	}
	prov, kab, err := actor.Scope()
	if err != nil {
		return nil, 0, err
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
	return s.repo.List(ctx, repository.OutboxFilter{
		Jenis: strings.TrimSpace(jenis), Status: strings.TrimSpace(status),
		ProvinsiID: prov, KabupatenID: kab, Limit: limit, Offset: (page - 1) * limit,
	})
}

func (s *outboxSvc) inScope(ctx context.Context, actor domain.ActorContext, id int64) error {
	it, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if !actor.CanAccessWilayah(derefInt(it.ProvinsiID), derefInt(it.KabupatenID)) {
		return domain.NewForbiddenError("Antrian di luar wilayah kerja Anda")
	}
	return nil
}

func (s *outboxSvc) Retry(ctx context.Context, actor domain.ActorContext, id int64) error {
	if s.repo == nil {
		return unavailable("antrian email")
	}
	if err := s.inScope(ctx, actor, id); err != nil {
		return err
	}
	return s.repo.RetryNow(ctx, id)
}

func (s *outboxSvc) RetryPending(ctx context.Context, actor domain.ActorContext) (int64, error) {
	if s.repo == nil {
		return 0, unavailable("antrian email")
	}
	prov, kab, err := actor.Scope()
	if err != nil {
		return 0, err
	}
	return s.repo.RetryPending(ctx, prov, kab)
}

func (s *outboxSvc) RetryMany(ctx context.Context, actor domain.ActorContext, ids []int64) (int64, error) {
	if s.repo == nil {
		return 0, unavailable("antrian email")
	}
	if len(ids) == 0 {
		return 0, nil
	}
	for _, id := range ids {
		if err := s.inScope(ctx, actor, id); err != nil {
			return 0, err
		}
	}
	return s.repo.RetryMany(ctx, ids)
}
