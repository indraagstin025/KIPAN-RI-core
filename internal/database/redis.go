package database

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"github.com/kipan-indonesia/sim-kipan-core/config"
)

// ConnectRedis menginisialisasi koneksi ke instance Redis.
//
// KEAMANAN (fail-closed — RULES #8, #25):
// Redis adalah backing store untuk kontrol keamanan kritis:
//   - blacklist JWT (logout / pencabutan token)
//   - rate limiter auth & login-attempt
//
// Jika Redis mati dan server tetap jalan dengan rdb == nil, kedua kontrol itu
// NONAKTIF diam-diam (isBlacklisted selalu false, limiter tanpa storage
// terdistribusi) = fail-open. Karena itu:
//   - di production: kegagalan koneksi Redis = ERROR, server MENOLAK start.
//   - di development/test/local: diizinkan degraded (rdb == nil) agar dev
//     tetap bisa jalan tanpa Redis, dengan peringatan eksplisit di log.
func ConnectRedis(cfg *config.Config) (*redis.Client, error) {
	env := cfg.App.Env
	isProd := env == "production"

	if cfg.Redis.Addr == "" {
		if isProd {
			return nil, fmt.Errorf(
				"REDIS_ADDR wajib diset di environment production " +
					"(blacklist token & rate limiter membutuhkannya)")
		}
		log.Warn().Msg("REDIS_ADDR kosong - Redis tidak diaktifkan (mode degraded, HANYA untuk development)")
		return nil, nil
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		_ = rdb.Close()
		if isProd {
			return nil, fmt.Errorf("gagal ping Redis di production, server menolak start: %w", err)
		}
		log.Warn().Err(err).Msg(
			"Gagal ping Redis - cache dinonaktifkan sementara " +
				"(mode degraded: blacklist & rate limiter tidak aktif, HANYA untuk development)")
		return nil, nil
	}

	log.Info().Msg("Redis terhubung")
	return rdb, nil
}
