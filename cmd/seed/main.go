// Package main — Seeder untuk data minimal SIM-KIPAN Core.
//
// Tujuan: menyiapkan data yang dibutuhkan oleh pentest_suite.ps1 dan
// pengujian manual (login, RBAC, scope wilayah).
//
// KARAKTERISTIK:
//   - Idempotent: aman dijalankan berkali-kali
//   - Password TIDAK di-hardcode: diambil dari environment SEED_ADMIN_PASSWORD;
//     bila kosong, seeder membuat password acak kuat (ditampilkan SEKALI)
//   - Password di-hash dengan Argon2id (parameter identik dengan auth_service.go)
//   - Lookup ID provinsi/kabupaten secara DINAMIS dari DB (tidak hardcode)
//   - Fail-closed: HANYA boleh dijalankan di APP_ENV development/test/local
//
// Cara pakai:
//
//	cd backend
//	$env:SEED_ADMIN_PASSWORD='<password-kuat>'; go run ./cmd/seed   # PowerShell
//	SEED_ADMIN_PASSWORD='<password-kuat>' go run ./cmd/seed         # bash
//
// Mode admin tunggal (provisioning per wilayah, tanpa menyentuh 5 akun fix):
//
//	go run ./cmd/seed -email a@x.id -role ADMIN_KABUPATEN -prov-kode 32 -kab-kode 3273 [-name "..."]
//	go run ./cmd/seed -email b@x.id -role ADMIN_PROVINSI -prov-kode 33
package main

import (
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	"math/big"
	"net/url"
	"os"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

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
	// PasswordEnvVar — nama environment variable yang menentukan password
	// akun admin yang di-seed. WAJIB diisi; jika kosong seeder membuat
	// password acak sendiri (ditampilkan SEKALI di output).
	PasswordEnvVar = "SEED_ADMIN_PASSWORD"

	// minPasswordLength — panjang minimum password seeder. Lebih ketat dari
	// rule password_strength di pkg/validator (min 8) karena akun ini adalah
	// akun admin dengan hak akses tinggi.
	minPasswordLength = 12

	// Email user testing (harus sama dengan pentest_suite.ps1)
	SuperAdminEmail    = "superadmin@kipan.id"
	NasionalAdminEmail = "adminnasional@kipan.id"
	ProvAdminEmail     = "adminprov.jabar@kipan.id"
	KabAdminEmail      = "adminkab.bandung@kipan.id"
	UserKaderEmail     = "user.kader@kipan.id"

	// Kode BPS (dipakai untuk lookup, bukan untuk ID)
	JawaBaratKode   = "32"
	KotaBandungKode = "3273"
)

// allowedSeedEnvs — seeder HANYA boleh berjalan di environment berikut.
// Fail-closed: environment yang tidak terdaftar (termasuk "production")
// ditolak, sehingga tidak ada satu pun string yang bisa "lolos".
var allowedSeedEnvs = map[string]bool{
	"development": true,
	"dev":         true,
	"test":        true,
	"local":       true,
}

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
	TipeUser    string
	ProvinsiID  *int
	KabupatenID *int
}

// ============================================================
// Entry point
// ============================================================

