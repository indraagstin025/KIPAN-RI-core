package database

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"github.com/kipan-indonesia/sim-kipan-core/config"
)

// ConnectRedis menginisialisasi koneksi ke instance Redis
func ConnectRedis(cfg *config.Config) (*redis.Client, error) {
	if cfg.Redis.Addr == "" {
		log.Warn().Msg("REDIS_ADDR kosong - Redis tidak diaktifkan")
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
		log.Warn().Err(err).Msg("Gagal ping Redis - cache dinonaktifkan sementara")
		return nil, nil
	}

	log.Info().Msg("Redis terhubung")
	return rdb, nil
}