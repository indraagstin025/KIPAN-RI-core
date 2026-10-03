package service

// pengurus_expiry.go — materialisasi kedaluwarsa masa bakti pengurus (TDD §5.4):
// menutup record pengurus Aktif yang SK-nya sudah lewat lalu menulis audit
// DEMISIONER_OTOMATIS per personalia. Dijalankan berkala oleh cmd/worker.

import (
	"context"
	"strconv"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
)

// PengurusExpiryService menutup pengurus kedaluwarsa (materialisasi).
type PengurusExpiryService interface {
	// RunOnce menutup seluruh pengurus kedaluwarsa; mengembalikan jumlahnya.
	RunOnce(ctx context.Context) (int, error)
}

type pengurusExpirySvc struct {
	pengurus repository.PengurusRepository
	audit    repository.AuditLogRepository
}

// NewPengurusExpiryService membangun service materialisasi kedaluwarsa.
func NewPengurusExpiryService(pengurus repository.PengurusRepository, audit repository.AuditLogRepository) PengurusExpiryService {
	return &pengurusExpirySvc{pengurus: pengurus, audit: audit}
}

// RunOnce menutup pengurus kedaluwarsa + audit per baris (aktor "Sistem").
func (s *pengurusExpirySvc) RunOnce(ctx context.Context) (int, error) {
	if s.pengurus == nil {
		return 0, nil
	}
	closed, err := s.pengurus.CloseExpiredAppointments(ctx)
	if err != nil {
		return 0, err
	}
	for i := range closed {
		it := closed[i]
		meta := `{"event":"demisioner_otomatis","nomor_sk":"` + it.NomorSK + `"}`
		if s.audit != nil {
			_ = s.audit.Create(ctx, &domain.ActivityLog{
				ActorName:  "Sistem",
				ActorRole:  "SYSTEM",
				EntityName: "pengurus",
				EntityID:   strconv.Itoa(it.ID),
				Action:     "DEMISIONER_OTOMATIS",
				Metadata:   &meta,
			})
		}
	}
	return len(closed), nil
}
