package service

// pendaftaran_query_service.go — service fokus: pelacakan publik & antrean
// admin (Fase A4, SRP).

import (
	"context"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/service/svcutil"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/keyset"
)

// PendaftaranQueryService menangani pelacakan publik dan antrean/detail admin.
type PendaftaranQueryService interface {
	// GetTracking adalah jalur publik: kembalikan DTO minimal tanpa PII.
	GetTracking(ctx context.Context, nomor string) (*domain.PendaftaranTrackingResponse, error)
	// ListQueue adalah antrean admin terfilter jurisdiction aktor.
	ListQueue(ctx context.Context, actor domain.ActorContext, status string, page, limit int) ([]domain.PendaftaranQueueItem, int, error)
	// ListQueueCursor varian keyset (tanpa COUNT) untuk antrean besar.
	ListQueueCursor(ctx context.Context, actor domain.ActorContext, status, cursor string, limit int) ([]domain.PendaftaranQueueItem, string, error)
	// GetDetail adalah jalur admin: tolak objek di luar wilayah aktor.
	GetDetail(ctx context.Context, id int, actor domain.ActorContext) (*domain.PendaftaranAdminDetail, error)
}

type pendaftaranQuerySvc struct{ *pendaftaranBase }

// GetTracking melayani pelacakan publik MINIMAL: nomor + status + timestamp.
func (s *pendaftaranQuerySvc) GetTracking(ctx context.Context, nomor string) (*domain.PendaftaranTrackingResponse, error) {
	nr, err := svcutil.NormalizeNomor(nomor)
	if err != nil {
		return nil, err
	}
	if s.repo == nil {
		return nil, domain.NewValidationError("Repository pendaftaran belum tersedia")
	}
	item, err := s.repo.GetByNomorPendaftaran(ctx, nr)
	if err != nil {
		return nil, err
	}
	// Endpoint publik GET tidak boleh menulis. Kedaluwarsa dihitung read-only:
	// DRAFT > 30 hari ditampilkan sebagai KEDALUWARSA (persist dilakukan admin).
	status := item.Status
	if status == domain.PendaftaranStatusDraft && time.Since(item.CreatedAt) > 30*24*time.Hour {
		status = domain.PendaftaranStatusKedaluwarsa
	}
	resp := &domain.PendaftaranTrackingResponse{
		NomorPendaftaran: item.NomorPendaftaran,
		Status:           string(status),
		StatusLabel:      domain.TrackingStatusLabel(status),
		CreatedAt:        item.CreatedAt,
		UpdatedAt:        item.UpdatedAt,
	}
	// Sub-status pengiriman kredensial hanya relevan setelah DISETUJUI.
	if status == domain.PendaftaranStatusDisetujui {
		s.fillKredensialStatus(ctx, item, resp)
	}
	return resp, nil
}

// fillKredensialStatus mengisi sub-status pengiriman email kredensial dari
// outbox (best-effort). Bila belum ada baris (belum diproses) → "menunggu".
func (s *pendaftaranQuerySvc) fillKredensialStatus(ctx context.Context, item *domain.Pendaftaran, resp *domain.PendaftaranTrackingResponse) {
	email := item.Email
	resp.KredensialStatus = "menunggu"
	if s.outboxRepo != nil {
		row, err := s.outboxRepo.LatestByPendaftaran(ctx, item.ID,
			string(domain.EmailOutboxSetPassword), string(domain.EmailOutboxAkunTerhubung))
		if err == nil && row != nil {
			email = row.ToEmail
			switch row.Status {
			case domain.EmailOutboxSent:
				resp.KredensialStatus = "terkirim"
			case domain.EmailOutboxFailed:
				resp.KredensialStatus = "gagal"
			default:
				resp.KredensialStatus = "menunggu"
			}
		}
	}
	resp.KredensialEmail = svcutil.MaskEmail(email)
}

// GetDetail melayani admin: tolak objek di luar wilayah kerja aktor (RULES 7).
func (s *pendaftaranQuerySvc) GetDetail(ctx context.Context, id int, actor domain.ActorContext) (*domain.PendaftaranAdminDetail, error) {
	if id <= 0 {
		return nil, domain.NewValidationError("ID pendaftaran tidak valid")
	}
	if s.repo == nil {
		return nil, domain.NewValidationError("Repository pendaftaran belum tersedia")
	}
	item, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !actor.CanAccessWilayah(item.ProvinsiID, item.KabupatenID) {
		return nil, domain.NewForbiddenError("Pendaftaran di luar wilayah kerja Anda")
	}
	out := &domain.PendaftaranAdminDetail{Pendaftaran: *item}
	if s.wilayahRepo != nil {
		if prov, kab, err := s.wilayahRepo.GetNames(ctx, item.ProvinsiID, item.KabupatenID); err == nil {
			out.ProvinsiNama, out.KabupatenNama = prov, kab
		} else {
			log.Warn().Err(err).Int("pendaftaran_id", id).Msg("GetDetail: gagal resolve nama wilayah")
		}
	}
	return out, nil
}

// ListQueue mengembalikan antrean sesuai jurisdiction aktor + filter status.
func (s *pendaftaranQuerySvc) ListQueue(ctx context.Context, actor domain.ActorContext, status string, page, limit int) ([]domain.PendaftaranQueueItem, int, error) {
	if s.repo == nil {
		return nil, 0, svcutil.Unavailable("pendaftaran")
	}
	// Lazy expiry: tandai DRAFT yang sudah lewat 30 hari menjadi KEDALUWARSA.
	_ = s.repo.ExpireStaleDrafts(ctx, 30)
	st := strings.TrimSpace(status)
	if st != "" && !domain.PendaftaranStatus(st).IsValid() {
		return nil, 0, domain.NewValidationError("Filter status tidak valid")
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

	provID, kabID, err := actor.Scope()
	if err != nil {
		return nil, 0, err
	}

	items, err := s.repo.ListQueue(ctx, provID, kabID, st, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.repo.CountQueue(ctx, provID, kabID, st)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// ListQueueCursor varian keyset (tanpa COUNT + tanpa OFFSET besar).
func (s *pendaftaranQuerySvc) ListQueueCursor(ctx context.Context, actor domain.ActorContext, status, cursor string, limit int) ([]domain.PendaftaranQueueItem, string, error) {
	if s.listRepo == nil {
		return nil, "", svcutil.Unavailable("pendaftaran")
	}
	_ = s.repo.ExpireStaleDrafts(ctx, 30)
	st := strings.TrimSpace(status)
	if st != "" && !domain.PendaftaranStatus(st).IsValid() {
		return nil, "", domain.NewValidationError("Filter status tidak valid")
	}
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}
	provID, kabID, err := actor.Scope()
	if err != nil {
		return nil, "", err
	}
	at := time.Now().UTC().Add(time.Hour)
	id := svcutil.MaxInt4
	if strings.TrimSpace(cursor) != "" {
		var err error
		at, id, err = keyset.Decode(cursor)
		if err != nil {
			return nil, "", domain.NewValidationError("Cursor tidak valid")
		}
	}
	items, err := s.listRepo.ListQueueKeyset(ctx, provID, kabID, st, at, id, limit+1)
	if err != nil {
		return nil, "", err
	}
	next := ""
	if len(items) > limit {
		items = items[:limit]
		last := items[len(items)-1]
		next = keyset.Encode(last.CreatedAt, last.ID)
	}
	return items, next, nil
}
