package service

// Helper lintas-service (R2): implementasi tunggal untuk pola yang
// sebelumnya diduplikasi per service.
//
// Catatan arsitektur: helper ini SENGAJA di paket internal/service,
// bukan pkg/audit — pkg tidak boleh bergantung pada internal/repository
// (arah dependensi harus ke dalam: Handler → Service → Repository).

import (
	"context"

	"github.com/rs/zerolog/log"

	"github.com/kipan-indonesia/sim-kipan-core/config"
	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
)

// writeAudit mencatat jejak audit secara best-effort (RULES 21).
// Kegagalan tulis TIDAK menggagalkan operasi utama (availability) —
// hanya diperingatkan di log server. PII tidak pernah masuk metadata;
// pemanggil wajib memasking sebelum memanggil helper ini.
func writeAudit(
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

// degradedSkip melaporkan dependensi yang belum dikonfigurasi.
// Kembali true (lewati dengan warning) hanya di non-production;
// di production pemanggil wajib gagal fail-closed (return 503).
func degradedSkip(cfg *config.Config, label string) bool {
	if cfg != nil && cfg.App.Env == "production" {
		return false
	}
	log.Warn().Str("dep", label).
		Msg("Dependensi tidak dikonfigurasi — dilewati (HANYA non-production)")
	return true
}

// unavailable membangun error 503 generik untuk dependensi yang belum
// di-wire (mis. repo nil di production). Satu konstruktor agar pesan
// konsisten dan tidak membocorkan detail wiring internal (BE-006).
func unavailable(layanan string) error {
	return domain.NewUnavailableError("Layanan " + layanan + " sedang tidak tersedia")
}