func main() {
	setupLogger()

	var singleEmail = flag.String("email", "", "Email akun admin tunggal (mode single; kosongkan untuk seed 5 akun fix)")
	var singleRole = flag.String("role", "", "Role: ADMIN_PROVINSI | ADMIN_KABUPATEN (wajib bila -email diisi)")
	var singleProv = flag.String("prov-kode", "", "Kode BPS provinsi 2 digit (cth. 32)")
	var singleKab = flag.String("kab-kode", "", "Kode BPS kabupaten 4 digit (wajib untuk ADMIN_KABUPATEN)")
	var singleName = flag.String("name", "", "Nama tampilan (opsional; default dari wilayah)")
	var allRegions = flag.Bool("all-regions", false, "Selain 5 akun tetap, buat 1 admin per provinsi & kabupaten (email adminprov.<kode> / adminkab.<kode> @kipan.id)")
	flag.Parse()

	log.Info().Msg("🌱 Memulai seeder data minimal...")

	v := loadEnv()

	// Guard FAIL-CLOSED: seeder hanya boleh berjalan di environment lokal.
	// APP_ENV dibaca lewat viper agar nilai di file .env juga terbaca.
	// (Sebelumnya hanya os.Getenv, sehingga APP_ENV yang hanya ada di .env
	// tidak terdeteksi dan seeder tetap jalan di server produksi.)
	appEnv := strings.ToLower(strings.TrimSpace(v.GetString("APP_ENV")))
	if appEnv == "" {
		appEnv = "development"
	}
	if !allowedSeedEnvs[appEnv] {
		log.Fatal().
			Str("app_env", appEnv).
			Msg("❌ Seeder ditolak: hanya boleh dijalankan di environment development/test/local")
	}

	password, generated, err := resolveSeedPassword(v)
	if err != nil {
		log.Fatal().Err(err).Msg("❌ Konfigurasi password seeder tidak valid")
	}

	db := mustConnectDB(v)
	defer func() {
		if err := db.Close(); err != nil {
			log.Warn().Err(err).Msg("Gagal menutup koneksi DB")
		}
	}()

	// Timeout longgar: hashing Argon2id per akun (ratusan akun regional).
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	// Mode admin tunggal: hanya buat/refresh SATU akun admin regional.
	// Dipakai untuk provisioning 38 provinsi / 514 kab-kota tanpa
	// menyentuh 5 akun fix. Kode wilayah wajib sudah ada di DB (tidak
	// auto-create agar salah ketik tidak melahirkan wilayah fiktif).
	if strings.TrimSpace(*singleEmail) != "" {
		if err := seedSingleAdmin(ctx, db, password, singleAdminArgs{
			Email:    *singleEmail,
			Role:     *singleRole,
			ProvKode: *singleProv,
			KabKode:  *singleKab,
			Name:     *singleName,
		}); err != nil {
			log.Fatal().Err(err).Msg("❌ Seeder admin tunggal gagal")
		}
		return
	}

	if err := seedAll(ctx, db, password); err != nil {
		log.Fatal().Err(err).Msg("❌ Seeder gagal")
	}

	if *allRegions {
		if err := seedRegions(ctx, db, password); err != nil {
			log.Fatal().Err(err).Msg("❌ Seeder admin regional gagal")
		}
	}

	printSummary(password, generated)
}

// ============================================================
// Konfigurasi environment & password
// ============================================================

// loadEnv membaca konfigurasi dari file .env (opsional) lalu environment.
// Dipakai bersama oleh guard APP_ENV, resolusi password, dan koneksi DB.
// Sengaja TIDAK memanggil config.Load() karena seeder tidak butuh crypto key.
func loadEnv() *viper.Viper {
	v := viper.New()
	v.SetConfigName(".env")
	v.SetConfigType("env")
	v.AddConfigPath(".")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	_ = v.ReadInConfig()
	return v
}

// resolveSeedPassword menentukan password akun admin yang akan di-seed.
//
// Aturan (RULES #9 — dilarang memakai password default produksi):
//  1. Utamakan nilai SEED_ADMIN_PASSWORD dari environment / .env.
//  2. Jika kosong, buat password acak kuat secara kriptografis.
//  3. Tolak password yang tidak memenuhi syarat kekuatan.
//
// Nilai `generated` menandai password dibuat otomatis, sehingga hanya
// password hasil generate yang ditampilkan ke stdout.
func resolveSeedPassword(v *viper.Viper) (password string, generated bool, err error) {
	raw := strings.TrimSpace(v.GetString(PasswordEnvVar))
	if raw == "" {
		pw, genErr := generateStrongPassword(24)
		if genErr != nil {
			return "", false, fmt.Errorf("gagal membuat password acak: %w", genErr)
		}
		return pw, true, nil
	}

	if err := validateSeedPassword(raw); err != nil {
		return "", false, fmt.Errorf("%s tidak memenuhi syarat: %w", PasswordEnvVar, err)
	}
	return raw, false, nil
}

