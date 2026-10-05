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
	"strings"

	"github.com/alexedwards/argon2id"
	"github.com/rs/zerolog/log"

	"github.com/kipan-indonesia/sim-kipan-core/config"
	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
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
