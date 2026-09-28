package config

import (
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	App      AppConfig
	Database DatabaseConfig
	Redis    RedisConfig
	Auth     AuthConfig
	Storage  StorageConfig
	Crypto   CryptoConfig
}

type AppConfig struct {
	Name        string
	Env         string
	Port        string
	AllowOrigin string
	Debug       bool
	// L-7: proxy tambahan yang dipercaya (koma, IP/CIDR).
	TrustedProxies string
	// Base URL publik verifikasi KTA yang tertanam di QR (tanpa trailing /).
	KTAVerifyBaseURL string
}

type DatabaseConfig struct {
	DSN             string
	Host            string
	Port            string
	User            string
	Password        string
	Name            string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type AuthConfig struct {
	AccessTokenSecret string
	AccessTokenTTL    time.Duration
	RefreshTokenTTL   time.Duration
	// L-6: atribut cookie refresh configurable dengan default aman.
	// Longgarkan (Lax / path /) hanya bila frontend beda-site, dengan
	// memahami risiko CSRF yang meningkat (SameSite=None wajib Secure).
	CookieSameSite string
	CookiePath     string
	CookieDomain   string
}

type StorageConfig struct {
	Endpoint        string
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	BucketPublic    string
	BucketPrivate   string
	BucketUploads   string
	PresignedTTL    time.Duration
}

type CryptoConfig struct {
	AESMasterKey  string
	BlindIndexKey string
	KTASigningKey string
	// KTASigningKeyPrev adalah kunci KTA sebelumnya (opsional, verify-only)
	// untuk rotasi tanpa mematikan kartu lama. Kosongkan bila tak dipakai.
	KTASigningKeyPrev string
}

// Load membaca konfigurasi dari environment + file .env (opsional),
// lalu memvalidasi seluruh nilai kritis sebelum server diizinkan start.
//
// Prinsip: FAIL-FAST. Jika ada satu kunci kriptografi tidak valid,
// server MENOLAK start — tidak boleh ada fallback default yang membahayakan.
func Load() (*Config, error) {
	v := viper.New()
	v.SetConfigName(".env")
	v.SetConfigType("env")
	v.AddConfigPath(".")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	_ = v.ReadInConfig() // .env bersifat opsional jika env sudah di-export

	// ============================================================
	// Defaults (NON-SECRET saja; kunci kriptografi WAJIB dari env)
	// ============================================================
	v.SetDefault("APP_NAME", "SIM-KIPAN Core API")
	v.SetDefault("APP_ENV", "development")
	v.SetDefault("APP_PORT", "8080")
	v.SetDefault("APP_DEBUG", false)
	v.SetDefault("APP_ALLOW_ORIGIN", "http://localhost:5173")
	v.SetDefault("APP_TRUSTED_PROXIES", "")

	v.SetDefault("DB_HOST", "127.0.0.1")
	v.SetDefault("DB_PORT", "5432")
	v.SetDefault("DB_USER", "postgres")
	v.SetDefault("DB_PASSWORD", "")
	v.SetDefault("DB_NAME", "kipan_core")
	v.SetDefault("DB_SSLMODE", "disable")
	v.SetDefault("DB_MAX_OPEN_CONNS", 25)
	v.SetDefault("DB_MAX_IDLE_CONNS", 5)
	v.SetDefault("DB_CONN_MAX_LIFETIME", "5m")

	v.SetDefault("REDIS_ADDR", "127.0.0.1:6379")
	v.SetDefault("REDIS_DB", 0)

	v.SetDefault("AUTH_ACCESS_TOKEN_TTL", "15m")
	v.SetDefault("AUTH_REFRESH_TOKEN_TTL", "168h")
	v.SetDefault("AUTH_COOKIE_SAMESITE", "Strict")
	v.SetDefault("AUTH_COOKIE_PATH", "/api/v1/auth")
	v.SetDefault("AUTH_COOKIE_DOMAIN", "")
	v.SetDefault("KTA_VERIFY_BASE_URL", "https://kipan.id")

	v.SetDefault("STORAGE_REGION", "auto")
	v.SetDefault("STORAGE_BUCKET_PUBLIC", "kipan-public")
	v.SetDefault("STORAGE_BUCKET_PRIVATE", "kipan-private")
	v.SetDefault("STORAGE_BUCKET_UPLOADS", "kipan-uploads")
	v.SetDefault("STORAGE_PRESIGNED_TTL", "5m")

	// ============================================================
	// Parse durations
	// ============================================================
	connLifetime := mustParseDuration(v.GetString("DB_CONN_MAX_LIFETIME"), 5*time.Minute)
	accessTTL := mustParseDuration(v.GetString("AUTH_ACCESS_TOKEN_TTL"), 15*time.Minute)
	refreshTTL := mustParseDuration(v.GetString("AUTH_REFRESH_TOKEN_TTL"), 7*24*time.Hour)
	presignedTTL := mustParseDuration(v.GetString("STORAGE_PRESIGNED_TTL"), 5*time.Minute)

	// ============================================================
	// Build DSN
	//   Fix C-13: user & password di-URL-encode agar aman terhadap
	//   karakter spesial (@, :, /, ?, #, spasi) yang bisa merusak parsing.
	// ============================================================
	dsn := v.GetString("DATABASE_URL")
	dbHost := v.GetString("DB_HOST")
	dbPort := v.GetString("DB_PORT")
	dbUser := v.GetString("DB_USER")
	dbPassword := v.GetString("DB_PASSWORD")
	dbName := v.GetString("DB_NAME")
	dbSSLMode := v.GetString("DB_SSLMODE")

	if dsn == "" && dbHost != "" {
		encodedUser := url.QueryEscape(dbUser)
		encodedPass := url.QueryEscape(dbPassword)

		if dbPassword != "" {
			dsn = fmt.Sprintf(
				"postgres://%s:%s@%s:%s/%s?sslmode=%s",
				encodedUser, encodedPass, dbHost, dbPort, dbName, dbSSLMode,
			)
		} else {
			dsn = fmt.Sprintf(
				"postgres://%s@%s:%s/%s?sslmode=%s",
				encodedUser, dbHost, dbPort, dbName, dbSSLMode,
			)
		}
	}

	cfg := &Config{
		App: AppConfig{
			Name:           v.GetString("APP_NAME"),
			Env:            v.GetString("APP_ENV"),
			Port:           v.GetString("APP_PORT"),
			AllowOrigin:    v.GetString("APP_ALLOW_ORIGIN"),
			Debug:          v.GetBool("APP_DEBUG"),
			TrustedProxies:   v.GetString("APP_TRUSTED_PROXIES"),
			KTAVerifyBaseURL: strings.TrimRight(strings.TrimSpace(v.GetString("KTA_VERIFY_BASE_URL")), "/"),
		},
		Database: DatabaseConfig{
			DSN:             dsn,
			Host:            dbHost,
			Port:            dbPort,
			User:            dbUser,
			Password:        dbPassword,
			Name:            dbName,
			SSLMode:         dbSSLMode,
			MaxOpenConns:    v.GetInt("DB_MAX_OPEN_CONNS"),
			MaxIdleConns:    v.GetInt("DB_MAX_IDLE_CONNS"),
			ConnMaxLifetime: connLifetime,
		},
		Redis: RedisConfig{
			Addr:     v.GetString("REDIS_ADDR"),
			Password: v.GetString("REDIS_PASSWORD"),
			DB:       v.GetInt("REDIS_DB"),
		},
		Auth: AuthConfig{
			AccessTokenSecret: v.GetString("AUTH_ACCESS_TOKEN_SECRET"),
			CookieSameSite:    v.GetString("AUTH_COOKIE_SAMESITE"),
			CookiePath:        v.GetString("AUTH_COOKIE_PATH"),
			CookieDomain:      v.GetString("AUTH_COOKIE_DOMAIN"),
			AccessTokenTTL:    accessTTL,
			RefreshTokenTTL:   refreshTTL,
		},
		Storage: StorageConfig{
			Endpoint:        v.GetString("STORAGE_ENDPOINT"),
			Region:          v.GetString("STORAGE_REGION"),
			AccessKeyID:     v.GetString("STORAGE_ACCESS_KEY_ID"),
			SecretAccessKey: v.GetString("STORAGE_SECRET_ACCESS_KEY"),
			BucketPublic:    v.GetString("STORAGE_BUCKET_PUBLIC"),
			BucketPrivate:   v.GetString("STORAGE_BUCKET_PRIVATE"),
			BucketUploads:   v.GetString("STORAGE_BUCKET_UPLOADS"),
			PresignedTTL:    presignedTTL,
		},
		Crypto: CryptoConfig{
			AESMasterKey:      v.GetString("AES_MASTER_KEY"),
			BlindIndexKey:     v.GetString("BLIND_INDEX_KEY"),
			KTASigningKey:     v.GetString("KTA_SIGNING_KEY"),
			KTASigningKeyPrev: v.GetString("KTA_SIGNING_KEY_PREV"),
		},
	}

	// ============================================================
	// Validation (Fail-fast)
	// ============================================================
	if err := validateCryptoKeys(cfg.Crypto); err != nil {
		return nil, err
	}
	if err := validateAuth(cfg.Auth); err != nil {
		return nil, err
	}
	if cfg.App.Env != "development" && cfg.Database.DSN == "" {
		return nil, fmt.Errorf(
			"konfigurasi database wajib diset (DATABASE_URL atau DB_HOST/DB_NAME) di environment %s",
			cfg.App.Env,
		)
	}
	// L-5: validasi khusus production (fail-fast sebelum listen).
	if cfg.App.Env == "production" {
		if err := validateProduction(cfg); err != nil {
			return nil, err
		}
	}

	return cfg, nil
}

// validateProduction menolak konfigurasi berbahaya di production (L-5).
// Dev/test/local tidak terpengaruh agar ergonomi lokal tetap jalan.
func validateProduction(cfg *Config) error {
	if cfg.Database.SSLMode != "require" {
		return fmt.Errorf("production menolak DB_SSLMODE=%q (wajib require/TLS)", cfg.Database.SSLMode)
	}
	if cfg.App.Debug {
		return fmt.Errorf("production menolak APP_DEBUG=true (cetak route + stack trace)")
	}
	if strings.TrimSpace(cfg.Redis.Password) == "" {
		return fmt.Errorf("production menolak REDIS_PASSWORD kosong")
	}
	if strings.Contains(cfg.App.AllowOrigin, "*") {
		return fmt.Errorf("production menolak APP_ALLOW_ORIGIN=%q (wildcard + credentials rawan CSRF)", cfg.App.AllowOrigin)
	}
	if strings.TrimSpace(cfg.Storage.Endpoint) == "" {
		return fmt.Errorf("production menolak STORAGE_ENDPOINT kosong (verifikasi dokumen wajib)")
	}
	return nil
}

// mustParseDuration parse durasi; fallback ke default jika gagal.
// Tidak mengembalikan error karena nilai default sudah aman.
func mustParseDuration(raw string, fallback time.Duration) time.Duration {
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return fallback
	}
	return d
}

