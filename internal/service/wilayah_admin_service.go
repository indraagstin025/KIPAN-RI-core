package service

import (
	"context"
	"strconv"
	"strings"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
)

// WilayahCards ringkasan kartu halaman Master Wilayah.
type WilayahCards struct {
	TotalProvinsi  int `json:"total_provinsi"`
	TotalKabupaten int `json:"total_kabupaten"`
	TotalPengurus  int `json:"total_pengurus"`
}

// WilayahAdminService melayani master wilayah untuk Super/Nasional.
type WilayahAdminService interface {
	Cards(ctx context.Context, actor domain.ActorContext) (*WilayahCards, error)
	List(ctx context.Context, actor domain.ActorContext, tipe, search, status string, provinsiID *int) ([]domain.WilayahAdminItem, error)
	Detail(ctx context.Context, actor domain.ActorContext, tipe string, id int) (*domain.WilayahDetailAdmin, error)
	Pengurus(ctx context.Context, actor domain.ActorContext, tipe string, id int, all bool) ([]domain.PengurusDetail, error)
	SetStatus(ctx context.Context, actor domain.ActorContext, audit domain.AuditContext, tipe string, id int, active bool) error
	Add(ctx context.Context, actor domain.ActorContext, audit domain.AuditContext, tipe string, provinsiID, kabupatenID int) error
}

type wilayahAdminSvc struct {
	repo      repository.WilayahAdminRepository
	auditRepo repository.AuditLogRepository
}

func NewWilayahAdminService(repo repository.WilayahAdminRepository, auditRepo repository.AuditLogRepository) WilayahAdminService {
	return &wilayahAdminSvc{repo: repo, auditRepo: auditRepo}
}

func isWilayahAdmin(role domain.Role) bool {
	return role == domain.RoleSuperAdmin || role == domain.RoleAdminNasional
}

// normTipe menormalkan jenis wilayah; "" bila tak dikenal.
func normTipe(tipe string) string {
	switch strings.ToLower(strings.TrimSpace(tipe)) {
	case "provinsi":
		return "provinsi"
	case "kabupaten":
		return "kabupaten"
	}
	return ""
}

func (s *wilayahAdminSvc) guard(actor domain.ActorContext) error {
	if !isWilayahAdmin(actor.Role) {
		return domain.NewForbiddenError("Master Wilayah hanya untuk Super/Nasional Admin")
	}
	if s.repo == nil {
		return unavailable("wilayah")
	}
	return nil
}

func (s *wilayahAdminSvc) Cards(ctx context.Context, actor domain.ActorContext) (*WilayahCards, error) {
	if err := s.guard(actor); err != nil {
		return nil, err
	}
	p, err := s.repo.CountProvinsi(ctx)
	if err != nil {
		return nil, err
	}
	k, err := s.repo.CountKabupaten(ctx, nil)
	if err != nil {
		return nil, err
	}
	g, err := s.repo.CountPengurusAktif(ctx)
	if err != nil {
		return nil, err
	}
	return &WilayahCards{TotalProvinsi: p, TotalKabupaten: k, TotalPengurus: g}, nil
}

func (s *wilayahAdminSvc) List(ctx context.Context, actor domain.ActorContext, tipe, search, status string, provinsiID *int) ([]domain.WilayahAdminItem, error) {
	if err := s.guard(actor); err != nil {
		return nil, err
	}
	t := normTipe(tipe)
	if t == "" {
		return nil, domain.NewValidationError("Tipe wilayah harus provinsi/kabupaten")
	}
	if t == "provinsi" {
		return s.repo.ListProvinsiAdmin(ctx, search, status)
	}
	return s.repo.ListKabupatenAdmin(ctx, provinsiID, search, status)
}

// scopeWilayah mengembalikan (level, provinsiID, kabupatenID) untuk detail/pengurus.
func (s *wilayahAdminSvc) scopeWilayah(ctx context.Context, tipe string, id int) (string, *domain.WilayahDetailAdmin, *int, *int, error) {
	det := &domain.WilayahDetailAdmin{Type: tipe, ID: id}
	switch tipe {
	case "provinsi":
		p, err := s.repo.GetProvinsi(ctx, id)
		if err != nil {
			return "", nil, nil, nil, err
		}
		det.Kode, det.Nama, det.IsActive = p.Kode, p.Nama, p.IsActive
		n, err := s.repo.CountKabupaten(ctx, &p.ID)
		if err != nil {
			return "", nil, nil, nil, err
		}
		det.Statistik.TotalKabupaten = n
		provID := p.ID
		return "PROVINSI", det, &provID, nil, nil
	case "kabupaten":
		k, err := s.repo.GetKabupaten(ctx, id)
		if err != nil {
			return "", nil, nil, nil, err
		}
		det.Kode, det.Nama, det.IsActive = k.Kode, k.Nama, k.IsActive
		provID := k.ProvinsiID
		kabID := k.ID
		det.ProvinsiID = &provID
		if p, err := s.repo.GetProvinsi(ctx, k.ProvinsiID); err == nil {
			det.ProvinsiNama = &p.Nama
		}
		return "KABUPATEN", det, &provID, &kabID, nil
	}
	return "", nil, nil, nil, domain.NewValidationError("Tipe wilayah harus provinsi/kabupaten")
}

