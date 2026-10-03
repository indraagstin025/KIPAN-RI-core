package service

// jabatan_service.go — service fokus: master jabatan (Fase A4, SRP).

import (
	"context"
	"strconv"
	"strings"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
)

// JabatanService mengelola master jabatan struktural (Super/Nasional).
type JabatanService interface {
	ListJabatan(ctx context.Context, includeInactive bool, level string) ([]domain.Jabatan, error)
	CreateJabatan(ctx context.Context, in domain.JabatanRequest, actor domain.ActorContext, audit domain.AuditContext) (*domain.Jabatan, error)
	UpdateJabatan(ctx context.Context, id int, in domain.JabatanRequest, actor domain.ActorContext, audit domain.AuditContext) (*domain.Jabatan, error)
}

type jabatanSvc struct{ *kepengurusanBase }

// validateJabatan memvalidasi payload master jabatan (nama, level, urutan).
func validateJabatan(in domain.JabatanRequest) error {
	nama := strings.TrimSpace(in.Nama)
	if len(nama) < 2 || len(nama) > 100 {
		return domain.NewValidationError("Nama jabatan wajib 2-100 karakter")
	}
	switch domain.TingkatWilayah(strings.ToUpper(strings.TrimSpace(in.Level))) {
	case domain.LevelNasional, domain.LevelProvinsi, domain.LevelKabupaten:
	default:
		return domain.NewValidationError("Level jabatan harus NASIONAL/PROVINSI/KABUPATEN")
	}
	if in.Urutan < 0 || in.Urutan > 9999 {
		return domain.NewValidationError("Urutan jabatan tidak valid")
	}
	return nil
}

// ListJabatan mengembalikan master jabatan (opsional filter level).
func (s *jabatanSvc) ListJabatan(ctx context.Context, includeInactive bool, level string) ([]domain.Jabatan, error) {
	if s.jabatanRepo == nil {
		return nil, unavailable("jabatan")
	}
	lvl := strings.ToUpper(strings.TrimSpace(level))
	if lvl != "" {
		switch domain.TingkatWilayah(lvl) {
		case domain.LevelNasional, domain.LevelProvinsi, domain.LevelKabupaten:
		default:
			return nil, domain.NewValidationError("Level jabatan tidak valid")
		}
	}
	return s.jabatanRepo.List(ctx, includeInactive, lvl)
}

// CreateJabatan membuat jabatan baru (hanya Nasional/Super).
func (s *jabatanSvc) CreateJabatan(ctx context.Context, in domain.JabatanRequest, actor domain.ActorContext, audit domain.AuditContext) (*domain.Jabatan, error) {
	if !isNasionalOrSuper(actor.Role) {
		return nil, domain.NewForbiddenError("Kelola jabatan hanya untuk Nasional/Super Admin")
	}
	if err := validateJabatan(in); err != nil {
		return nil, err
	}
	if s.jabatanRepo == nil {
		return nil, unavailable("jabatan")
	}
	in.Level = strings.ToUpper(strings.TrimSpace(in.Level))
	out, err := s.jabatanRepo.Create(ctx, in)
	if err != nil {
		return nil, err
	}
	meta := `{"event":"jabatan_create"}`
	s.audit(ctx, audit, actor, "jabatan", strconv.Itoa(out.ID), "CREATE", &meta)
	return out, nil
}

// UpdateJabatan memperbarui jabatan; level yang sedang dipakai pengurus tak
// boleh diubah (menjaga konsistensi level SK).
func (s *jabatanSvc) UpdateJabatan(ctx context.Context, id int, in domain.JabatanRequest, actor domain.ActorContext, audit domain.AuditContext) (*domain.Jabatan, error) {
	if !isNasionalOrSuper(actor.Role) {
		return nil, domain.NewForbiddenError("Kelola jabatan hanya untuk Nasional/Super Admin")
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
	in.Level = strings.ToUpper(strings.TrimSpace(in.Level))
	// Cegah perubahan LEVEL jabatan yang sedang dipakai pengurus, agar
	// jabatan.level tidak menyimpang dari level SK pengurus terkait.
	existing, err := s.jabatanRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if domain.TingkatWilayah(in.Level) != existing.Level {
		n, err := s.jabatanRepo.CountPengurus(ctx, id)
		if err != nil {
			return nil, err
		}
		if n > 0 {
			return nil, domain.NewConflictError("Level jabatan tidak dapat diubah karena masih dipakai pengurus")
		}
	}
	out, err := s.jabatanRepo.Update(ctx, id, in)
	if err != nil {
		return nil, err
	}
	meta := `{"event":"jabatan_update"}`
	s.audit(ctx, audit, actor, "jabatan", strconv.Itoa(id), "UPDATE", &meta)
	return out, nil
}