// validateSeedPassword memastikan password memenuhi syarat minimum:
// panjang >= minPasswordLength serta memuat huruf besar, huruf kecil,
// angka, dan simbol (selaras dengan rule password_strength).
func validateSeedPassword(pw string) error {
	if utf8.RuneCountInString(pw) < minPasswordLength {
		return fmt.Errorf("minimal %d karakter", minPasswordLength)
	}

	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, ch := range pw {
		switch {
		case unicode.IsUpper(ch):
			hasUpper = true
		case unicode.IsLower(ch):
			hasLower = true
		case unicode.IsDigit(ch):
			hasDigit = true
		case unicode.IsPunct(ch) || unicode.IsSymbol(ch):
			hasSpecial = true
		}
	}

	if !hasUpper || !hasLower || !hasDigit || !hasSpecial {
		return fmt.Errorf("wajib memuat huruf besar, huruf kecil, angka, dan simbol")
	}
	return nil
}

// passwordAlphabet — karakter untuk password acak. Simbol yang dipilih aman
// untuk shell dan file .env (tanpa kutip, backslash, dolar, atau spasi).
const passwordAlphabet = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789!@#%^*-_=+?"

// generateStrongPassword membuat password acak dengan crypto/rand.
// Setiap kelas karakter dijamin muncul minimal satu kali.
func generateStrongPassword(length int) (string, error) {
	if length < minPasswordLength {
		length = minPasswordLength
	}

	classes := []string{
		"abcdefghijkmnopqrstuvwxyz",
		"ABCDEFGHJKLMNPQRSTUVWXYZ",
		"23456789",
		"!@#%^*-_=+?",
	}

	out := make([]byte, 0, length)
	for _, class := range classes {
		idx, err := randInt(len(class))
		if err != nil {
			return "", err
		}
		out = append(out, class[idx])
	}

	for len(out) < length {
		idx, err := randInt(len(passwordAlphabet))
		if err != nil {
			return "", err
		}
		out = append(out, passwordAlphabet[idx])
	}

	// Acak posisi agar kelas karakter tidak selalu berada di depan.
	for i := len(out) - 1; i > 0; i-- {
		j, err := randInt(i + 1)
		if err != nil {
			return "", err
		}
		out[i], out[j] = out[j], out[i]
	}

	return string(out), nil
}

// randInt mengembalikan angka acak pada rentang [0, max) dari crypto/rand.
func randInt(max int) (int, error) {
	if max <= 0 {
		return 0, nil
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0, err
	}
	return int(n.Int64()), nil
}

func setupLogger() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: time.RFC3339,
	})
}

