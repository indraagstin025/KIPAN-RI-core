package worker

import (
	"context"
	"strings"
	"time"

	// Sertakan database zona waktu (tzdata) agar time.LoadLocation tetap
	// bekerja di sistem tanpa zoneinfo (mis. Windows).
	_ "time/tzdata"

	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"github.com/kipan-indonesia/sim-kipan-core/config"
	"github.com/kipan-indonesia/sim-kipan-core/internal/infra"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
	"github.com/kipan-indonesia/sim-kipan-core/internal/service"
)

// Runner merangkai seluruh pekerjaan worker: repositori, EmailWorker, dan
// scheduler singleton. Dijalankan oleh biner cmd/worker.
type Runner struct {
	sched *Scheduler
}

// NewRunner membangun Runner dari konfigurasi + koneksi. Repositori yang
// dibutuhkan worker dibangun di sini (bukan dari composition root HTTP).
func NewRunner(cfg *config.Config, db *sqlx.DB, rdb *redis.Client) *Runner {
	emailOutboxRepo := repository.NewEmailOutboxRepository(db)
	userRepo := repository.NewUserRepository(db)
	anggotaRepo := repository.NewAnggotaRepository(db)
	auditRepo := repository.NewAuditLogRepository(db)
	pengurusRepo := repository.NewPengurusRepository(db)
	notifRepo := repository.NewNotificationRepository(db)
	mailSender := infra.MailSender(cfg)

	emailWorker := service.NewEmailWorker(cfg, emailOutboxRepo, mailSender, rdb, userRepo, anggotaRepo, notifRepo)
	expirySvc := service.NewPengurusExpiryService(pengurusRepo, auditRepo)

	sched := NewScheduler(NewAdvisoryLocker(db), resolveLocation(cfg.Worker.Timezone))
	sched.Register(Job{
		Name:      "email-outbox",
		Interval:  cfg.Outbox.Interval,
		Immediate: true,
		Run:       emailWorker.ProcessOnce,
	})
	// Materialisasi kedaluwarsa masa bakti pengurus (TDD §5.4).
	sched.Register(Job{
		Name:      "pengurus-expired",
		Interval:  cfg.Worker.ExpiryInterval,
		Immediate: true,
		Run: func(ctx context.Context) error {
			n, err := expirySvc.RunOnce(ctx)
			if err != nil {
				return err
			}
			if n > 0 {
				log.Info().Int("closed", n).Msg("Kedaluwarsa: pengurus didemosikan otomatis")
			}
			return nil
		},
	})

	return &Runner{sched: sched}
}

// Run menjalankan scheduler sampai ctx dibatalkan (graceful shutdown).
func (r *Runner) Run(ctx context.Context) error {
	log.Info().
		Str("timezone", r.sched.Location().String()).
		Msg("Worker aktif (scheduler)")
	r.sched.Run(ctx)
	return nil
}

// resolveLocation memuat zona waktu IANA; fallback UTC bila kosong/tidak valid.
func resolveLocation(name string) *time.Location {
	name = strings.TrimSpace(name)
	if name == "" {
		return time.UTC
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		log.Warn().Err(err).Str("timezone", name).Msg("Zona waktu tidak valid, fallback UTC")
		return time.UTC
	}
	return loc
}
