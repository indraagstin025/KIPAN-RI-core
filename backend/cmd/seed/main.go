// Package main — Seeder untuk data minimal SIM-KIPAN Core.
//
// Tujuan: menyiapkan data yang dibutuhkan oleh pentest_suite.ps1 dan
// pengujian manual (login, RBAC, scope wilayah).
//
// KARAKTERISTIK:
//   - Idempotent: aman dijalankan berkali-kali
//   - Password di-hash dengan Argon2id (parameter identik dengan auth_service.go)
//   - Lookup ID provinsi/kabupaten secara DINAMIS dari DB (tidak hardcode)
//   - TIDAK BOLEH dijalankan di production (destructive if misused)
//
// Cara pakai:
//   cd backend
//   go run ./cmd/seed
package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/alexedwards/argon2id"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// ============================================================
// Konstanta seeder
// ============================================================

const (
	// Password untuk SEMUA user testing. Ganti jika perlu.
	DefaultPassword = "AdminKipan2026!"

	// Email user testing (harus sama dengan pentest_suite.ps1)
	SuperAdminEmail = "superadmin@kipan.id"
	ProvAdminEmail  = "adminprov.jabar@kipan.id"
	KabAdminEmail   = "adminkab.bandung@kipan.id"

	// Kode BPS (dipakai untuk lookup, bukan untuk ID)
	JawaBaratKode   = "32"
	KotaBandungKode = "3273"
)

// Parameter Argon2id — WAJIB sama dengan auth_service.go agar
// hash yang di-seed bisa diverifikasi oleh ComparePasswordAndHash.
var argon2Params = &argon2id.Params{
	Memory:      64 * 1024, // 64 MB
	Iterations:  3,
	Parallelism: 2,
	SaltLength:  16,
	KeyLength:   32,
}

// ============================================================
// Struct seed
// ============================================================

type seedUser struct {
	Email       string
	Name        string
	Role        string
	ProvinsiID  *int
	KabupatenID *int
}

// ============================================================
// Entry point
// ============================================================

func main() {
	setupLogger()

	log.Info().Msg("🌱 Memulai seeder data minimal...")

	// Guard: cegah eksekusi di production
	if os.Getenv("APP_ENV") == "production" {
		log.Fatal().Msg("❌ Seeder tidak boleh dijalankan di environment production")
	}

	db := mustConnectDB()
	defer func() {
		if err := db.Close(); err != nil {
			log.Warn().Err(err).Msg("Gagal menutup koneksi DB")
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := seedAll(ctx, db); err != nil {
		log.Fatal().Err(err).Msg("❌ Seeder gagal")
	}

	printSummary()
}

func setupLogger() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: time.RFC3339,
	})
}

