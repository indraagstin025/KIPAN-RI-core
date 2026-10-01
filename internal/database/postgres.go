package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"

	"github.com/kipan-indonesia/sim-kipan-core/config"
)

// ConnectPostgres menginisialisasi koneksi pool ke database PostgreSQL
func ConnectPostgres(cfg *config.Config) (*sqlx.DB, error) {
	if cfg.Database.DSN == "" {
		log.Warn().Msg("Konfigurasi database kosong - menggunakan mode tanpa database (development only)")
		return nil, nil
	}

	connConfig, err := pgx.ParseConfig(cfg.Database.DSN)
	if err != nil {
		return nil, fmt.Errorf("gagal parse koneksi database: %w", err)
	}

	connStr := stdlib.RegisterConnConfig(connConfig)
	db, err := sqlx.Open("pgx", connStr)
	if err != nil {
		return nil, fmt.Errorf("gagal membuka koneksi DB: %w", err)
	}

	db.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	db.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.Database.ConnMaxLifetime)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		log.Warn().
			Str("host", connConfig.Host).
			Uint16("port", connConfig.Port).
			Str("user", connConfig.User).
			Str("database", connConfig.Database).
			Err(err).
			Msg("Database PostgreSQL belum dapat dihubungi (pastikan service DB aktif)")
	} else {
		log.Info().
			Str("host", connConfig.Host).
			Uint16("port", connConfig.Port).
			Str("user", connConfig.User).
			Str("database", connConfig.Database).
			Msg("Berhasil terhubung ke database PostgreSQL")
	}

	return db, nil
}