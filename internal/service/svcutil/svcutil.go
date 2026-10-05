// Package svcutil menampung helper lintas-service: implementasi tunggal
// untuk pola yang sebelumnya diduplikasi per service.
//
// Catatan arsitektur: helper ini SENGAJA di paket internal/service/svcutil,
// bukan pkg/... — pkg tidak boleh bergantung pada internal/repository
// (arah dependensi harus ke dalam: Handler → Service → Repository).
// Subpackage service (auth, pendaftaran, dst.) mengimpor paket ini;
// paket ini TIDAK boleh mengimpor subpackage service lain (leaf).
package svcutil

import (
	"context"
	"crypto/rand"
	"math/big"
	"regexp"
	"strings"

	"github.com/alexedwards/argon2id"
	"github.com/rs/zerolog/log"

	"github.com/kipan-indonesia/sim-kipan-core/config"
	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/crypto"
)

// WriteAudit mencatat jejak audit secara best-effort (RULES 21).
// Kegagalan tulis TIDAK menggagalkan operasi utama (availability) —
// hanya diperingatkan di log server. PII tidak pernah masuk metadata;
// pemanggil wajib memasking sebelum memanggil helper ini.
func WriteAudit(
	ctx context.Context,
	repo repository.AuditLogRepository,
	tr domain.AuditContext,
	actorID *string,
	actorName, actorRole, entity, entityID, action string,
	metadata *string,
) {
	if repo == nil {
		return
	}
	e := &domain.ActivityLog{
		ActorID:    actorID,
		ActorName:  actorName,
		ActorRole:  actorRole,
		IPAddress:  tr.IP,
		UserAgent:  tr.UserAgent,
		EntityName: entity,
		EntityID:   entityID,
		Action:     action,
		Metadata:   metadata,
		RequestID:  tr.RequestID,
	}
	if err := repo.Create(ctx, e); err != nil {
		log.Warn().
			Err(err).
			Str("action", action).
			Str("entity_id", entityID).
			Msg("Gagal mencatat audit trail")
	}
}

// DegradedSkip melaporkan dependensi yang belum dikonfigurasi.
// Kembali true (lewati dengan warning) hanya di non-production;
// di production pemanggil wajib gagal fail-closed (return 503).
func DegradedSkip(cfg *config.Config, label string) bool {
	if cfg != nil && cfg.App.Env == "production" {
		return false
	}
	log.Warn().Str("dep", label).
		Msg("Dependensi tidak dikonfigurasi — dilewati (HANYA non-production)")
	return true
}

// Unavailable membangun error 503 generik untuk dependensi yang belum
// di-wire (mis. repo nil di production). Satu konstruktor agar pesan
// konsisten dan tidak membocorkan detail wiring internal (BE-006).
func Unavailable(layanan string) error {
	return domain.NewUnavailableError("Layanan " + layanan + " sedang tidak tersedia")
}

// PublicURLFrom mengembalikan base URL frontend untuk tautan email.
func PublicURLFrom(cfg *config.Config) string {
	if cfg != nil && strings.TrimSpace(cfg.App.PublicURL) != "" {
		return strings.TrimRight(strings.TrimSpace(cfg.App.PublicURL), "/")
	}
	return "http://localhost:5173"
}

// Argon2Params adalah parameter Argon2id sesuai OWASP 2025 & spesifikasi
// proyek (Rule 9). Satu definisi bersama agar seluruh service memakai
// parameter hashing yang sama.
var Argon2Params = &argon2id.Params{
	Memory:      64 * 1024, // 64 MB
	Iterations:  3,
	Parallelism: 2,
	SaltLength:  16,
	KeyLength:   32,
}

// memberPasswordAlphabet aman untuk shell/.env (tanpa kutip, backslash,
// dolar, atau spasi) - selaras generator password seeder.
const memberPasswordAlphabet = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789!@#%^*-_=+?"

// GenerateMemberPassword membuat password awal acak (crypto/rand) dengan
// tiap kelas karakter (kecil, besar, digit, simbol) minimal satu.
func GenerateMemberPassword(length int) (string, error) {
	if length < 12 {
		length = 12
	}
	classes := []string{
		"abcdefghijkmnopqrstuvwxyz",
		"ABCDEFGHJKLMNPQRSTUVWXYZ",
		"23456789",
		"!@#%^*-_=+?",
	}
	out := make([]byte, 0, length)
	for _, class := range classes {
		idx, err := RandIntN(len(class))
		if err != nil {
			return "", err
		}
		out = append(out, class[idx])
	}
	for len(out) < length {
		idx, err := RandIntN(len(memberPasswordAlphabet))
		if err != nil {
			return "", err
		}
		out = append(out, memberPasswordAlphabet[idx])
	}
	for i := len(out) - 1; i > 0; i-- {
		j, err := RandIntN(i + 1)
		if err != nil {
			return "", err
		}
		out[i], out[j] = out[j], out[i]
	}
	return string(out), nil
}

