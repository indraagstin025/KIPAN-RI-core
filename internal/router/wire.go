package router

// wire.go memilih implementasi gateway/infrastruktur dari konfigurasi
// (dev vs produksi) — bagian dari composition root.

import (
	"context"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/kipan-indonesia/sim-kipan-core/config"
	"github.com/kipan-indonesia/sim-kipan-core/internal/gateway"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
	"github.com/kipan-indonesia/sim-kipan-core/internal/service"
	storagepkg "github.com/kipan-indonesia/sim-kipan-core/pkg/storage"
)

// wireWAGateway memilih implementasi WAGateway dari config: Fonnte untuk
// produksi, LogGateway untuk dev/test (fail-fast produksi dijamin config).
func wireWAGateway(cfg *config.Config) gateway.WAGateway {
	if cfg.WA.Provider == "fonnte" {
		log.Info().Msg("WA gateway aktif: Fonnte")
		return gateway.NewFonnteGateway(cfg.WA.FonnteToken)
	}
	log.Warn().Msg("WA gateway: log-only (HANYA dev/test — jangan dipakai di produksi)")
	return gateway.NewLogGateway()
}

// wireMailSender memilih implementasi MailSender: SMTP bila MAIL_HOST diisi
// (Mailpit dev / Mailtrap sandbox / produksi), selain itu LogMailSender
// (dev/test; produksi dijamin oleh validasi config).
func wireMailSender(cfg *config.Config) gateway.MailSender {
	if cfg.Mail.Enabled() {
		log.Info().
			Str("host", cfg.Mail.Host).
			Int("port", cfg.Mail.Port).
			Msg("Email gateway aktif: SMTP")
		return gateway.NewSMTPSender(gateway.MailConfig{
			Host:      cfg.Mail.Host,
			Port:      cfg.Mail.Port,
			Username:  cfg.Mail.Username,
			Password:  cfg.Mail.Password,
			FromEmail: cfg.Mail.FromEmail,
			FromName:  cfg.Mail.FromName,
		})
	}
	log.Warn().Msg("Email gateway: log-only (set MAIL_HOST untuk SMTP; HANYA dev/test)")
	return gateway.NewLogMailSender()
}

// wireStorageService membangun S3 client + StorageService.
//
// Fail-closed: bila endpoint/kredensial tidak dikonfigurasi, service tetap
// dibangun dengan client nil sehingga RequestUploadPresign/RequestViewPresign
// mengembalikan 503 (bukan bypass diam-diam). Bucket disiapkan otomatis
// hanya di non-production.
func wireStorageService(cfg *config.Config, auditRepo repository.AuditLogRepository, docRepo repository.DocumentRepository) *service.StorageService {
	var client *storagepkg.Client
	if strings.TrimSpace(cfg.Storage.Endpoint) != "" {
		c, err := storagepkg.NewClient(
			cfg.Storage.Endpoint,
			cfg.Storage.Region,
			cfg.Storage.AccessKeyID,
			cfg.Storage.SecretAccessKey,
		)
		if err != nil {
			log.Warn().Err(err).Msg("Storage tidak aktif (konfigurasi tidak lengkap) — presign fail-closed 503")
		} else {
			client = c
		}
	} else {
		log.Warn().Msg("STORAGE_ENDPOINT kosong — presign upload/view nonaktif (fail-closed 503)")
	}

	svc := service.NewStorageService(cfg, client, auditRepo, docRepo)
	if client != nil && cfg.App.Env != "production" {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		svc.EnsureBuckets(ctx)
	}
	return svc
}