func printSummary() {
	log.Info().Msg("✅ Seeder selesai dengan sukses")
	log.Info().Msg("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Info().Msg("Kredensial login untuk testing:")
	log.Info().
		Str("email", SuperAdminEmail).
		Str("password", DefaultPassword).
		Str("role", "SUPER_ADMIN").
		Msg("  → Super Admin")
	log.Info().
		Str("email", ProvAdminEmail).
		Str("password", DefaultPassword).
		Str("role", "ADMIN_PROVINSI").
		Msg("  → Admin DPD Jawa Barat")
	log.Info().
		Str("email", KabAdminEmail).
		Str("password", DefaultPassword).
		Str("role", "ADMIN_KABUPATEN").
		Msg("  → Admin DPC Kota Bandung")
	log.Info().Msg("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}

// ============================================================
// Database connection
// ============================================================

// mustConnectDB membaca konfigurasi dari .env (jika ada) atau environment.
// Sengaja TIDAK memanggil config.Load() karena seeder tidak butuh crypto key.
func mustConnectDB() *sqlx.DB {
	v := viper.New()
	v.SetConfigName(".env")
	v.SetConfigType("env")
	v.AddConfigPath(".")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	_ = v.ReadInConfig()

	dsn := buildDSN(v)
	if dsn == "" {
		log.Fatal().Msg("Konfigurasi DB tidak ditemukan. Set DATABASE_URL atau DB_HOST/DB_NAME.")
	}

	db, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		log.Fatal().Err(err).Msg("Gagal koneksi ke PostgreSQL")
	}

	log.Info().Msg("Terhubung ke PostgreSQL")
	return db
}

func buildDSN(v *viper.Viper) string {
	if dsn := v.GetString("DATABASE_URL"); dsn != "" {
		return dsn
	}

	host := getDefault(v, "DB_HOST", "127.0.0.1")
	port := getDefault(v, "DB_PORT", "5432")
	user := getDefault(v, "DB_USER", "postgres")
	pass := v.GetString("DB_PASSWORD")
	name := getDefault(v, "DB_NAME", "kipan_core")
	sslmode := getDefault(v, "DB_SSLMODE", "disable")

	if pass != "" {
		return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
			url.QueryEscape(user), url.QueryEscape(pass), host, port, name, sslmode)
	}
	return fmt.Sprintf("postgres://%s@%s:%s/%s?sslmode=%s",
		url.QueryEscape(user), host, port, name, sslmode)
}

func getDefault(v *viper.Viper, key, fallback string) string {
	if val := v.GetString(key); val != "" {
		return val
	}
	return fallback
}

// ============================================================
// Seed orchestration
// ============================================================

func seedAll(ctx context.Context, db *sqlx.DB) error {
	provID, err := seedProvinsi(ctx, db)
	if err != nil {
		return fmt.Errorf("seed provinsi: %w", err)
	}

	kabID, err := seedKabupaten(ctx, db, provID)
	if err != nil {
		return fmt.Errorf("seed kabupaten: %w", err)
	}

	if err := seedUsers(ctx, db, provID, kabID); err != nil {
		return fmt.Errorf("seed users: %w", err)
	}

	log.Info().
		Int("provinsi_id", provID).
		Int("kabupaten_id", kabID).
		Msg("📌 ID aktual dari DB — sesuaikan pentest_suite.ps1 jika berbeda")
	return nil
}

// ============================================================
// Seed Provinsi
// ============================================================

// seedProvinsi memastikan Jawa Barat ada, lalu mengembalikan ID aktualnya.
func seedProvinsi(ctx context.Context, db *sqlx.DB) (int, error) {
	var id int
	err := db.GetContext(ctx, &id,
		`SELECT id FROM wilayah_provinsi WHERE kode = $1`, JawaBaratKode)

	if err == nil {
		log.Info().Int("id", id).Str("nama", "Jawa Barat").
			Msg("Provinsi Jawa Barat sudah ada")
		return id, nil
	}

	// Belum ada → insert (tanpa explicit ID, biarkan SERIAL auto-generate)
	_, err = db.ExecContext(ctx, `
		INSERT INTO wilayah_provinsi (kode, nama, is_active, created_at, updated_at)
		VALUES ($1, 'Jawa Barat', TRUE, NOW(), NOW())
		ON CONFLICT (kode) DO NOTHING
	`, JawaBaratKode)
	if err != nil {
		return 0, fmt.Errorf("insert provinsi: %w", err)
	}

	// Ambil ID hasil insert
	if err := db.GetContext(ctx, &id,
		`SELECT id FROM wilayah_provinsi WHERE kode = $1`, JawaBaratKode); err != nil {
		return 0, fmt.Errorf("baca ID provinsi: %w", err)
	}

	log.Info().Int("id", id).Msg("✅ Provinsi Jawa Barat dibuat")
	return id, nil
}

// ============================================================
// Seed Kabupaten
// ============================================================

// seedKabupaten memastikan Kota Bandung ada, lalu mengembalikan ID aktualnya.
func seedKabupaten(ctx context.Context, db *sqlx.DB, provinsiID int) (int, error) {
	var id int
	err := db.GetContext(ctx, &id,
		`SELECT id FROM wilayah_kabupaten WHERE kode = $1`, KotaBandungKode)

	if err == nil {
		log.Info().Int("id", id).Str("nama", "Kota Bandung").
			Msg("Kabupaten Kota Bandung sudah ada")
		return id, nil
	}

	_, err = db.ExecContext(ctx, `
		INSERT INTO wilayah_kabupaten (provinsi_id, kode, nama, is_active, created_at, updated_at)
		VALUES ($1, $2, 'Kota Bandung', TRUE, NOW(), NOW())
		ON CONFLICT (kode) DO NOTHING
	`, provinsiID, KotaBandungKode)
	if err != nil {
		return 0, fmt.Errorf("insert kabupaten: %w", err)
	}

	if err := db.GetContext(ctx, &id,
		`SELECT id FROM wilayah_kabupaten WHERE kode = $1`, KotaBandungKode); err != nil {
		return 0, fmt.Errorf("baca ID kabupaten: %w", err)
	}

	log.Info().Int("id", id).Msg("✅ Kabupaten Kota Bandung dibuat")
	return id, nil
}

// ============================================================
// Seed Users
// ============================================================

func seedUsers(ctx context.Context, db *sqlx.DB, provinsiID, kabupatenID int) error {
	hash, err := argon2id.CreateHash(DefaultPassword, argon2Params)
	if err != nil {
		return fmt.Errorf("gagal hash password: %w", err)
	}

	users := []seedUser{
		{
			Email: SuperAdminEmail,
			Name:  "Super Admin DPP",
			Role:  "SUPER_ADMIN",
		},
		{
			Email:      ProvAdminEmail,
			Name:       "Admin DPD Jawa Barat",
			Role:       "ADMIN_PROVINSI",
			ProvinsiID: &provinsiID,
		},
		{
			Email:       KabAdminEmail,
			Name:        "Admin DPC Kota Bandung",
			Role:        "ADMIN_KABUPATEN",
			ProvinsiID:  &provinsiID,
			KabupatenID: &kabupatenID,
		},
	}

	for _, u := range users {
		if err := upsertUser(ctx, db, u, hash); err != nil {
			return fmt.Errorf("upsert user %s: %w", u.Email, err)
		}
	}

	return nil
}

// upsertUser insert user jika belum ada, atau update (password + role + wilayah)
// jika sudah ada. Aman dijalankan berkali-kali.
func upsertUser(ctx context.Context, db *sqlx.DB, u seedUser, hash string) error {
	var existingID string
	err := db.GetContext(ctx, &existingID,
		`SELECT id FROM users WHERE LOWER(email) = LOWER($1) AND deleted_at IS NULL`,
		u.Email)

	if err == nil {
		// User sudah ada — update supaya state konsisten untuk test
		_, err = db.ExecContext(ctx, `
			UPDATE users
			SET password_hash = $1,
			    role          = $2,
			    status        = 'Aktif',
			    name          = $3,
			    provinsi_id   = $4,
			    kabupaten_id  = $5,
			    updated_at    = NOW()
			WHERE id = $6
		`, hash, u.Role, u.Name, u.ProvinsiID, u.KabupatenID, existingID)
		if err != nil {
			return fmt.Errorf("update: %w", err)
		}
		log.Info().Str("email", u.Email).Str("role", u.Role).
			Msg("User sudah ada — password/role/wilayah di-refresh")
		return nil
	}

	// User belum ada — insert baru
	_, err = db.ExecContext(ctx, `
		INSERT INTO users (id, email, password_hash, name, role, status,
		                   provinsi_id, kabupaten_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, 'Aktif', $6, $7, NOW(), NOW())
	`, uuid.NewString(), u.Email, hash, u.Name, u.Role, u.ProvinsiID, u.KabupatenID)
	if err != nil {
		return fmt.Errorf("insert: %w", err)
	}

	log.Info().Str("email", u.Email).Str("role", u.Role).Msg("User baru dibuat")
	return nil
}