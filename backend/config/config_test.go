package config

import (
	"testing"
	"time"
)

func baseProdConfig() *Config {
	return &Config{
		App:      AppConfig{Env: "production", AllowOrigin: "https://sim.kipan.id"},
		Database: DatabaseConfig{SSLMode: "require"},
		Redis:    RedisConfig{Addr: "127.0.0.1:6379", Password: "secret"},
		Storage: StorageConfig{Endpoint: "https://is3.cloudhost.id"},
		Auth: AuthConfig{
			AccessTokenSecret: "0123456789abcdef0123456789abcdef",
			AccessTokenTTL:    15 * time.Minute,
			RefreshTokenTTL:   168 * time.Hour,
		},
	}
}

func TestValidateProductionAcceptsSafeConfig(t *testing.T) {
	if err := validateProduction(baseProdConfig()); err != nil {
		t.Fatalf("config aman ditolak: %v", err)
	}
}

func TestValidateProductionRejectsDangerous(t *testing.T) {
	base := baseProdConfig()

	insecure := *base
	insecure.Database.SSLMode = "disable"
	if err := validateProduction(&insecure); err == nil {
		t.Error("sslmode=disable DITERIMA di production")
	}

	debug := *base
	debug.App.Debug = true
	if err := validateProduction(&debug); err == nil {
		t.Error("APP_DEBUG=true DITERIMA di production")
	}

	noRedisPass := *base
	noRedisPass.Redis.Password = ""
	if err := validateProduction(&noRedisPass); err == nil {
		t.Error("REDIS_PASSWORD kosong DITERIMA di production")
	}

	wildcard := *base
	wildcard.App.AllowOrigin = "*"
	if err := validateProduction(&wildcard); err == nil {
		t.Error("ALLOW_ORIGIN=* DITERIMA di production")
	}

	noStorage := *base
	noStorage.Storage.Endpoint = ""
	if err := validateProduction(&noStorage); err == nil {
		t.Error("STORAGE_ENDPOINT kosong DITERIMA di production")
	}
}

func TestValidateAuthRejectsLongTTL(t *testing.T) {
	a := AuthConfig{
		AccessTokenSecret: "0123456789abcdef0123456789abcdef",
		AccessTokenTTL:    720 * time.Hour,
		RefreshTokenTTL:   168 * time.Hour,
	}
	if err := validateAuth(a); err == nil {
		t.Error("TTL 720 jam DITERIMA (M-2 regresi)")
	}
}
