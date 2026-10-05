package service

// sk_service.go — service fokus: Surat Keputusan (Fase A4, SRP).

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
	"github.com/kipan-indonesia/sim-kipan-core/internal/service/svcutil"
)

// SKService mengelola dokumen Surat Keputusan (buat, daftar, detail, rantai
// persetujuan, dan status).
type SKService interface {
	CreateSK(ctx context.Context, in domain.SKCreateRequest, actor domain.ActorContext, audit domain.AuditContext) (*domain.SuratKeputusan, error)
	ListSK(ctx context.Context, actor domain.ActorContext, level, status, approval, search string, withTotal bool, page, limit int) ([]domain.SKListItem, int, error)
	GetSK(ctx context.Context, id int, actor domain.ActorContext) (*SKDetail, error)
	ApproveSK(ctx context.Context, id int, action domain.SKApprovalAction, catatan string, actor domain.ActorContext, audit domain.AuditContext) error
	SetSKStatus(ctx context.Context, id int, status domain.SKStatus, pengurusStatus domain.PengurusStatus, keterangan string, actor domain.ActorContext, audit domain.AuditContext) error
}

type skSvc struct{ *kepengurusanBase }

// CreateSK menyimpan SK baru sebagai DRAFT (level/wilayah dari role aktor).
func (s *skSvc) CreateSK(ctx context.Context, in domain.SKCreateRequest, actor domain.ActorContext, audit domain.AuditContext) (*domain.SuratKeputusan, error) {
	nomor := strings.TrimSpace(in.NomorSK)
	if nomor == "" || len(nomor) > 100 {
		return nil, domain.NewValidationError("Nomor SK wajib diisi (maks 100 karakter)")
	}
	judul := strings.TrimSpace(in.Judul)
	if judul == "" || len(judul) > 255 {
		return nil, domain.NewValidationError("Judul SK wajib diisi (maks 255 karakter)")
	}
	if in.TanggalTerbit.IsZero() {
		return nil, domain.NewValidationError("Tanggal terbit SK wajib diisi")
	}
	if in.TanggalBerakhir.IsZero() {
		return nil, domain.NewValidationError("Tanggal berakhir SK wajib diisi")
	}
	if !in.TanggalBerakhir.After(in.TanggalTerbit) {
		return nil, domain.NewValidationError("Tanggal berakhir harus setelah tanggal terbit")
	}
	fileKey := strings.TrimSpace(in.FileSKKey)
	if fileKey == "" || len(fileKey) > 255 {
		return nil, domain.NewValidationError("File SK wajib diunggah")
	}

	level, provID, kabID, err := s.resolveSKScope(ctx, in, actor)
	if err != nil {
		return nil, err
	}

	if s.skRepo == nil {
		return nil, svcutil.Unavailable("surat keputusan")
	}
	berakhir := in.TanggalBerakhir
	// Semua SK disimpan sebagai DRAFT; publikasi final lewat aksi AJUKAN.
	sk := &domain.SuratKeputusan{
		NomorSK:         nomor,
		Judul:           judul,
		Level:           level,
		ProvinsiID:      provID,
		KabupatenID:     kabID,
		TanggalTerbit:   in.TanggalTerbit,
		TanggalBerakhir: &berakhir,
		FileSKKey:       fileKey,
		Status:          domain.SKStatusAktif,
		ApprovalStatus:  domain.SKApprovalStatusDraft,
		CreatedBy:       &actor.UserID,
	}

	out, err := s.skRepo.Create(ctx, sk)
	if err != nil {
		return nil, err
	}
	meta := `{"event":"sk_create","nomor_sk":"` + out.NomorSK + `","level":"` + string(out.Level) + `"}`
	s.audit(ctx, audit, actor, "surat_keputusan", strconv.Itoa(out.ID), "CREATE", &meta)
	return out, nil
}