func (s *wilayahAdminSvc) Detail(ctx context.Context, actor domain.ActorContext, tipe string, id int) (*domain.WilayahDetailAdmin, error) {
	if err := s.guard(actor); err != nil {
		return nil, err
	}
	t := normTipe(tipe)
	if t == "" {
		return nil, domain.NewValidationError("Tipe wilayah harus provinsi/kabupaten")
	}
	if id <= 0 {
		return nil, domain.NewValidationError("ID wilayah tidak valid")
	}
	level, det, prov, kab, err := s.scopeWilayah(ctx, t, id)
	if err != nil {
		return nil, err
	}
	total, aktif, err := s.repo.StatsPengurusWilayah(ctx, level, prov, kab)
	if err != nil {
		return nil, err
	}
	det.Statistik.TotalPengurus = total
	det.Statistik.PengurusAktif = aktif
	tren, err := s.repo.TrenPengurusWilayah(ctx, level, prov, kab)
	if err != nil {
		return nil, err
	}
	det.Statistik.Tren = tren
	list, err := s.repo.ListPengurusWilayah(ctx, level, prov, kab, true)
	if err != nil {
		return nil, err
	}
	det.Pengurus = list
	act, err := s.repo.ListActivityWilayah(ctx, "wilayah_"+t, strconv.Itoa(id), 20)
	if err != nil {
		return nil, err
	}
	det.Activity = act
	return det, nil
}

func (s *wilayahAdminSvc) Pengurus(ctx context.Context, actor domain.ActorContext, tipe string, id int, all bool) ([]domain.PengurusDetail, error) {
	if err := s.guard(actor); err != nil {
		return nil, err
	}
	t := normTipe(tipe)
	if t == "" {
		return nil, domain.NewValidationError("Tipe wilayah harus provinsi/kabupaten")
	}
	if id <= 0 {
		return nil, domain.NewValidationError("ID wilayah tidak valid")
	}
	level, _, prov, kab, err := s.scopeWilayah(ctx, t, id)
	if err != nil {
		return nil, err
	}
	return s.repo.ListPengurusWilayah(ctx, level, prov, kab, all)
}

func (s *wilayahAdminSvc) SetStatus(ctx context.Context, actor domain.ActorContext, audit domain.AuditContext, tipe string, id int, active bool) error {
	if err := s.guard(actor); err != nil {
		return err
	}
	t := normTipe(tipe)
	if t == "" {
		return domain.NewValidationError("Tipe wilayah harus provinsi/kabupaten")
	}
	if id <= 0 {
		return domain.NewValidationError("ID wilayah tidak valid")
	}
	var err error
	if t == "provinsi" {
		err = s.repo.SetProvinsiActive(ctx, id, active)
	} else {
		err = s.repo.SetKabupatenActive(ctx, id, active)
	}
	if err != nil {
		return err
	}
	meta := `{"event":"wilayah_status","is_active":` + strconv.FormatBool(active) + `}`
	s.audit(ctx, audit, actor, "wilayah_"+t, strconv.Itoa(id), "UPDATE", &meta)
	return nil
}

func (s *wilayahAdminSvc) Add(ctx context.Context, actor domain.ActorContext, audit domain.AuditContext, tipe string, provinsiID, kabupatenID int) error {
	if err := s.guard(actor); err != nil {
		return err
	}
	t := normTipe(tipe)
	if t == "" {
		return domain.NewValidationError("Tipe wilayah harus provinsi/kabupaten")
	}
	var err error
	entityID := strconv.Itoa(provinsiID)
	if t == "provinsi" {
		if provinsiID <= 0 {
			return domain.NewValidationError("Provinsi wajib dipilih")
		}
		err = s.repo.EnsureProvinsiActive(ctx, provinsiID)
	} else {
		if provinsiID <= 0 || kabupatenID <= 0 {
			return domain.NewValidationError("Provinsi & Kabupaten/Kota wajib dipilih")
		}
		err = s.repo.EnsureKabupatenActive(ctx, provinsiID, kabupatenID)
		entityID = strconv.Itoa(kabupatenID)
	}
	if err != nil {
		return err
	}
	meta := `{"event":"wilayah_add","provinsi_id":` + strconv.Itoa(provinsiID) + `,"kabupaten_id":` + strconv.Itoa(kabupatenID) + `}`
	s.audit(ctx, audit, actor, "wilayah_"+t, entityID, "CREATE", &meta)
	return nil
}

func (s *wilayahAdminSvc) audit(ctx context.Context, tr domain.AuditContext, actor domain.ActorContext, entity, entityID, action string, metadata *string) {
	id := actor.UserID
	writeAudit(ctx, s.auditRepo, tr, &id, actor.Name, string(actor.Role), entity, entityID, action, metadata)
}
