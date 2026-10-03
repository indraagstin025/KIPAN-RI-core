package config

import (
	"encoding/hex"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

type Config struct {
	App      AppConfig
	Database DatabaseConfig
	Redis    RedisConfig
	Auth     AuthConfig
	Storage  StorageConfig
	Crypto   CryptoConfig
	WA       WAConfig
	Mail     MailConfig
	Outbox   OutboxConfig
	Worker   WorkerConfig
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
	// Base URL frontend publik untuk tautan di pesan (lacak, revisi),
	// tanpa trailing slash. Contoh: http://localhost:5173 (dev).
	PublicURL string
	// #2: token rahasia untuk /internal/health. Kosong = endpoint tidak
	// didaftarkan di produksi (fail-closed); di dev dibiarkan terbuka.
	InternalHealthToken string
}

// WAConfig mengatur pengiriman WhatsApp. Produksi WAJIB provider nyata
// (fonnte) + token — LogGateway hanya untuk dev/test.
type WAConfig struct {
	Provider    string // "log" | "fonnte"
	FonnteToken string
}

// MailConfig mengatur SMTP (Mailpit dev / Mailtrap sandbox / produksi).
// Host kosong = LogMailSender (HANYA non-production; produksi fail-fast).
type MailConfig struct {
	Host      string
	Port      int
	Username  string
	Password  string
	FromEmail string
	FromName  string
}

func (m MailConfig) Enabled() bool {
	return strings.TrimSpace(m.Host) != ""
}

// OutboxConfig mengatur worker antrian email (email_outbox).
type OutboxConfig struct {
	Interval      time.Duration // jeda poll worker
	Batch         int           // jumlah email per siklus
	MaxAttempts   int           // batas percobaan sebelum gagal (DLQ)
	SetupTokenTTL time.Duration // masa berlaku tautan set-password
}

// WorkerConfig mengatur biner worker terpisah (cmd/worker): penjadwal tugas
// berkala + zona waktu lokal organisasi (dipakai jadwal harian, mis. deteksi
// kedaluwarsa masa bakti). Zona waktu tidak valid akan fallback ke UTC.
type WorkerConfig struct {
	Timezone string // IANA, mis. "Asia/Jakarta"
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
	v.SetDefault("APP_PUBLIC_URL", "http://localhost:5173")
	v.SetDefault("INTERNAL_HEALTH_TOKEN", "")

	v.SetDefault("WA_GATEWAY_PROVIDER", "log")

	// Email: host kosong = log-only (dev). Set MAIL_HOST=127.0.0.1 untuk
	// Mailpit (port 1025), atau sandbox.smtp.mailtrap.io:2525 untuk Mailtrap.
	v.SetDefault("MAIL_PORT", 1025)
	v.SetDefault("MAIL_FROM_EMAIL", "no-reply@kipan.id")
	v.SetDefault("MAIL_FROM_NAME", "Sistem Informasi KIPAN")

	// Worker antrian email.
	v.SetDefault("EMAIL_WORKER_INTERVAL", "10s")
	v.SetDefault("EMAIL_WORKER_BATCH", 25)
	v.SetDefault("EMAIL_WORKER_MAX_ATTEMPTS", 5)
	v.SetDefault("SETUP_TOKEN_TTL", "168h")

	// Worker terpisah (cmd/worker).
	v.SetDefault("WORKER_TIMEZONE", "Asia/Jakarta")

	v.SetDefault("STORAGE_REGION", "auto")
	v.SetDefault("STORAGE_BUCKET_PUBLIC", "kipan-public")
	v.SetDefault("STORAGE_BUCKET_PRIVATE", "kipan-private")
	v.SetDefault("STORAGE_BUCKET_UPLOADS", "kipan-uploads")
	v.SetDefault("STORAGE_PRESIGNED_TTL", "5m")

	// ============================================================
	// Parse durations
	// ============================================================
	connLifetime := mustParseDuration(v.GetString("DB_CONN_MAX_LIFETIME"), 5*time.Minute, "DB_CONN_MAX_LIFETIME")
	accessTTL := mustParseDuration(v.GetString("AUTH_ACCESS_TOKEN_TTL"), 15*time.Minute, "AUTH_ACCESS_TOKEN_TTL")
	refreshTTL := mustParseDuration(v.GetString("AUTH_REFRESH_TOKEN_TTL"), 7*24*time.Hour, "AUTH_REFRESH_TOKEN_TTL")
	presignedTTL := mustParseDuration(v.GetString("STORAGE_PRESIGNED_TTL"), 5*time.Minute, "STORAGE_PRESIGNED_TTL")
	emailInterval := mustParseDuration(v.GetString("EMAIL_WORKER_INTERVAL"), 10*time.Second, "EMAIL_WORKER_INTERVAL")
	setupTTL := mustParseDuration(v.GetString("SETUP_TOKEN_TTL"), 168*time.Hour, "SETUP_TOKEN_TTL")

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
			Name:                v.GetString("APP_NAME"),
			Env:                 v.GetString("APP_ENV"),
			Port:                v.GetString("APP_PORT"),
			AllowOrigin:         normalizeOrigins(v.GetString("APP_ALLOW_ORIGIN")),
			Debug:               v.GetBool("APP_DEBUG"),
			TrustedProxies:      v.GetString("APP_TRUSTED_PROXIES"),
			KTAVerifyBaseURL:    strings.TrimRight(strings.TrimSpace(v.GetString("KTA_VERIFY_BASE_URL")), "/"),
			PublicURL:           strings.TrimRight(strings.TrimSpace(v.GetString("APP_PUBLIC_URL")), "/"),
			InternalHealthToken: strings.TrimSpace(v.GetString("INTERNAL_HEALTH_TOKEN")),
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
		WA: WAConfig{
			Provider:    strings.ToLower(strings.TrimSpace(v.GetString("WA_GATEWAY_PROVIDER"))),
			FonnteToken: strings.TrimSpace(v.GetString("FONNTE_TOKEN_KEY")),
		},
		Mail: MailConfig{
			Host:      strings.TrimSpace(v.GetString("MAIL_HOST")),
			Port:      v.GetInt("MAIL_PORT"),
			Username:  strings.TrimSpace(v.GetString("MAIL_USERNAME")),
			Password:  v.GetString("MAIL_PASSWORD"),
			FromEmail: strings.TrimSpace(v.GetString("MAIL_FROM_EMAIL")),
			FromName:  strings.TrimSpace(v.GetString("MAIL_FROM_NAME")),
		},
		Outbox: OutboxConfig{
			Interval:      emailInterval,
			Batch:         v.GetInt("EMAIL_WORKER_BATCH"),
			MaxAttempts:   v.GetInt("EMAIL_WORKER_MAX_ATTEMPTS"),
			SetupTokenTTL: setupTTL,
		},
		Worker: WorkerConfig{
			Timezone: strings.TrimSpace(v.GetString("WORKER_TIMEZONE")),
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
	// #10: SameSite=None hanya sah dengan Secure. Produksi selalu Secure
	// (cookie hanya di-set saat APP_ENV != development). Di non-production
	// flag Secure mati sehingga browser modern menolak cookie None — beri
	// peringatan agar tidak bingung "refresh cookie hilang".
	if strings.EqualFold(cfg.Auth.CookieSameSite, "None") && cfg.App.Env != "production" {
		log.Warn().Msg(
			"AUTH_COOKIE_SAMESITE=None di non-production: browser modern menolak cookie tanpa Secure; " +
				"pakai Lax/Strict untuk dev, atau jalankan via HTTPS")
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
	// T5: trusted proxy tidak boleh wildcard (semua X-Forwarded-For dipercaya
	// → rate-limit per-IP bisa dilewati & IP audit dipalsukan). Setiap entri
	// juga wajib IP/CIDR valid agar salah ketik tidak diam-diam diabaikan.
	if err := validateTrustedProxies(cfg.App.TrustedProxies); err != nil {
		return err
	}
	if strings.TrimSpace(cfg.Storage.Endpoint) == "" {
		return fmt.Errorf("production menolak STORAGE_ENDPOINT kosong (verifikasi dokumen wajib)")
	}
	// Batch 2: gateway WA nyata wajib (OTP tidak boleh berakhir di log) +
	// base URL publik untuk tautan pesan.
	if cfg.WA.Provider != "fonnte" {
		return fmt.Errorf(
			"production menolak WA_GATEWAY_PROVIDER=%q (wajib 'fonnte'; gateway log membocorkan OTP ke log)", cfg.WA.Provider)
	}
	if strings.TrimSpace(cfg.WA.FonnteToken) == "" {
		return fmt.Errorf("production menolak FONNTE_TOKEN_KEY kosong")
	}
	if cfg.App.PublicURL == "" {
		return fmt.Errorf("production menolak APP_PUBLIC_URL kosong (tautan lacak di pesan WA/email)")
	}
	// Batch 3: email nyata wajib di produksi (reset password + notifikasi
	// status). SMTP lokal (Mailpit) tidak boleh dipakai di produksi.
	if !cfg.Mail.Enabled() {
		return fmt.Errorf("production menolak MAIL_HOST kosong (reset password & notifikasi email wajib)")
	}
	if h := strings.ToLower(cfg.Mail.Host); h == "127.0.0.1" || h == "localhost" || h == "::1" {
		return fmt.Errorf("production menolak MAIL_HOST=%q (SMTP lokal hanya untuk dev)", cfg.Mail.Host)
	}
	if cfg.Mail.Username == "" {
		return fmt.Errorf("production menolak MAIL_USERNAME kosong (SMTP produksi wajib terautentikasi)")
	}
	if cfg.Mail.Port <= 0 || cfg.Mail.Port > 65535 {
		return fmt.Errorf("production menolak MAIL_PORT=%d (harus 1-65535)", cfg.Mail.Port)
	}
	return nil
}

// normalizeOrigins membereskan daftar origin CORS yang dipisah koma:
// trim spasi tiap entri dan buang yang kosong, lalu gabung kembali. Tanpa
// ini, "https://a.com, https://b.com" membuat origin kedua (spasi di depan)
// tidak cocok (T12).
func normalizeOrigins(raw string) string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return strings.Join(out, ",")
}

// validateTrustedProxies memastikan APP_TRUSTED_PROXIES hanya berisi IP/CIDR
// eksplisit: wildcard ditolak dan entri tak valid dilaporkan (fail-fast),
// bukan diabaikan diam-diam.
func validateTrustedProxies(raw string) error {
	for _, p := range strings.Split(raw, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if p == "*" {
			return fmt.Errorf(
				"APP_TRUSTED_PROXIES menolak wildcard %q (X-Forwarded-For bisa dipalsukan)", p)
		}
		if net.ParseIP(p) != nil {
			continue
		}
		if _, _, err := net.ParseCIDR(p); err != nil {
			return fmt.Errorf(
				"APP_TRUSTED_PROXIES berisi entri tidak valid %q (harus IP atau CIDR)", p)
		}
	}
	return nil
}

// mustParseDuration parse durasi; fallback ke default jika gagal.
// Tidak mengembalikan error karena nilai default sudah aman, tetapi
// mencatat warning bila nilai yang diisi tidak valid (T13) agar salah
// ketik tidak diam-diam diabaikan.
func mustParseDuration(raw string, fallback time.Duration, label string) time.Duration {
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		if strings.TrimSpace(raw) != "" {
			log.Warn().Str("field", label).Str("value", raw).Dur("fallback", fallback).
				Msg("Durasi tidak valid — memakai nilai default")
		}
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