func printSummary(password string, generated bool) {
	log.Info().Msg("✅ Seeder selesai dengan sukses")
	log.Info().Msg("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	if generated {
		log.Warn().Msg("🔐 Password acak dibuat untuk sesi seeder ini.")
		log.Warn().Msg("   Password hanya ditampilkan di sini dan TIDAK disimpan di mana pun.")
		log.Warn().Msg("   Salin sekarang ke password manager sebelum terminal ditutup.")
		// M-3 (RULES 12): password TIDAK boleh masuk structured log — field
		// zerolog bisa diteruskan ke agregator (Loki/ELK/file). Cetak sekali
		// ke stdout murni yang tidak melewati logger.
		fmt.Println("   → Password akun admin (tampilkan sekali, jangan commit): " + password)
	} else {
		log.Info().
			Str("env", PasswordEnvVar).
			Msg("   → Password diambil dari environment (sengaja tidak ditampilkan)")
	}

	log.Info().Msg("Kredensial login untuk testing:")
	log.Info().
		Str("email", SuperAdminEmail).
		Str("role", "SUPER_ADMIN").
		Msg("  → Super Admin")
	log.Info().
		Str("email", NasionalAdminEmail).
		Str("role", "ADMIN_NASIONAL").
		Msg("  → Admin DPP Nasional")
	log.Info().
		Str("email", ProvAdminEmail).
		Str("role", "ADMIN_PROVINSI").
		Msg("  → Admin DPD Jawa Barat")
	log.Info().
		Str("email", KabAdminEmail).
		Str("role", "ADMIN_KABUPATEN").
		Msg("  → Admin DPC Kota Bandung")
	log.Info().
		Str("email", UserKaderEmail).
		Str("role", "USER (KADER, tanpa anggota terhubung)").
		Msg("  → Kader Uji")
	log.Info().Msg("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}

// ============================================================
// Database connection
// ============================================================

// mustConnectDB membuka koneksi PostgreSQL memakai konfigurasi dari loadEnv.
// Sengaja TIDAK memanggil config.Load() karena seeder tidak butuh crypto key.
func mustConnectDB(v *viper.Viper) *sqlx.DB {
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

func seedAll(ctx context.Context, db *sqlx.DB, password string) error {
	provID, err := seedProvinsi(ctx, db)
	if err != nil {
		return fmt.Errorf("seed provinsi: %w", err)
	}

	kabID, err := seedKabupaten(ctx, db, provID)
	if err != nil {
		return fmt.Errorf("seed kabupaten: %w", err)
	}

	if err := seedUsers(ctx, db, password, provID, kabID); err != nil {
		return fmt.Errorf("seed users: %w", err)
	}

	log.Info().
		Int("provinsi_id", provID).
		Int("kabupaten_id", kabID).
		Msg("📌 ID aktual dari DB — sesuaikan pentest_suite.ps1 jika berbeda")
	return nil
}

// ============================================================
// Mode admin tunggal (provisioning regional)
// ============================================================

type singleAdminArgs struct {
	Email    string
	Role     string
	ProvKode string
	KabKode  string
	Name     string
}

// seedSingleAdmin membuat/me-refresh SATU akun ADMIN_PROVINSI /
// ADMIN_KABUPATEN untuk kode wilayah yang SUDAH ADA di DB. Idempoten
// (reuse upsertUser). Gagal keras bila kode tak dikenal agar salah ketik
// tidak melahirkan wilayah fiktif.
func seedSingleAdmin(ctx context.Context, db *sqlx.DB, password string, a singleAdminArgs) error {
	email := strings.TrimSpace(a.Email)
	if len(email) < 5 || len(email) > 255 || !strings.Contains(email, "@") {
		return fmt.Errorf("email tidak valid: %q", a.Email)
	}
	role := strings.ToUpper(strings.TrimSpace(a.Role))
	if role != "ADMIN_PROVINSI" && role != "ADMIN_KABUPATEN" {
		return fmt.Errorf("role harus ADMIN_PROVINSI atau ADMIN_KABUPATEN (dapat %q)", a.Role)
	}
	provKode := strings.TrimSpace(a.ProvKode)
	if len(provKode) != 2 {
		return fmt.Errorf("prov-kode harus 2 digit BPS (dapat %q)", a.ProvKode)
	}
	var provID int
	var provNama string
	if err := db.QueryRowxContext(ctx,
		`SELECT id, nama FROM wilayah_provinsi WHERE kode = $1 AND is_active`,
		provKode).Scan(&provID, &provNama); err != nil {
		return fmt.Errorf("provinsi kode %q tidak dikenal di DB: %w", provKode, err)
	}

	var kabID *int
	var kabNama string
	if role == "ADMIN_KABUPATEN" {
		kabKode := strings.TrimSpace(a.KabKode)
		if len(kabKode) != 4 {
			return fmt.Errorf("kab-kode harus 4 digit BPS untuk ADMIN_KABUPATEN (dapat %q)", a.KabKode)
		}
		var id int
		var nama string
		var ownerProv int
		if err := db.QueryRowxContext(ctx,
			`SELECT id, nama, provinsi_id FROM wilayah_kabupaten WHERE kode = $1 AND is_active`,
			kabKode).Scan(&id, &nama, &ownerProv); err != nil {
			return fmt.Errorf("kabupaten kode %q tidak dikenal di DB: %w", kabKode, err)
		}
		if ownerProv != provID {
			return fmt.Errorf("kabupaten %q bukan bagian provinsi %q", kabKode, provKode)
		}
		kabID, kabNama = &id, nama
	} else if strings.TrimSpace(a.KabKode) != "" {
		return fmt.Errorf("kab-kode hanya untuk ADMIN_KABUPATEN")
	}

	name := strings.TrimSpace(a.Name)
	if name == "" {
		if role == "ADMIN_KABUPATEN" {
			name = "Admin Kabupaten/Kota " + kabNama
		} else {
			name = "Admin Provinsi " + provNama
		}
	}

	u := seedUser{Email: email, Name: name, Role: role, ProvinsiID: &provID, KabupatenID: kabID}
	if err := upsertUser(ctx, db, u, password); err != nil {
		return err
	}
	log.Info().
		Str("email", email).
		Str("role", role).
		Str("wilayah", name).
		Msg("✅ Akun admin regional siap (password dari environment / acak sesi ini)")
	return nil
}

// ============================================================
// Seed admin regional (bulk)
// ============================================================

// seedRegions membuat/merefresh satu akun ADMIN_PROVINSI per provinsi dan
// satu akun ADMIN_KABUPATEN per kabupaten/kota. Tiap akun di-hash terpisah
// (salt unik) di dalam upsertUser. Email berpola:
//
//	adminprov.<kode-bps>@kipan.id
//	adminkab.<kode-bps>@kipan.id
func seedRegions(ctx context.Context, db *sqlx.DB, password string) error {
	type provRow struct {
		ID   int    `db:"id"`
		Kode string `db:"kode"`
		Nama string `db:"nama"`
	}
	var provs []provRow
	if err := db.SelectContext(ctx, &provs,
		`SELECT id, kode, nama FROM wilayah_provinsi WHERE is_active ORDER BY kode`); err != nil {
		return fmt.Errorf("baca provinsi: %w", err)
	}
	for _, p := range provs {
		id := p.ID
		u := seedUser{
			Email:      "adminprov." + p.Kode + "@kipan.id",
			Name:       "Admin Provinsi " + p.Nama,
			Role:       "ADMIN_PROVINSI",
			ProvinsiID: &id,
		}
		if err := upsertUser(ctx, db, u, password); err != nil {
			return fmt.Errorf("upsert %s: %w", u.Email, err)
		}
	}

	type kabRow struct {
		ID         int    `db:"id"`
		Kode       string `db:"kode"`
		Nama       string `db:"nama"`
		ProvinsiID int    `db:"provinsi_id"`
	}
	var kabs []kabRow
	if err := db.SelectContext(ctx, &kabs,
		`SELECT id, kode, nama, provinsi_id FROM wilayah_kabupaten WHERE is_active ORDER BY kode`); err != nil {
		return fmt.Errorf("baca kabupaten: %w", err)
	}
	for _, k := range kabs {
		id := k.ID
		provID := k.ProvinsiID
		u := seedUser{
			Email:       "adminkab." + k.Kode + "@kipan.id",
			Name:        "Admin Kabupaten/Kota " + k.Nama,
			Role:        "ADMIN_KABUPATEN",
			ProvinsiID:  &provID,
			KabupatenID: &id,
		}
		if err := upsertUser(ctx, db, u, password); err != nil {
			return fmt.Errorf("upsert %s: %w", u.Email, err)
		}
	}

	log.Info().
		Int("provinsi", len(provs)).
		Int("kabupaten", len(kabs)).
		Msg("✅ Admin regional dibuat/di-refresh")
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

// seedUsers membuat / memperbarui akun pengujian memakai `password`
// yang diberikan pemanggil. Password TIDAK di-hardcode di sini (RULES #9).
func seedUsers(ctx context.Context, db *sqlx.DB, password string, provinsiID, kabupatenID int) error {
	users := []seedUser{
		{
			Email: SuperAdminEmail,
			Name:  "Super Admin DPP",
			Role:  "SUPER_ADMIN",
		},
		{
			Email: NasionalAdminEmail,
			Name:  "Sekretariat DPP KIPAN Nasional",
			Role:  "ADMIN_NASIONAL",
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
		{
			Email:       UserKaderEmail,
			Name:        "Kader Uji Kota Bandung",
			Role:        "USER",
			TipeUser:    "KADER",
			ProvinsiID:  &provinsiID,
			KabupatenID: &kabupatenID,
		},
	}

	for _, u := range users {
		if err := upsertUser(ctx, db, u, password); err != nil {
			return fmt.Errorf("upsert user %s: %w", u.Email, err)
		}
	}

	return nil
}

// upsertUser insert user jika belum ada, atau update (password + role + wilayah)
// jika sudah ada. Password di-hash DI SINI sehingga tiap akun dapat salt acak
// sendiri (hash berbeda walau passwordnya sama). Aman dijalankan berkali-kali.
func upsertUser(ctx context.Context, db *sqlx.DB, u seedUser, password string) error {
	hash, err := argon2id.CreateHash(password, argon2Params)
	if err != nil {
		return fmt.Errorf("gagal hash password: %w", err)
	}

	var existingID string
	err = db.GetContext(ctx, &existingID,
		`SELECT id FROM users WHERE LOWER(email) = LOWER($1) AND deleted_at IS NULL`,
		u.Email)

	if err == nil {
		// User sudah ada — update supaya state konsisten untuk test
		tipe := resolveTipe(u)
		_, err = db.ExecContext(ctx, `
			UPDATE users
			SET password_hash = $1,
			    role          = $2,
			    tipe_user     = $3,
			    status        = 'Aktif',
			    name          = $4,
			    provinsi_id   = $5,
			    kabupaten_id  = $6,
			    updated_at    = NOW()
			WHERE id = $7
		`, hash, u.Role, tipe, u.Name, u.ProvinsiID, u.KabupatenID, existingID)
		if err != nil {
			return fmt.Errorf("update: %w", err)
		}
		log.Info().Str("email", u.Email).Str("role", u.Role).
			Msg("User sudah ada — password/role/wilayah di-refresh")
		return nil
	}

	// User belum ada — insert baru
	tipe := resolveTipe(u)
	_, err = db.ExecContext(ctx, `
		INSERT INTO users (id, email, password_hash, name, role, tipe_user, status,
		                   provinsi_id, kabupaten_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, 'Aktif', $7, $8, NOW(), NOW())
	`, uuid.NewString(), u.Email, hash, u.Name, u.Role, tipe, u.ProvinsiID, u.KabupatenID)
	if err != nil {
		return fmt.Errorf("insert: %w", err)
	}

	log.Info().Str("email", u.Email).Str("role", u.Role).Msg("User baru dibuat")
	return nil
}

// resolveTipe menentukan tipe_user: akun admin (role<>USER) => ADMIN;
// akun USER => TipeUser yang diberikan, default KADER.
func resolveTipe(u seedUser) string {
	if u.Role != "USER" {
		return "ADMIN"
	}
	if t := strings.TrimSpace(u.TipeUser); t != "" {
		return t
	}
	return "KADER"
}
