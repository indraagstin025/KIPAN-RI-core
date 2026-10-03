package service

// EmailWorker adalah worker pengirim antrian email (outbox) — poll berkala,
// kirim via SMTP (Mailtrap sandbox/dev), tandai sent/failed dengan backoff.
// Untuk jenis SET_PASSWORD, worker menerbitkan token set-password (Redis,
// prefix `pwsetup:`) lalu mengirim tautan (Opsi A: tanpa password plaintext).

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"github.com/kipan-indonesia/sim-kipan-core/config"
	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/gateway"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/crypto"
)

// setupKey menyimpan token di Redis berdasarkan hash (tanpa token mentah).
func setupKey(rawToken string) string {
	return "pwsetup:" + crypto.HashToken(rawToken)
}

// EmailWorker poller antrian email.
type EmailWorker struct {
	outbox      repository.EmailOutboxRepository
	mail        gateway.MailSender
	rdb         *redis.Client
	userRepo    repository.UserRepository
	cfg         *config.Config
	interval    time.Duration
	batch       int
	maxAttempts int
	setupTTL    time.Duration
}

func NewEmailWorker(
	cfg *config.Config,
	outbox repository.EmailOutboxRepository,
	mail gateway.MailSender,
	rdb *redis.Client,
	userRepo repository.UserRepository,
) *EmailWorker {
	interval, batch, maxAttempts, setupTTL := 10*time.Second, 25, 5, 168*time.Hour
	if cfg != nil {
		if cfg.Outbox.Interval > 0 {
			interval = cfg.Outbox.Interval
		}
		if cfg.Outbox.Batch > 0 {
			batch = cfg.Outbox.Batch
		}
		if cfg.Outbox.MaxAttempts > 0 {
			maxAttempts = cfg.Outbox.MaxAttempts
		}
		if cfg.Outbox.SetupTokenTTL > 0 {
			setupTTL = cfg.Outbox.SetupTokenTTL
		}
	}
	return &EmailWorker{
		outbox: outbox, mail: mail, rdb: rdb, userRepo: userRepo, cfg: cfg,
		interval: interval, batch: batch, maxAttempts: maxAttempts, setupTTL: setupTTL,
	}
}

// Run menjalankan loop poll sampai ctx dibatalkan.
func (w *EmailWorker) Run(ctx context.Context) {
	if w == nil || w.outbox == nil || w.mail == nil {
		log.Warn().Msg("Email worker tidak dijalankan (dependensi tidak lengkap)")
		return
	}
	log.Info().Dur("interval", w.interval).Int("batch", w.batch).Msg("Email worker aktif (outbox)")
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Email worker berhenti")
			return
		case <-ticker.C:
			w.drain(ctx)
		}
	}
}

// drain mengirim satu batch item yang siap.
func (w *EmailWorker) drain(ctx context.Context) {
	items, err := w.outbox.ClaimNext(ctx, w.batch)
	if err != nil {
		log.Warn().Err(err).Msg("Email worker: gagal claim antrian")
		return
	}
	for i := range items {
		w.processOne(ctx, items[i])
	}
}

func backoffFor(attempts int) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	if attempts > 6 {
		attempts = 6
	}
	return time.Duration(1<<uint(attempts-1)) * time.Minute
}

func (w *EmailWorker) processOne(ctx context.Context, it domain.EmailOutbox) {
	subject, text, html := it.Subject, it.TextBody, ""
	if it.HTMLBody != nil {
		html = *it.HTMLBody
	}

	if it.Jenis == domain.EmailOutboxSetPassword {
		if it.UserID == nil || w.rdb == nil || w.userRepo == nil {
			w.fail(ctx, it, "konfigurasi set-password tidak lengkap", true)
			return
		}
		user, err := w.userRepo.GetByID(ctx, *it.UserID)
		if err != nil {
			w.fail(ctx, it, "user tidak ditemukan untuk set-password", true)
			return
		}
		token, err := crypto.GenerateSecureToken(32)
		if err != nil {
			w.fail(ctx, it, "gagal menerbitkan token", false)
			return
		}
		if err := w.rdb.Set(ctx, setupKey(token), user.ID, w.setupTTL).Err(); err != nil {
			w.fail(ctx, it, "gagal menyimpan token set-password", false)
			return
		}
		link := publicURLFrom(w.cfg) + "/set-password?token=" + token
		c := AccountSetupEmail(user.Name, link)
		subject, text, html = c.Subject, c.TextBody, c.HTMLBody
	}

	sendCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if err := w.mail.Send(sendCtx, it.ToEmail, subject, text, html); err != nil {
		w.fail(ctx, it, err.Error(), it.Attempts >= w.maxAttempts)
		return
	}
	if err := w.outbox.MarkSent(ctx, it.ID); err != nil {
		log.Warn().Err(err).Int64("outbox_id", it.ID).Msg("Email terkirim tetapi gagal menandai sent")
	}
}

func (w *EmailWorker) fail(ctx context.Context, it domain.EmailOutbox, msg string, dead bool) {
	next := time.Now().Add(backoffFor(it.Attempts))
	log.Warn().Int64("outbox_id", it.ID).Str("jenis", string(it.Jenis)).Bool("dead", dead).
		Msg("Email worker: pengiriman gagal")
	_ = w.outbox.MarkFailed(ctx, it.ID, msg, next, dead)
}
