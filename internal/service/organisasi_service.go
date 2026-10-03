package service

// organisasi_service.go — profil organisasi (TDD §2.1 halaman publik):
// baca publik + sunting Super Admin. Konten dibersihkan dari karakter `<`/`>`
// untuk mencegah stored-XSS pada halaman publik.

import (
	"context"
	"strings"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
)

// OrganisasiService melayani profil organisasi.
type OrganisasiService interface {
	Get(ctx context.Context) (*domain.OrganisasiProfile, error)
	Update(ctx context.Context, actor domain.ActorContext, audit domain.AuditContext, in domain.OrganisasiUpdateRequest) (*domain.OrganisasiProfile, error)
}

type organisasiService struct {
	repo      repository.OrganisasiRepository
	auditRepo repository.AuditLogRepository
}

// NewOrganisasiService membangun service profil organisasi.
func NewOrganisasiService(repo repository.OrganisasiRepository, auditRepo repository.AuditLogRepository) OrganisasiService {
	return &organisasiService{repo: repo, auditRepo: auditRepo}
}

// Get mengembalikan profil organisasi (publik).
func (s *organisasiService) Get(ctx context.Context) (*domain.OrganisasiProfile, error) {
	if s.repo == nil {
		return nil, unavailable("profil organisasi")
	}
	return s.repo.Get(ctx)
}

// Update menyimpan profil organisasi (Super Admin) + audit.
func (s *organisasiService) Update(ctx context.Context, actor domain.ActorContext, audit domain.AuditContext, in domain.OrganisasiUpdateRequest) (*domain.OrganisasiProfile, error) {
	if actor.Role != domain.RoleSuperAdmin {
		return nil, domain.NewForbiddenError("Profil Organisasi hanya untuk Super Admin")
	}
	if s.repo == nil {
		return nil, unavailable("profil organisasi")
	}
	clean := func(v string, max int) (string, bool) {
		v = strings.TrimSpace(v)
		if len(v) > max || strings.ContainsAny(v, "<>") {
			return "", false
		}
		return v, true
	}
	fields := []struct {
		val *string
		raw string
		max int
	}{
		{&in.Nama, in.Nama, 150}, {&in.Singkatan, in.Singkatan, 50},
		{&in.Deskripsi, in.Deskripsi, 4000}, {&in.Visi, in.Visi, 2000}, {&in.Misi, in.Misi, 4000},
		{&in.Alamat, in.Alamat, 500}, {&in.Email, in.Email, 150}, {&in.Telepon, in.Telepon, 50},
		{&in.Whatsapp, in.Whatsapp, 25}, {&in.Website, in.Website, 255}, {&in.Instagram, in.Instagram, 255},
		{&in.Facebook, in.Facebook, 255}, {&in.Youtube, in.Youtube, 255}, {&in.Tiktok, in.Tiktok, 255},
		{&in.LogoURL, in.LogoURL, 500},
	}
	for _, f := range fields {
		v, ok := clean(f.raw, f.max)
		if !ok {
			return nil, domain.NewValidationError("Teks profil tidak valid atau terlalu panjang")
		}
		*f.val = v
	}
	if in.Nama == "" {
		return nil, domain.NewValidationError("Nama organisasi wajib diisi")
	}

	out, err := s.repo.Update(ctx, in, actor.UserID)
	if err != nil {
		return nil, err
	}
	meta := `{"event":"organisasi_update"}`
	writeAudit(ctx, s.auditRepo, audit, &actor.UserID, actor.Name, string(actor.Role),
		"organisasi_profile", "1", "UPDATE", &meta)
	return out, nil
}
