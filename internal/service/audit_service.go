package service

// audit_service.go — penelusur jejak audit (TDD §7.2): daftar terfilter +
// ekspor CSV. Hanya untuk Admin Nasional/Super (ditegakkan di route).

import (
	"bytes"
	"context"
	"encoding/csv"
	"strings"
	"time"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
	"github.com/kipan-indonesia/sim-kipan-core/internal/service/svcutil"
)

// AuditService menelusuri & mengekspor jejak audit.
type AuditService interface {
	List(ctx context.Context, f domain.AuditFilter) ([]domain.ActivityLog, int, error)
	ExportCSV(ctx context.Context, f domain.AuditFilter) ([]byte, error)
}

type auditService struct {
	repo repository.AuditLogRepository
}

// NewAuditService membangun service audit.
func NewAuditService(repo repository.AuditLogRepository) AuditService {
	return &auditService{repo: repo}
}

// List mengembalikan jejak audit terfilter + total.
func (s *auditService) List(ctx context.Context, f domain.AuditFilter) ([]domain.ActivityLog, int, error) {
	if s.repo == nil {
		return nil, 0, svcutil.Unavailable("audit")
	}
	if f.Limit <= 0 {
		f.Limit = 25
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
	return s.repo.List(ctx, f)
}

// ExportCSV mengekspor audit terfilter (dibatasi 5000 baris).
func (s *auditService) ExportCSV(ctx context.Context, f domain.AuditFilter) ([]byte, error) {
	if s.repo == nil {
		return nil, svcutil.Unavailable("audit")
	}
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := w.Write([]string{"Waktu", "Aktor", "Peran", "Aksi", "Entitas", "EntityID", "IP", "RequestID", "Metadata"}); err != nil {
		return nil, err
	}
	const pageSize = 200
	const maxRows = 5000
	f.Limit = pageSize
	f.WithTotal = false
	offset := 0
	for {
		f.Offset = offset
		items, _, err := s.repo.List(ctx, f)
		if err != nil {
			return nil, err
		}
		if len(items) == 0 {
			break
		}
		for i := range items {
			it := items[i]
			meta := ""
			if it.Metadata != nil {
				meta = *it.Metadata
			}
			if err := w.Write([]string{
				it.CreatedAt.Format("2006-01-02 15:04:05"),
				it.ActorName, it.ActorRole, it.Action, it.EntityName, it.EntityID,
				it.IPAddress, it.RequestID, meta,
			}); err != nil {
				return nil, err
			}
		}
		offset += len(items)
		if len(items) < pageSize || offset >= maxRows {
			break
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ParseAuditFilter membaca filter audit dari query string (dipakai handler).
func ParseAuditFilter(action, entity, actor, dari, ke string, page, limit int, withTotal bool) domain.AuditFilter {
	f := domain.AuditFilter{
		Action:    strings.TrimSpace(action),
		Entity:    strings.TrimSpace(entity),
		Actor:     strings.TrimSpace(actor),
		Limit:     limit,
		WithTotal: withTotal,
	}
	if page < 1 {
		page = 1
	}
	if f.Limit <= 0 {
		f.Limit = 25
	}
	f.Offset = (page - 1) * f.Limit
	f.From = parseAuditTime(dari)
	f.To = parseAuditTime(ke)
	return f
}

// parseAuditTime menerima RFC3339 atau tanggal "2006-01-02".
func parseAuditTime(s string) *time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return &t
		}
	}
	return nil
}