// resolveSKScope menentukan level & wilayah SK dari ROLE aktor (bukan input
// klien). Super/Nasional bebas memilih level + wilayah (divalidasi ke master);
// Provinsi/Kabupaten dipaksa ke wilayahnya sendiri.
func (s *skSvc) resolveSKScope(ctx context.Context, in domain.SKCreateRequest, actor domain.ActorContext) (domain.TingkatWilayah, *int, *int, error) {
	switch actor.Role {
	case domain.RoleAdminKabupaten:
		if actor.ProvinsiID == nil || actor.KabupatenID == nil {
			return "", nil, nil, domain.NewForbiddenError("Akun Admin Kabupaten tidak memiliki wilayah")
		}
		return domain.LevelKabupaten, actor.ProvinsiID, actor.KabupatenID, nil
	case domain.RoleAdminProvinsi:
		if actor.ProvinsiID == nil {
			return "", nil, nil, domain.NewForbiddenError("Akun Admin Provinsi tidak memiliki wilayah")
		}
		return domain.LevelProvinsi, actor.ProvinsiID, nil, nil
	case domain.RoleAdminNasional, domain.RoleSuperAdmin:
		if s.wilayahRepo == nil {
			return "", nil, nil, svcutil.Unavailable("wilayah")
		}
		switch domain.TingkatWilayah(strings.ToUpper(strings.TrimSpace(in.Level))) {
		case domain.LevelNasional:
			return domain.LevelNasional, nil, nil, nil
		case domain.LevelProvinsi:
			if in.ProvinsiID == nil {
				return "", nil, nil, domain.NewValidationError("Provinsi wajib dipilih untuk SK tingkat Provinsi")
			}
			ok, err := s.wilayahRepo.ExistsProvinsi(ctx, *in.ProvinsiID)
			if err != nil {
				return "", nil, nil, err
			}
			if !ok {
				return "", nil, nil, domain.NewValidationError("Provinsi tidak valid / tidak aktif")
			}
			return domain.LevelProvinsi, in.ProvinsiID, nil, nil
		case domain.LevelKabupaten:
			if in.ProvinsiID == nil || in.KabupatenID == nil {
				return "", nil, nil, domain.NewValidationError("Provinsi & Kabupaten/Kota wajib dipilih untuk SK tingkat Kabupaten")
			}
			ok, err := s.wilayahRepo.KabupatenInProvinsi(ctx, *in.KabupatenID, *in.ProvinsiID)
			if err != nil {
				return "", nil, nil, err
			}
			if !ok {
				return "", nil, nil, domain.NewValidationError("Kabupaten/Kota tidak valid untuk provinsi tersebut")
			}
			return domain.LevelKabupaten, in.ProvinsiID, in.KabupatenID, nil
		default:
			return "", nil, nil, domain.NewValidationError("Level SK harus NASIONAL/PROVINSI/KABUPATEN")
		}
	default:
		return "", nil, nil, domain.NewForbiddenError("Pembuatan SK hanya untuk admin")
	}
}

// ListSK mengembalikan daftar SK ter-scope (bounded, opsional with_total).
func (s *skSvc) ListSK(ctx context.Context, actor domain.ActorContext, level, status, approval, search string, withTotal bool, page, limit int) ([]domain.SKListItem, int, error) {
	if s.skRepo == nil {
		return nil, 0, svcutil.Unavailable("surat keputusan")
	}
	prov, kab, err := actor.Scope()
	if err != nil {
		return nil, 0, err
	}
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}
	return s.skRepo.List(ctx, repository.SKFilter{
		Level: level, ProvinsiID: prov, KabupatenID: kab,
		Status: status, ApprovalStatus: approval, Search: search,
		WithTotal: withTotal, Limit: limit, Offset: (page - 1) * limit,
	})
}

// GetSK mengambil detail SK + susunan pengurus (menolak lintas wilayah).
func (s *skSvc) GetSK(ctx context.Context, id int, actor domain.ActorContext) (*SKDetail, error) {
	if id <= 0 {
		return nil, domain.NewValidationError("ID SK tidak valid")
	}
	if s.skRepo == nil {
		return nil, svcutil.Unavailable("surat keputusan")
	}
	sk, err := s.skRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !actor.CanAccessWilayah(derefInt(sk.ProvinsiID), derefInt(sk.KabupatenID)) {
		return nil, domain.NewForbiddenError("SK di luar wilayah kerja Anda")
	}
	list, err := s.pengurus.ListBySK(ctx, id)
	if err != nil {
		return nil, err
	}
	return &SKDetail{SK: sk, Pengurus: list}, nil
}

