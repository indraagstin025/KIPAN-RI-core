// Package main — Utility DEV untuk membersihkan Redis cache/key testing.
//
// Tujuan: memungkinkan pentest_suite.ps1 membersihkan state rate limit
// tanpa memerlukan binary redis-cli eksternal.
//
// ⚠️  HANYA UNTUK DEV/STAGING — jangan dijalankan di production.
//
// Cara pakai:
//   cd backend
//   go run ./cmd/flushcache
package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

// Pattern key yang akan dibersihkan. Semua ini adalah state transient
// untuk testing — TIDAK ADA data bisnis di sini.
var flushPatterns = []string{
	"auth_rate:*",       // Rate limiter per IP (AuthRateLimiter)
	"login_attempt:*",   // Rate limiter per email (LoginAttemptLimiter)
	"blacklist:jti:*",   // JWT blacklist (setelah logout)
}

func main() {
	setupLogger()

	// Guard: cegah eksekusi di production
	if os.Getenv("APP_ENV") == "production" {
		log.Fatal().Msg("❌ flushcache tidak boleh dijalankan di production")
	}

	cfg := loadRedisConfig()
	if cfg.Addr == "" {
		log.Warn().Msg("REDIS_ADDR kosong — tidak ada yang dibersihkan")
		os.Exit(0)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	defer func() {
		if err := rdb.Close(); err != nil {
			log.Warn().Err(err).Msg("Gagal menutup koneksi Redis")
		}
	}()

	// Ping untuk verifikasi koneksi
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatal().Err(err).Msg("Gagal koneksi ke Redis")
	}

	totalDeleted := 0
	for _, pattern := range flushPatterns {
		deleted, err := deleteByPattern(ctx, rdb, pattern)
		if err != nil {
			log.Warn().Err(err).Str("pattern", pattern).Msg("Gagal hapus pattern")
			continue
		}
		if deleted > 0 {
			log.Info().Str("pattern", pattern).Int("count", deleted).Msg("✅ Dihapus")
		}
		totalDeleted += deleted
	}

	if totalDeleted == 0 {
		log.Info().Msg("ℹ️  Tidak ada key yang perlu dibersihkan")
	} else {
		log.Info().Int("total", totalDeleted).Msg("🎉 Redis state dibersihkan")
	}
}

func setupLogger() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: time.RFC3339,
	})
}

type redisConfig struct {
	Addr     string
	Password string
	DB       int
}

func loadRedisConfig() redisConfig {
	v := viper.New()
	v.SetConfigName(".env")
	v.SetConfigType("env")
	v.AddConfigPath(".")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	_ = v.ReadInConfig()

	return redisConfig{
		Addr:     getDefault(v, "REDIS_ADDR", "127.0.0.1:6379"),
		Password: v.GetString("REDIS_PASSWORD"),
		DB:       v.GetInt("REDIS_DB"),
	}
}

func getDefault(v *viper.Viper, key, fallback string) string {
	if val := v.GetString(key); val != "" {
		return val
	}
	return fallback
}

// deleteByPattern menggunakan SCAN (bukan KEYS) untuk menghindari blocking
// Redis di production-scale key space.
func deleteByPattern(ctx context.Context, rdb *redis.Client, pattern string) (int, error) {
	var cursor uint64
	total := 0

	for {
		keys, nextCursor, err := rdb.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return total, fmt.Errorf("scan: %w", err)
		}

		if len(keys) > 0 {
			if err := rdb.Del(ctx, keys...).Err(); err != nil {
				return total, fmt.Errorf("del: %w", err)
			}
			total += len(keys)
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	return total, nil
}