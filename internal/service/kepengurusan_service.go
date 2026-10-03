package service

// kepengurusan_service.go menyusun layanan kepengurusan dari tiga service
// fokus (Jabatan, SK, Pengurus) yang berbagi dependensi via `kepengurusanBase`.
// Handler cukup bergantung pada agregat `KepengurusanService`.

import (
	"context"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/kipan-indonesia/sim-kipan-core/config"
	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
)

// KepengurusanService (agregat) menggabungkan tiga service fokus:
// JabatanService, SKService, dan PengurusService.
type KepengurusanService interface {
	JabatanService
	SKService
	PengurusService
}

// SKDetail detail SK beserta susunan pengurusnya.
type SKDetail struct {
	SK       *domain.SuratKeputusan  `json:"sk"`
	Pengurus []domain.PengurusDetail `json:"pengurus"`
}

// KepengurusanDeps dependensi service kepengurusan (pola deps, R1).
type KepengurusanDeps struct {
	JabatanRepo  repository.JabatanRepository
	SKRepo       repository.SKRepository
	PengurusRepo repository.PengurusRepository
	AnggotaRepo  repository.AnggotaRepository
	UserRepo     repository.UserRepository
	AuditRepo    repository.AuditLogRepository
	WilayahRepo  repository.WilayahRepository
	OutboxRepo   repository.EmailOutboxRepository
}

// kepengurusanBase menampung dependensi bersama + helper lintas sub-service
// (audit & email pengangkatan). Sub-service menempel (embed) base ini.
type kepengurusanBase struct {
	cfg         *config.Config
	jabatanRepo repository.JabatanRepository
	skRepo      repository.SKRepository
	pengurus    repository.PengurusRepository
	anggotaRepo repository.AnggotaRepository
	userRepo    repository.UserRepository
	auditRepo   repository.AuditLogRepository
	wilayahRepo repository.WilayahRepository
	outboxRepo  repository.EmailOutboxRepository
}

// audit mencatat jejak audit aksi kepengurusan (best-effort).
func (b *kepengurusanBase) audit(ctx context.Context, audit domain.AuditContext, actor domain.ActorContext, entity, entityID, action string, metadata *string) {
	id := actor.UserID
	writeAudit(ctx, b.auditRepo, audit, &id, actor.Name, string(actor.Role), entity, entityID, action, metadata)
}

// enqueueEmail menulis satu baris antrian email kepengurusan (best-effort,
// non-fatal) agar pengiriman konsisten lewat worker outbox. Cakupan wilayah
// diambil dari SK (bila ada) untuk keperluan monitoring admin.
func (b *kepengurusanBase) enqueueEmail(ctx context.Context, jenis domain.EmailOutboxKind, sk *domain.SuratKeputusan, userID *string, toEmail string, c EmailContent) {
	if b.outboxRepo == nil || strings.TrimSpace(toEmail) == "" {
		return
	}
	html := c.HTMLBody
	entry := &domain.EmailOutbox{
		Jenis: jenis, UserID: userID, ToEmail: toEmail,
		Subject: c.Subject, TextBody: c.TextBody, HTMLBody: &html,
	}
	if sk != nil {
		entry.ProvinsiID = sk.ProvinsiID
		entry.KabupatenID = sk.KabupatenID
	}
	enqCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := b.outboxRepo.Enqueue(enqCtx, entry); err != nil {
		log.Warn().Err(err).Str("jenis", string(jenis)).Msg("Gagal enqueue email outbox kepengurusan")
	}
}

// kepengurusanAggregate menyalurkan pemanggilan method ke sub-service terkait
// dengan meng-embed ketiga interface-nya.
type kepengurusanAggregate struct {
	JabatanService
	SKService
	PengurusService
}

