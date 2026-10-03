package config

import (
	"testing"
	"time"
)

func baseProdConfig() *Config {
	return &Config{
		App:      AppConfig{Env: "production", AllowOrigin: "https://sim.kipan.id", PublicURL: "https://sim.kipan.id"},
		Database: DatabaseConfig{SSLMode: "require"},
		Redis:    RedisConfig{Addr: "127.0.0.1:6379", Password: "secret"},
		Storage:  StorageConfig{Endpoint: "https://is3.cloudhost.id"},
		Auth: AuthConfig{
			AccessTokenSecret: "0123456789abcdef0123456789abcdef",
			AccessTokenTTL:    15 * time.Minute,
			RefreshTokenTTL:   168 * time.Hour,
		},
		WA: WAConfig{Provider: "fonnte", FonnteToken: "fonnte-token"},
		Mail: MailConfig{
			Host: "smtp.mailtrap.io", Port: 587, Username: "user", Password: "pass",
			FromEmail: "no-reply@kipan.id", FromName: "KIPAN",
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

func TestNormalizeOrigins(t *testing.T) {
	cases := map[string]string{
		"https://a.com, https://b.com": "https://a.com,https://b.com",
		"  https://a.com  ":            "https://a.com",
		"https://a.com,,https://b.com": "https://a.com,https://b.com",
		"":                             "",
	}
	for in, want := range cases {
		if got := normalizeOrigins(in); got != want {
			t.Errorf("normalizeOrigins(%q) = %q, harap %q", in, got, want)
		}
	}
}

func TestValidateTrustedProxies(t *testing.T) {
	aman := []string{"", "127.0.0.1", "::1", "10.0.0.5", "10.0.0.0/8, 172.16.0.1", " 203.0.113.7 , 2001:db8::/32 "}
	for _, raw := range aman {
		if err := validateTrustedProxies(raw); err != nil {
			t.Errorf("%q seharusnya valid: %v", raw, err)
		}
	}

	tolak := []string{"*", "10.0.0.1, *", "bukan-ip", "10.0.0.1/99", "192.168.1"}
	for _, raw := range tolak {
		if err := validateTrustedProxies(raw); err == nil {
			t.Errorf("%q seharusnya ditolak", raw)
		}
	}
}

func TestValidateProductionRejectsWildcardProxy(t *testing.T) {
	base := baseProdConfig()
	proxy := *base
	proxy.App.TrustedProxies = "10.0.0.1, *"
	if err := validateProduction(&proxy); err == nil {
		t.Error("APP_TRUSTED_PROXIES wildcard DITERIMA di production")
	}

	badEntry := *base
	badEntry.App.TrustedProxies = "bukan-ip"
	if err := validateProduction(&badEntry); err == nil {
		t.Error("entri trusted proxy tidak valid DITERIMA di production")
	}
}

func TestValidateProductionWA(t *testing.T) {
	base := baseProdConfig()

	logProvider := *base
	logProvider.WA = WAConfig{Provider: "log"}
	if err := validateProduction(&logProvider); err == nil {
		t.Error("WA_GATEWAY_PROVIDER=log DITERIMA di production (OTP bocor ke log)")
	}

	emptyToken := *base
	emptyToken.WA = WAConfig{Provider: "fonnte", FonnteToken: ""}
	if err := validateProduction(&emptyToken); err == nil {
		t.Error("FONNTE_TOKEN_KEY kosong DITERIMA di production")
	}

	noPublicURL := *base
	noPublicURL.App.PublicURL = ""
	if err := validateProduction(&noPublicURL); err == nil {
		t.Error("APP_PUBLIC_URL kosong DITERIMA di production")
	}
}

func TestValidateProductionMail(t *testing.T) {
	base := baseProdConfig()

	noHost := *base
	noHost.Mail.Host = ""
	if err := validateProduction(&noHost); err == nil {
		t.Error("MAIL_HOST kosong DITERIMA di production")
	}

	localHost := *base
	localHost.Mail.Host = "127.0.0.1"
	if err := validateProduction(&localHost); err == nil {
		t.Error("MAIL_HOST lokal DITERIMA di production")
	}

	noUser := *base
	noUser.Mail.Username = ""
	if err := validateProduction(&noUser); err == nil {
		t.Error("MAIL_USERNAME kosong DITERIMA di production")
	}

	badPort := *base
	badPort.Mail.Port = 0
	if err := validateProduction(&badPort); err == nil {
		t.Error("MAIL_PORT=0 DITERIMA di production")
	}
}

func TestValidateAuthCookieSameSite(t *testing.T) {
	base := AuthConfig{
		AccessTokenSecret: "0123456789abcdef0123456789abcdef",
		AccessTokenTTL:    15 * time.Minute,
		RefreshTokenTTL:   168 * time.Hour,
		CookieSameSite:    "Strict",
	}
	if err := validateAuth(base); err != nil {
		t.Fatalf("Strict ditolak: %v", err)
	}
	bad := base
	bad.CookieSameSite = "Sometimes"
	if err := validateAuth(bad); err == nil {
		t.Error("SameSite invalid DITERIMA")
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
