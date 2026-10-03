// Command worker adalah biner worker terpisah untuk SIM-KIPAN: menjalankan
// antrian email (outbox) dan tugas terjadwal lain (mis. deteksi kedaluwarsa
// masa bakti). Dipisah dari proses API agar aman di-scale >1 instance — setiap
// tugas dilindungi kunci singleton (PostgreSQL advisory lock).
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog/log"

	"github.com/kipan-indonesia/sim-kipan-core/config"
	"github.com/kipan-indonesia/sim-kipan-core/internal/database"
	"github.com/kipan-indonesia/sim-kipan-core/internal/logging"
	"github.com/kipan-indonesia/sim-kipan-core/internal/worker"
)

func main() {
	logging.Setup()

	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("Gagal memuat konfigurasi. Worker tidak dapat berjalan.")
	}

	log.Info().
		Str("env", cfg.App.Env).
		Str("timezone", cfg.Worker.Timezone).
		Msg("SIM-KIPAN Core worker starting...")

	db, err := database.ConnectPostgres(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Gagal menyambung database PostgreSQL")
	}
	if db == nil {
		log.Fatal().Msg("Worker membutuhkan database PostgreSQL")
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Warn().Err(err).Msg("Gagal menutup koneksi database")
		}
	}()

	rdb, err := database.ConnectRedis(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Gagal menyambung Redis")
	}
	if rdb != nil {
		defer func() {
			if err := rdb.Close(); err != nil {
				log.Warn().Err(err).Msg("Gagal menutup koneksi Redis")
			}
		}()
	}

	runner := worker.NewRunner(cfg, db, rdb)

	// Graceful shutdown pada SIGINT/SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := runner.Run(ctx); err != nil {
		log.Error().Err(err).Msg("Worker berhenti karena error")
		return
	}
	log.Info().Msg("Worker berhenti dengan bersih. Selesai.")
}