// NewKepengurusanService membangun base bersama + tiga sub-service fokus.
func NewKepengurusanService(cfg *config.Config, deps KepengurusanDeps) KepengurusanService {
	base := &kepengurusanBase{
		cfg: cfg, jabatanRepo: deps.JabatanRepo, skRepo: deps.SKRepo,
		pengurus: deps.PengurusRepo, anggotaRepo: deps.AnggotaRepo,
		userRepo: deps.UserRepo, auditRepo: deps.AuditRepo,
		wilayahRepo: deps.WilayahRepo, outboxRepo: deps.OutboxRepo,
	}
	return &kepengurusanAggregate{
		JabatanService:  &jabatanSvc{kepengurusanBase: base},
		SKService:       &skSvc{kepengurusanBase: base},
		PengurusService: &pengurusSvc{kepengurusanBase: base},
	}
}

// isNasionalOrSuper true untuk peran Nasional/Super.
func isNasionalOrSuper(role domain.Role) bool {
	return role == domain.RoleSuperAdmin || role == domain.RoleAdminNasional
}

// derefInt mengembalikan nilai dari pointer int (0 bila nil).
func derefInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

// levelAuthorityMatch menentukan kecocokan peran aktor dengan LEVEL SK
// (tanpa Super): KABUPATEN -> Kabupaten sekab; PROVINSI -> Provinsi seprov;
// NASIONAL -> Nasional.
func levelAuthorityMatch(actor domain.ActorContext, sk *domain.SuratKeputusan) bool {
	if sk == nil {
		return false
	}
	switch sk.Level {
	case domain.LevelKabupaten:
		return actor.Role == domain.RoleAdminKabupaten &&
			actor.KabupatenID != nil && sk.KabupatenID != nil &&
			*actor.KabupatenID == *sk.KabupatenID
	case domain.LevelProvinsi:
		return actor.Role == domain.RoleAdminProvinsi &&
			actor.ProvinsiID != nil && sk.ProvinsiID != nil &&
			*actor.ProvinsiID == *sk.ProvinsiID
	case domain.LevelNasional:
		return actor.Role == domain.RoleAdminNasional
	}
	return false
}

// canAjukanSK: wewenang mengajukan SK (DRAFT -> tahap berikut) menurut LEVEL
// SK. KABUPATEN -> Admin Kabupaten sekab; PROVINSI -> Admin Provinsi seprov;
// NASIONAL -> Nasional; Super oversight semua.
func canAjukanSK(actor domain.ActorContext, sk *domain.SuratKeputusan) bool {
	if actor.Role == domain.RoleSuperAdmin {
		return true
	}
	return levelAuthorityMatch(actor, sk)
}

// canManageSK menentukan wewenang mengelola PENGURUS pada SK berdasarkan LEVEL
// SK (Opsi A): KABUPATEN -> Admin Kabupaten sekab ATAU Admin Provinsi seprov;
// PROVINSI -> Admin Provinsi seprov; NASIONAL -> Nasional; Super oversight semua.
func canManageSK(actor domain.ActorContext, sk *domain.SuratKeputusan) bool {
	if sk == nil {
		return false
	}
	if actor.Role == domain.RoleSuperAdmin {
		return true
	}
	switch sk.Level {
	case domain.LevelKabupaten:
		if actor.Role == domain.RoleAdminKabupaten &&
			actor.KabupatenID != nil && sk.KabupatenID != nil &&
			*actor.KabupatenID == *sk.KabupatenID {
			return true
		}
		return actor.Role == domain.RoleAdminProvinsi &&
			actor.ProvinsiID != nil && sk.ProvinsiID != nil &&
			*actor.ProvinsiID == *sk.ProvinsiID
	case domain.LevelProvinsi:
		return actor.Role == domain.RoleAdminProvinsi &&
			actor.ProvinsiID != nil && sk.ProvinsiID != nil &&
			*actor.ProvinsiID == *sk.ProvinsiID
	case domain.LevelNasional:
		return actor.Role == domain.RoleAdminNasional
	}
	return false
}

// anggotaInSKScope memastikan anggota berada dalam cakupan wilayah SK.
func anggotaInSKScope(member *domain.Anggota, sk *domain.SuratKeputusan) bool {
	switch sk.Level {
	case domain.LevelKabupaten:
		return sk.KabupatenID != nil && member.KabupatenID == *sk.KabupatenID
	case domain.LevelProvinsi:
		return sk.ProvinsiID != nil && member.ProvinsiID == *sk.ProvinsiID
	case domain.LevelNasional:
		return true
	}
	return false
}
