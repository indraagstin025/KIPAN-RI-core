package service

// jabatan_service.go — service fokus: master jabatan (Fase A4, SRP; TDD D14:
// tanpa level, is_ketua_umum, semua admin boleh menambah).

import (
	"context"
	"strconv"
	"strings"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
)

// JabatanService mengelola master jabatan struktural.
// Create: semua admin. Update: Super/Nasional (keputusan TDD D14).
type JabatanService interface {
	ListJabatan(ctx context.Context, includeInactive bool) ([]domain.Jabatan, error)
	CreateJabatan(ctx context.Context, in domain.JabatanRequest, actor domain.ActorContext, audit domain.AuditContext) (*domain.Jabatan, error)
	UpdateJabatan(ctx context.Context, id int, in domain.JabatanRequest, actor domain.ActorContext, audit domain.AuditContext) (*domain.Jabatan, error)
}

type jabatanSvc struct{ *kepengurusanBase }

// validateJabatan memvalidasi payload master jabatan (nama, urutan).
func validateJabatan(in domain.JabatanRequest) error {
	nama := strings.TrimSpace(in.Nama)
	if len(nama) < 2 || len(nama) > 100 {
		return domain.NewValidationError("Nama jabatan wajib 2-100 karakter")
	}
	if in.Urutan < 0 || in.Urutan > 9999 {
		return domain.NewValidationError("Urutan jabatan tidak valid")
	}
	return nil
}

// ListJabatan mengembalikan master jabatan (opsional termasuk nonaktif).
func (s *jabatanSvc) ListJabatan(ctx context.Context, includeInactive bool) ([]domain.Jabatan, error) {
	if s.jabatanRepo == nil {
		return nil, unavailable("jabatan")
	}
	return s.jabatanRepo.List(ctx, includeInactive)
}

// CreateJabatan membuat jabatan baru. Semua admin boleh menambah (kab/prov
// bisa butuh jabatan sendiri); nama unik dijaga constraint DB.
func (s *jabatanSvc) CreateJabatan(ctx context.Context, in domain.JabatanRequest, actor domain.ActorContext, audit domain.AuditContext) (*domain.Jabatan, error) {
	switch actor.Role {
	case domain.RoleSuperAdmin, domain.RoleAdminNasional, domain.RoleAdminProvinsi, domain.RoleAdminKabupaten:
	default:
		return nil, domain.NewForbiddenError("Hanya admin yang dapat menambah jabatan")
	}
	if err := validateJabatan(in); err != nil {
		return nil, err
	}
	if s.jabatanRepo == nil {
		return nil, unavailable("jabatan")
	}
	out, err := s.jabatanRepo.Create(ctx, in)
	if err != nil {
		return nil, err
	}
	meta := `{"event":"jabatan_create"}`
	s.audit(ctx, audit, actor, "jabatan", strconv.Itoa(out.ID), "CREATE", &meta)
	return out, nil
}

// UpdateJabatan memperbarui jabatan (ubah nama/urutan/penanda). Super/Nasional.
func (s *jabatanSvc) UpdateJabatan(ctx context.Context, id int, in domain.JabatanRequest, actor domain.ActorContext, audit domain.AuditContext) (*domain.Jabatan, error) {
	if !isNasionalOrSuper(actor.Role) {
		return nil, domain.NewForbiddenError("Ubah jabatan hanya untuk Nasional/Super Admin")
	}
	if id <= 0 {
		return nil, domain.NewValidationError("ID jabatan tidak valid")
	}
	if err := validateJabatan(in); err != nil {
		return nil, err
	}
	if s.jabatanRepo == nil {
		return nil, unavailable("jabatan")
	}
	out, err := s.jabatanRepo.Update(ctx, id, in)
	if err != nil {
		return nil, err
	}
	meta := `{"event":"jabatan_update"}`
	s.audit(ctx, audit, actor, "jabatan", strconv.Itoa(id), "UPDATE", &meta)
	return out, nil
}
