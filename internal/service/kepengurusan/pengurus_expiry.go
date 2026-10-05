package kepengurusan

// pengurus_expiry.go — materialisasi + notifikasi kedaluwarsa masa bakti
// pengurus (TDD §5.4): menutup record pengurus Aktif yang SK-nya sudah lewat
// lalu menulis audit DEMISIONER_OTOMATIS per personalia; serta peringatan
// H-30/H-7 sebelum berakhir. Dijalankan berkala oleh cmd/worker.

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
)

// Milestone peringatan kedaluwarsa (hari sebelum tanggal berakhir SK).
const (
	expiryWarn30Days = 30
	expiryWarn7Days  = 7
)

// PengurusExpiryService menutup pengurus kedaluwarsa (materialisasi) dan
// mengirim peringatan sebelum masa bakti berakhir.
type PengurusExpiryService interface {
	// RunOnce menutup seluruh pengurus kedaluwarsa; mengembalikan jumlahnya.
	RunOnce(ctx context.Context) (int, error)
	// WarnExpiringSoon mengirim peringatan H-30/H-7 (sekali per milestone)
	// ke admin sewilayah SK; mengembalikan jumlah peringatan terkirim.
	WarnExpiringSoon(ctx context.Context) (int, error)
}

type pengurusExpirySvc struct {
	pengurus repository.PengurusRepository
	audit    repository.AuditLogRepository
	notif    repository.NotificationRepository
}

// NewPengurusExpiryService membangun service materialisasi kedaluwarsa.
func NewPengurusExpiryService(pengurus repository.PengurusRepository, audit repository.AuditLogRepository, notif repository.NotificationRepository) PengurusExpiryService {
	return &pengurusExpirySvc{pengurus: pengurus, audit: audit, notif: notif}
}

// RunOnce menutup pengurus kedaluwarsa + audit per baris (aktor "Sistem"),
// lalu memberi tahu admin sewilayah SK (best-effort) + anggota yang
// didemosikan bila punya akun.
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
		s.notifyDemotion(ctx, it)
	}
	return len(closed), nil
}

// WarnExpiringSoon mengirim peringatan H-30/H-7 untuk SK yang segera
// berakhir. Tiap milestone dikirim sekali (flag notified_h*_at); baris yang
// sudah ditandai dilewati.
func (s *pengurusExpirySvc) WarnExpiringSoon(ctx context.Context) (int, error) {
	if s.pengurus == nil {
		return 0, nil
	}
	rows, err := s.pengurus.ListExpiringSoon(ctx, expiryWarn30Days)
	if err != nil {
		return 0, err
	}
	now := time.Now()
	sent := 0
	for _, r := range rows {
		daysLeft := int(r.TanggalBerakhir.Sub(now).Hours() / 24)
		// H-7 diprioritaskan: baris yang sudah ≤7 hari tidak lagi
		// mendapat peringatan H-30 yang basi.
		milestone := 0
		switch {
		case daysLeft <= expiryWarn7Days:
			if r.NotifiedH7At == nil {
				milestone = expiryWarn7Days
			}
		case daysLeft <= expiryWarn30Days:
			if r.NotifiedH30At == nil {
				milestone = expiryWarn30Days
			}
		}
		if milestone == 0 {
			continue
		}
		if err := s.notifyWarning(ctx, r, milestone, daysLeft); err != nil {
			log.Warn().Err(err).Int("pengurus_id", r.ID).Int("milestone", milestone).
				Msg("Gagal mengirim peringatan kedaluwarsa; dicoba lagi periode berikut")
			continue
		}
		if err := s.pengurus.MarkExpiryNotified(ctx, r.ID, milestone); err != nil {
			log.Warn().Err(err).Int("pengurus_id", r.ID).
				Msg("Gagal menandai peringatan terkirim")
			continue
		}
		sent++
	}
	return sent, nil
}

// notifyWarning mengirim peringatan H-milestone ke admin sewilayah SK.
func (s *pengurusExpirySvc) notifyWarning(ctx context.Context, r domain.ExpiringAppointment, milestone, daysLeft int) error {
	if s.notif == nil {
		return nil
	}
	prov, kab := 0, 0
	if r.ProvinsiID != nil {
		prov = *r.ProvinsiID
	}
	if r.KabupatenID != nil {
		kab = *r.KabupatenID
	}
	if daysLeft < 0 {
		daysLeft = 0
	}
	title := fmt.Sprintf("Masa bakti SK berakhir %d hari lagi", milestone)
	msg := fmt.Sprintf("SK %s (%s %s) berakhir pada %s (%d hari lagi). Susun SK pengganti.",
		r.NomorSK, r.Jabatan, r.NamaLengkap,
		r.TanggalBerakhir.Format("02-01-2006"), daysLeft)
	return s.notif.NotifyAdmins(ctx, title, msg, domain.NotifTypeSK,
		"#admin/pengurus/"+strconv.Itoa(r.ID), prov, kab)
}

// notifyDemotion memberi tahu admin sewilayah SK + anggota yang didemosikan
// (bila punya akun). Best-effort: kegagalan hanya di-log.
func (s *pengurusExpirySvc) notifyDemotion(ctx context.Context, it domain.ExpiredAppointment) {
	if s.notif == nil {
		return
	}
	prov, kab := 0, 0
	if it.ProvinsiID != nil {
		prov = *it.ProvinsiID
	}
	if it.KabupatenID != nil {
		kab = *it.KabupatenID
	}
	link := "#admin/pengurus/" + strconv.Itoa(it.ID)
	msg := fmt.Sprintf("Masa bakti SK %s berakhir. %s (%s) didemosikan otomatis.",
		it.NomorSK, it.NamaLengkap, it.Jabatan)
	if err := s.notif.NotifyAdmins(ctx, "Pengurus didemosikan otomatis", msg,
		domain.NotifTypeSK, link, prov, kab); err != nil {
		log.Warn().Err(err).Int("pengurus_id", it.ID).
			Msg("Gagal mengirim notifikasi demosi otomatis ke admin")
	}
	if it.UserID != nil && *it.UserID != "" {
		memberMsg := fmt.Sprintf("Masa jabatan Anda sebagai %s (SK %s) telah berakhir. %s",
			it.Jabatan, it.NomorSK, "Hubungi sekretariat untuk informasi kepengurusan berikutnya.")
		if err := s.notif.NotifyUser(ctx, *it.UserID, "Masa jabatan Anda berakhir",
			memberMsg, domain.NotifTypeSK, "#akun/kta"); err != nil {
			log.Warn().Err(err).Int("pengurus_id", it.ID).
				Msg("Gagal mengirim notifikasi demosi otomatis ke anggota")
		}
	}
}