// RandIntN mengembalikan angka acak [0, max) dari crypto/rand.
func RandIntN(max int) (int, error) {
	if max <= 0 {
		return 0, nil
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0, err
	}
	return int(n.Int64()), nil
}

// Batas panjang field selaras kolom database. Dipakai bersama alur admin
// dan pendaftaran agar pesan 422 konsisten (bukan 500).
const (
	MaxNamaLen        = 150
	MaxTempatLahirLen = 100
	MaxAlamatLen      = 2000
)

// NipPattern menerima NIK 16 digit.
var NipPattern = regexp.MustCompile(`^\d{16}$`)

// PhonePattern selaras dengan rule id_phone di pkg/validator: prefix
// 0/62/+62, operator 8, digit kedua bukan 0, total 9-14 digit.
var PhonePattern = regexp.MustCompile(`^(\+62|62|0)8[1-9][0-9]{6,10}$`)

// KodePosPattern mewajibkan 5 digit (format Indonesia).
var KodePosPattern = regexp.MustCompile(`^\d{5}$`)

// nomorPattern menerima REG-YYYYMM-XXXX / XXXXX (dan lebih bila periode
// melampaui 99.999 pendaftaran).
var nomorPattern = regexp.MustCompile(`^REG-\d{6}-\d{4,}$`)

// MaskEmail menyamarkan email untuk tampilan publik: 3 karakter pertama
// local-part dipertahankan. mis. ind***@gmail.com.
func MaskEmail(email string) string {
	e := strings.TrimSpace(email)
	at := strings.LastIndexByte(e, '@')
	if at <= 0 {
		return e
	}
	local, domain := e[:at], e[at:]
	if len(local) <= 3 {
		return "***" + domain
	}
	return local[:3] + "***" + domain
}

// NormalizeEmail menyeragamkan email untuk perbandingan bukti pemilik.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// NormalizeWA menyeragamkan nomor WA ke digit inti: buang non-digit lalu
// buang prefix negara/ trunk (62/0) sehingga 08xx, 62xxx, +62xxx setara.
func NormalizeWA(wa string) string {
	var digits strings.Builder
	for _, r := range wa {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	d := digits.String()
	d = strings.TrimPrefix(d, "62")
	d = strings.TrimPrefix(d, "0")
	return d
}

// NormalizeNomor menormalkan nomor pendaftaran (case + spasi).
func NormalizeNomor(nomor string) (string, error) {
	nr := strings.ToUpper(strings.TrimSpace(nomor))
	if nr == "" || len(nr) > 30 || !nomorPattern.MatchString(nr) {
		return "", domain.NewValidationError("Nomor pendaftaran tidak valid")
	}
	return nr, nil
}

// DerefInt mengembalikan nilai dari pointer int (0 bila nil).
func DerefInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

// MaxInt4 = nilai maksimum tipe integer Postgres; dipakai sebagai sentinel id
// saat keyset pagination tanpa cursor agar cocok dengan tipe kolom id.
const MaxInt4 = 2147483647

// SetupKey menyimpan token set-password di Redis berdasarkan hash (tanpa
// token mentah). Dipakai bersama worker email (penerbit) dan reset service
// (verifikator) agar kunci konsisten.
func SetupKey(rawToken string) string {
	return "pwsetup:" + crypto.HashToken(rawToken)
}

// ContainsAngleBracket menolak < > pada field plain-text (anti stored-XSS).
func ContainsAngleBracket(s string) bool {
	return strings.ContainsAny(s, "<>")
}

// IsPlausibleNIKDate memeriksa kewarasan segmen tanggal NIK (digit 7-12).
func IsPlausibleNIKDate(nik string) bool {
	if len(nik) != 16 {
		return false
	}
	dd := int(nik[6]-'0')*10 + int(nik[7]-'0')
	mm := int(nik[8]-'0')*10 + int(nik[9]-'0')
	if mm < 1 || mm > 12 {
		return false
	}
	if (dd >= 1 && dd <= 31) || (dd >= 41 && dd <= 71) {
		return true
	}
	return false
}

// AesKey/BlindIndexKey membaca kunci kripto dari config (R3: fungsi paket).
func AesKey(cfg *config.Config) (string, error) {
	if cfg == nil || strings.TrimSpace(cfg.Crypto.AESMasterKey) == "" {
		return "", domain.NewValidationError("AES_MASTER_KEY belum dikonfigurasi")
	}
	return cfg.Crypto.AESMasterKey, nil
}

func BlindIndexKey(cfg *config.Config) (string, error) {
	if cfg == nil || strings.TrimSpace(cfg.Crypto.BlindIndexKey) == "" {
		return "", domain.NewValidationError("BLIND_INDEX_KEY belum dikonfigurasi")
	}
	return cfg.Crypto.BlindIndexKey, nil
}
