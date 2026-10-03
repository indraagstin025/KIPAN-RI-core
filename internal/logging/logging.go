// Package logging menyediakan setup zerolog yang konsisten untuk seluruh biner
// (API maupun worker). Development memakai console writer berwarna; production
// memakai JSON untuk aggregator (Loki, ELK, Datadog).
package logging

import (
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Setup mengonfigurasi logger global zerolog berdasarkan APP_ENV.
func Setup() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	if os.Getenv("APP_ENV") == "production" {
		log.Logger = zerolog.New(os.Stdout).
			With().
			Timestamp().
			Logger()
		return
	}

	log.Logger = log.Output(zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: time.RFC3339,
	})
}