// validateCryptoKeys memastikan ketiga kunci kriptografi:
//   - Ada (tidak kosong)
//   - Tepat 64 karakter hex (= 32 byte)
//   - Berformat hex valid (bisa di-decode)
//   - Semua berbeda satu sama lain
//
// Fix C-10: sebelumnya hanya cek panjang; sekarang cek format hex juga,
// sehingga server menolak start jika kunci bukan hex (mencegah panic runtime).
func validateCryptoKeys(c CryptoConfig) error {
	keys := map[string]string{
		"AES_MASTER_KEY":  c.AESMasterKey,
		"BLIND_INDEX_KEY": c.BlindIndexKey,
		"KTA_SIGNING_KEY": c.KTASigningKey,
	}

	for name, val := range keys {
		if val == "" {
			return fmt.Errorf(
				"kunci kriptografi %s wajib diset (32-byte hex = 64 karakter)", name)
		}
		if len(val) != 64 {
			return fmt.Errorf(
				"kunci kriptografi %s harus tepat 64 karakter hex (32 byte), saat ini %d karakter",
				name, len(val))
		}
		// Fix C-10: verifikasi format hex yang sebenarnya
		if _, err := hex.DecodeString(val); err != nil {
			return fmt.Errorf(
				"kunci kriptografi %s bukan hex valid: %w. "+
					"Generate dengan: openssl rand -hex 32",
				name, err)
		}
	}

	if c.AESMasterKey == c.BlindIndexKey {
		return fmt.Errorf("AES_MASTER_KEY dan BLIND_INDEX_KEY harus berbeda")
	}
	if c.AESMasterKey == c.KTASigningKey {
		return fmt.Errorf("AES_MASTER_KEY dan KTA_SIGNING_KEY harus berbeda")
	}
	if c.BlindIndexKey == c.KTASigningKey {
		return fmt.Errorf("BLIND_INDEX_KEY dan KTA_SIGNING_KEY harus berbeda")
	}

	// Kunci rotasi (opsional): bila diisi wajib 64-char hex valid dan
	// berbeda dari kunci aktif. Kunci aktif dipakai untuk tanda tangan
	// baru; kunci lama hanya untuk verifikasi kartu lama.
	if c.KTASigningKeyPrev != "" {
		if len(c.KTASigningKeyPrev) != 64 {
			return fmt.Errorf(
				"kunci kriptografi KTA_SIGNING_KEY_PREV harus tepat 64 karakter hex (32 byte), saat ini %d karakter",
				len(c.KTASigningKeyPrev))
		}
		if _, err := hex.DecodeString(c.KTASigningKeyPrev); err != nil {
			return fmt.Errorf(
				"kunci kriptografi KTA_SIGNING_KEY_PREV bukan hex valid: %w. "+
					"Generate dengan: openssl rand -hex 32",
				err)
		}
		if c.KTASigningKeyPrev == c.KTASigningKey {
			return fmt.Errorf("KTA_SIGNING_KEY_PREV harus berbeda dari KTA_SIGNING_KEY")
		}
		if c.KTASigningKeyPrev == c.AESMasterKey || c.KTASigningKeyPrev == c.BlindIndexKey {
			return fmt.Errorf("KTA_SIGNING_KEY_PREV harus berbeda dari kunci lainnya")
		}
	}

	return nil
}