// ApproveSK menjalankan rantai persetujuan SK (AJUKAN/TERUSKAN/SAHKAN/TOLAK).
func (s *skSvc) ApproveSK(ctx context.Context, id int, action domain.SKApprovalAction, catatan string, actor domain.ActorContext, audit domain.AuditContext) error {
	if id <= 0 {
		return domain.NewValidationError("ID SK tidak valid")
	}
	if s.skRepo == nil {
		return svcutil.Unavailable("surat keputusan")
	}
	sk, err := s.skRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	var note *string

	// 1) Otorisasi + validasi khas aksi (sebelum menyentuh transisi).
	switch action {
	case domain.SKActionAjukan:
		if !canAjukanSK(actor, sk) {
			return domain.NewForbiddenError("Anda tidak berwenang mengajukan SK ini")
		}
	case domain.SKActionTeruskan:
		if actor.Role != domain.RoleAdminProvinsi || actor.ProvinsiID == nil ||
			sk.ProvinsiID == nil || *actor.ProvinsiID != *sk.ProvinsiID {
			return domain.NewForbiddenError("Penerusan SK hanya oleh Admin Provinsi wilayah SK")
		}
	case domain.SKActionSahkan:
		if !isNasionalOrSuper(actor.Role) {
			return domain.NewForbiddenError("Pengesahan SK hanya oleh Nasional/Super Admin")
		}
	case domain.SKActionTolak:
		if !isNasionalOrSuper(actor.Role) {
			return domain.NewForbiddenError("Penolakan SK hanya oleh Nasional/Super Admin")
		}
		n := strings.TrimSpace(catatan)
		if n == "" {
			return domain.NewValidationError("Catatan wajib diisi untuk menolak SK")
		}
		note = &n
	default:
		return domain.NewValidationError("Aksi persetujuan SK tidak valid")
	}

	// 2) Transisi dari state machine tersurat (domain.sk_transition.go).
	from, to, ok := domain.ResolveSKApprovalTransition(action, sk.Level)
	if !ok {
		return domain.NewValidationError("Level/status SK tidak valid untuk aksi ini")
	}

	// 3) Kolom pengesah diisi saat hasil final DISETUJUI atau saat penolakan.
	var approvedBy *string
	var approvedAt *time.Time
	if to == domain.SKApprovalStatusDisetujui || action == domain.SKActionTolak {
		now := time.Now()
		approvedBy, approvedAt = &actor.UserID, &now
	}

	if sk.ApprovalStatus != from {
		return domain.NewConflictError("Status SK tidak sesuai tahap aksi ini. Muat ulang lalu coba lagi.")
	}
	if to == domain.SKApprovalStatusDisetujui {
		// Finalisasi + Single Active SK rule (atomik: SK lama nonaktif, pengurus demisioner).
		if err := s.skRepo.FinalizeSK(ctx, id, from, approvedBy, approvedAt); err != nil {
			return err
		}
	} else if err := s.skRepo.UpdateApproval(ctx, id, from, to, note, approvedBy, approvedAt); err != nil {
		return err
	}
	auditAction := "APPROVE"
	switch action {
	case domain.SKActionAjukan:
		auditAction = "SUBMIT"
	case domain.SKActionTolak:
		auditAction = "REJECT"
	}
	meta := `{"event":"sk_approval","action":"` + string(action) + `","to":"` + string(to) + `"}`
	s.audit(ctx, audit, actor, "surat_keputusan", strconv.Itoa(id), auditAction, &meta)
	return nil
}

// SetSKStatus mengubah status SK; menonaktifkan SK mendemosi pengurus aktif.
func (s *skSvc) SetSKStatus(ctx context.Context, id int, status domain.SKStatus, pengurusStatus domain.PengurusStatus, keterangan string, actor domain.ActorContext, audit domain.AuditContext) error {
	if id <= 0 {
		return domain.NewValidationError("ID SK tidak valid")
	}
	if status != domain.SKStatusAktif && status != domain.SKStatusTidakAktif && status != domain.SKStatusDigantikan {
		return domain.NewValidationError("Status SK tidak valid")
	}
	// Saat menonaktifkan SK: wajib keterangan + status pengurus terpilih.
	if status == domain.SKStatusTidakAktif {
		switch pengurusStatus {
		case "", domain.PengurusStatusDemisioner, domain.PengurusStatusDiberhentikan:
		default:
			return domain.NewValidationError("Status pengurus tidak valid (Demisioner/Diberhentikan)")
		}
		if strings.TrimSpace(keterangan) == "" {
			return domain.NewValidationError("Keterangan wajib diisi saat menonaktifkan SK")
		}
	}
	if s.skRepo == nil {
		return svcutil.Unavailable("surat keputusan")
	}
	sk, err := s.skRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if !actor.CanAccessWilayah(derefInt(sk.ProvinsiID), derefInt(sk.KabupatenID)) {
		return domain.NewForbiddenError("SK di luar wilayah kerja Anda")
	}
	if err := s.skRepo.SetStatusWithDemotion(ctx, id, status, pengurusStatus, keterangan); err != nil {
		return err
	}
	meta := `{"event":"sk_status","to":"` + string(status) + `"}`
	s.audit(ctx, audit, actor, "surat_keputusan", strconv.Itoa(id), "UPDATE", &meta)
	return nil
}
