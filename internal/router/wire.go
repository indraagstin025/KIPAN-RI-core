package router

// wire.go memilih implementasi gateway/infrastruktur dari konfigurasi
// (dev vs produksi) — bagian dari composition root.

import (
	"context"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/kipan-indonesia/sim-kipan-core/config"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
	"github.com/kipan-indonesia/sim-kipan-core/internal/service"
	storagepkg "github.com/kipan-indonesia/sim-kipan-core/pkg/storage"
)

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