// validateAuth memastikan konfigurasi auth memiliki secret yang cukup kuat.
func validateAuth(a AuthConfig) error {
	if a.AccessTokenSecret == "" {
		return fmt.Errorf("AUTH_ACCESS_TOKEN_SECRET wajib diset")
	}
	if len(a.AccessTokenSecret) < 32 {
		return fmt.Errorf(
			"AUTH_ACCESS_TOKEN_SECRET terlalu pendek: %d karakter, minimum 32",
			len(a.AccessTokenSecret))
	}
	if a.AccessTokenTTL <= 0 {
		return fmt.Errorf("AUTH_ACCESS_TOKEN_TTL harus > 0")
	}
	// Batas atas fail-fast (M-2): access token wajib berumur pendek agar
	// jendela penyalahgunaan token curian tetap kecil dan blacklist Redis
	// tidak membengkak. Default proyek 15 menit; toleransi maks 30 menit.
	if a.AccessTokenTTL > 30*time.Minute {
		return fmt.Errorf(
			"AUTH_ACCESS_TOKEN_TTL terlalu lama: %s, maksimum 30m "+
				"(gunakan refresh token rotation untuk sesi panjang)",
			a.AccessTokenTTL)
	}
	if a.RefreshTokenTTL <= 0 {
		return fmt.Errorf("AUTH_REFRESH_TOKEN_TTL harus > 0")
	}
	// L-6: SameSite hanya boleh Strict/Lax/None. None membutuhkan Secure
	// (browser modern menolak None tanpa Secure); di production flag Secure
	// selalu true sehingga None valid untuk frontend beda-site via HTTPS.
	switch a.CookieSameSite {
	case "Strict", "Lax", "None", "":
		// "" diperlakukan sebagai Strict oleh handler.
	default:
		return fmt.Errorf("AUTH_COOKIE_SAMESITE harus Strict, Lax, atau None (dapat %q)", a.CookieSameSite)
	}
	return nil
}