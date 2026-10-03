package service

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/kipan-indonesia/sim-kipan-core/config"
	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/gateway"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
)

// KepengurusanService menangani SK, jabatan, dan pengangkatan kader menjadi
// pengurus. Otorisasi berlapis: peran (route) + yurisdiksi (di sini) +
// aturan rantai persetujuan.
type KepengurusanService interface {
	ListJabatan(ctx context.Context, includeInactive bool, level string) ([]domain.Jabatan, error)
	CreateJabatan(ctx context.Context, in domain.JabatanRequest, actor domain.ActorContext, audit domain.AuditContext) (*domain.Jabatan, error)
	UpdateJabatan(ctx context.Context, id int, in domain.JabatanRequest, actor domain.ActorContext, audit domain.AuditContext) (*domain.Jabatan, error)

	CreateSK(ctx context.Context, in domain.SKCreateRequest, actor domain.ActorContext, audit domain.AuditContext) (*domain.SuratKeputusan, error)
	ListSK(ctx context.Context, actor domain.ActorContext, level, status, approval, search string, withTotal bool, page, limit int) ([]domain.SKListItem, int, error)
	GetSK(ctx context.Context, id int, actor domain.ActorContext) (*SKDetail, error)
	ApproveSK(ctx context.Context, id int, action domain.SKApprovalAction, catatan string, actor domain.ActorContext, audit domain.AuditContext) error
	SetSKStatus(ctx context.Context, id int, status domain.SKStatus, actor domain.ActorContext, audit domain.AuditContext) error

	AddPengurus(ctx context.Context, skID int, in domain.AddPengurusRequest, actor domain.ActorContext, audit domain.AuditContext) (*domain.PengurusDetail, error)
	RemovePengurus(ctx context.Context, skID, pengurusID int, actor domain.ActorContext, audit domain.AuditContext) error
	ListPengurus(ctx context.Context, actor domain.ActorContext, level, status, masaJabatan, search string, provFilter, kabFilter *int, withTotal bool, page, limit int) ([]domain.PengurusDetail, int, error)
	PengurusStats(ctx context.Context, actor domain.ActorContext) (*domain.PengurusStats, error)
	UpdatePengurusStatus(ctx context.Context, id int, status domain.PengurusStatus, keterangan string, actor domain.ActorContext, audit domain.AuditContext) error
	UpdatePengurusJabatan(ctx context.Context, id int, in domain.UpdateJabatanRequest, actor domain.ActorContext, audit domain.AuditContext) (*domain.PengurusDetail, error)
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
	Mail         gateway.MailSender
}

type kepengurusanSvc struct {
	cfg         *config.Config
	jabatanRepo repository.JabatanRepository
	skRepo      repository.SKRepository
	pengurus    repository.PengurusRepository
	anggotaRepo repository.AnggotaRepository
	userRepo    repository.UserRepository
	auditRepo   repository.AuditLogRepository
	wilayahRepo repository.WilayahRepository
	mail        gateway.MailSender
}

func NewKepengurusanService(cfg *config.Config, deps KepengurusanDeps) KepengurusanService {
	return &kepengurusanSvc{
		cfg: cfg, jabatanRepo: deps.JabatanRepo, skRepo: deps.SKRepo,
		pengurus: deps.PengurusRepo, anggotaRepo: deps.AnggotaRepo,
		userRepo: deps.UserRepo, auditRepo: deps.AuditRepo,
		wilayahRepo: deps.WilayahRepo, mail: deps.Mail,
	}
}

func isNasionalOrSuper(role domain.Role) bool {
	return role == domain.RoleSuperAdmin || role == domain.RoleAdminNasional
}

func derefInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

// ============================================================
// JABATAN
// ============================================================

func (s *kepengurusanSvc) ListJabatan(ctx context.Context, includeInactive bool, level string) ([]domain.Jabatan, error) {
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

func (s *kepengurusanSvc) CreateJabatan(ctx context.Context, in domain.JabatanRequest, actor domain.ActorContext, audit domain.AuditContext) (*domain.Jabatan, error) {
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

func (s *kepengurusanSvc) UpdateJabatan(ctx context.Context, id int, in domain.JabatanRequest, actor domain.ActorContext, audit domain.AuditContext) (*domain.Jabatan, error) {
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

// ============================================================
// SURAT KEPUTUSAN
// ============================================================

func (s *kepengurusanSvc) CreateSK(ctx context.Context, in domain.SKCreateRequest, actor domain.ActorContext, audit domain.AuditContext) (*domain.SuratKeputusan, error) {
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
		return nil, unavailable("surat keputusan")
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
func (s *kepengurusanSvc) resolveSKScope(ctx context.Context, in domain.SKCreateRequest, actor domain.ActorContext) (domain.TingkatWilayah, *int, *int, error) {
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
			return "", nil, nil, unavailable("wilayah")
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

// scopeForActor mengembalikan batas wilayah daftar (nil = nasional).
func scopeForActor(actor domain.ActorContext) (*int, *int) {
	switch actor.Role {
	case domain.RoleAdminProvinsi:
		return actor.ProvinsiID, nil
	case domain.RoleAdminKabupaten:
		return actor.ProvinsiID, actor.KabupatenID
	default:
		return nil, nil
	}
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

func (s *kepengurusanSvc) ListSK(ctx context.Context, actor domain.ActorContext, level, status, approval, search string, withTotal bool, page, limit int) ([]domain.SKListItem, int, error) {
	if s.skRepo == nil {
		return nil, 0, unavailable("surat keputusan")
	}
	prov, kab := scopeForActor(actor)
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

func (s *kepengurusanSvc) GetSK(ctx context.Context, id int, actor domain.ActorContext) (*SKDetail, error) {
	if id <= 0 {
		return nil, domain.NewValidationError("ID SK tidak valid")
	}
	if s.skRepo == nil {
		return nil, unavailable("surat keputusan")
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

func (s *kepengurusanSvc) ApproveSK(ctx context.Context, id int, action domain.SKApprovalAction, catatan string, actor domain.ActorContext, audit domain.AuditContext) error {
	if id <= 0 {
		return domain.NewValidationError("ID SK tidak valid")
	}
	if s.skRepo == nil {
		return unavailable("surat keputusan")
	}
	sk, err := s.skRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	var from, to domain.SKApprovalStatus
	var approvedBy *string
	var approvedAt *time.Time
	var note *string
	now := time.Now()

	switch action {
	case domain.SKActionAjukan:
		if !canAjukanSK(actor, sk) {
			return domain.NewForbiddenError("Anda tidak berwenang mengajukan SK ini")
		}
		from = domain.SKApprovalStatusDraft
		switch sk.Level {
		case domain.LevelKabupaten:
			to = domain.SKApprovalStatusMenungguProvinsi
		case domain.LevelProvinsi:
			to = domain.SKApprovalStatusMenungguNasional
		case domain.LevelNasional:
			to = domain.SKApprovalStatusDisetujui
			approvedBy, approvedAt = &actor.UserID, &now
		default:
			return domain.NewValidationError("Level SK tidak valid")
		}
	case domain.SKActionTeruskan:
		if actor.Role != domain.RoleAdminProvinsi || actor.ProvinsiID == nil ||
			sk.ProvinsiID == nil || *actor.ProvinsiID != *sk.ProvinsiID {
			return domain.NewForbiddenError("Penerusan SK hanya oleh Admin Provinsi wilayah SK")
		}
		from, to = domain.SKApprovalStatusMenungguProvinsi, domain.SKApprovalStatusMenungguNasional
	case domain.SKActionSahkan:
		if !isNasionalOrSuper(actor.Role) {
			return domain.NewForbiddenError("Pengesahan SK hanya oleh Nasional/Super Admin")
		}
		from, to = domain.SKApprovalStatusMenungguNasional, domain.SKApprovalStatusDisetujui
		approvedBy, approvedAt = &actor.UserID, &now
	case domain.SKActionTolak:
		if !isNasionalOrSuper(actor.Role) {
			return domain.NewForbiddenError("Penolakan SK hanya oleh Nasional/Super Admin")
		}
		n := strings.TrimSpace(catatan)
		if n == "" {
			return domain.NewValidationError("Catatan wajib diisi untuk menolak SK")
		}
		note = &n
		from, to = domain.SKApprovalStatusMenungguNasional, domain.SKApprovalStatusDitolak
		approvedBy, approvedAt = &actor.UserID, &now
	default:
		return domain.NewValidationError("Aksi persetujuan SK tidak valid")
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

func (s *kepengurusanSvc) SetSKStatus(ctx context.Context, id int, status domain.SKStatus, actor domain.ActorContext, audit domain.AuditContext) error {
	if id <= 0 {
		return domain.NewValidationError("ID SK tidak valid")
	}
	if status != domain.SKStatusAktif && status != domain.SKStatusTidakAktif && status != domain.SKStatusDigantikan {
		return domain.NewValidationError("Status SK tidak valid")
	}
	if s.skRepo == nil {
		return unavailable("surat keputusan")
	}
	sk, err := s.skRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if !actor.CanAccessWilayah(derefInt(sk.ProvinsiID), derefInt(sk.KabupatenID)) {
		return domain.NewForbiddenError("SK di luar wilayah kerja Anda")
	}
	if err := s.skRepo.SetStatus(ctx, id, status); err != nil {
		return err
	}
	meta := `{"event":"sk_status","to":"` + string(status) + `"}`
	s.audit(ctx, audit, actor, "surat_keputusan", strconv.Itoa(id), "UPDATE", &meta)
	return nil
}

// ============================================================
// PENGURUS
// ============================================================

func (s *kepengurusanSvc) AddPengurus(ctx context.Context, skID int, in domain.AddPengurusRequest, actor domain.ActorContext, audit domain.AuditContext) (*domain.PengurusDetail, error) {
	if skID <= 0 || in.AnggotaID <= 0 || in.JabatanID <= 0 {
		return nil, domain.NewValidationError("Data pengangkatan tidak lengkap")
	}
	if !in.Konfirmasi {
		return nil, domain.NewValidationError("Konfirmasi kelayakan wajib dicentang")
	}
	if s.skRepo == nil || s.pengurus == nil || s.anggotaRepo == nil || s.jabatanRepo == nil {
		return nil, unavailable("kepengurusan")
	}

	sk, err := s.skRepo.GetByID(ctx, skID)
	if err != nil {
		return nil, err
	}
	if !canManageSK(actor, sk) {
		return nil, domain.NewForbiddenError("Anda tidak berwenang mengelola pengurus pada SK ini")
	}
	if sk.Status != domain.SKStatusAktif {
		return nil, domain.NewValidationError("SK tidak aktif")
	}
	if sk.ApprovalStatus == domain.SKApprovalStatusDisetujui {
		return nil, domain.NewForbiddenError("SK sudah final. Buat SK baru untuk perubahan susunan.")
	}
	if strings.TrimSpace(sk.FileSKKey) == "" {
		return nil, domain.NewValidationError("SK belum memiliki file. Unggah file SK terlebih dahulu.")
	}

	member, err := s.anggotaRepo.GetByID(ctx, in.AnggotaID)
	if err != nil {
		return nil, err
	}
	if member.Status != domain.AnggotaStatusAktif {
		return nil, domain.NewValidationError("Anggota tidak berstatus AKTIF")
	}
	if !anggotaInSKScope(member, sk) {
		return nil, domain.NewValidationError("Anggota berada di luar wilayah SK")
	}

	jabatan, err := s.jabatanRepo.GetByID(ctx, in.JabatanID)
	if err != nil {
		return nil, err
	}
	if !jabatan.IsActive {
		return nil, domain.NewValidationError("Jabatan tidak aktif")
	}
	if jabatan.Level != sk.Level {
		return nil, domain.NewValidationError("Jabatan tidak sesuai tingkat SK")
	}

	if exists, err := s.pengurus.ExistsInSK(ctx, skID, in.AnggotaID); err != nil {
		return nil, err
	} else if exists {
		return nil, domain.NewConflictError("Anggota sudah tercantum pada SK ini")
	}
	if jabatan.IsInti {
		n, err := s.pengurus.CountJabatanInSK(ctx, skID, in.JabatanID, in.AnggotaID)
		if err != nil {
			return nil, err
		}
		if n > 0 {
			return nil, domain.NewConflictError("Jabatan inti " + jabatan.Nama + " sudah terisi pada SK ini")
		}
	}

	userID := ""
	if member.UserID != nil {
		userID = *member.UserID
	}
	newID, err := s.pengurus.AddWithPromotion(ctx, repository.PromoteInput{
		AnggotaID: in.AnggotaID, SKID: skID, UserID: userID,
		Level: string(sk.Level), ProvinsiID: sk.ProvinsiID, KabupatenID: sk.KabupatenID,
		JabatanID: in.JabatanID, TanggalMulai: sk.TanggalTerbit,
		TanggalSelesai: time.Now(), Keterangan: "Digantikan pengurus baru",
	})
	if err != nil {
		return nil, err
	}

	pMeta := `{"event":"pengurus_create","sk_id":` + strconv.Itoa(skID) + `,"jabatan_id":` + strconv.Itoa(in.JabatanID) + `}`
	s.audit(ctx, audit, actor, "pengurus", strconv.Itoa(newID), "CREATE", &pMeta)
	tMeta := `{"event":"tipe_promote","to":"PENGURUS"}`
	s.audit(ctx, audit, actor, "anggota", strconv.Itoa(in.AnggotaID), "UPDATE", &tMeta)
	if userID != "" {
		uMeta := `{"event":"tipe_user_promote","to":"PENGURUS","sessions_revoked":true}`
		s.audit(ctx, audit, actor, "users", userID, "UPDATE", &uMeta)
	}
	s.sendAppointmentEmail(member.NamaLengkap, member.Email, member.NIA, jabatan.Nama, sk.NomorSK)

	list, err := s.pengurus.ListBySK(ctx, skID)
	if err != nil {
		return nil, err
	}
	for i := range list {
		if list[i].ID == newID {
			return &list[i], nil
		}
	}
	return &domain.PengurusDetail{ID: newID}, nil
}

func (s *kepengurusanSvc) RemovePengurus(ctx context.Context, skID, pengurusID int, actor domain.ActorContext, audit domain.AuditContext) error {
	if skID <= 0 || pengurusID <= 0 {
		return domain.NewValidationError("Data tidak valid")
	}
	if s.skRepo == nil || s.pengurus == nil {
		return unavailable("kepengurusan")
	}
	sk, err := s.skRepo.GetByID(ctx, skID)
	if err != nil {
		return err
	}
	if !canManageSK(actor, sk) {
		return domain.NewForbiddenError("Anda tidak berwenang mengelola pengurus pada SK ini")
	}
	if sk.ApprovalStatus == domain.SKApprovalStatusDisetujui {
		return domain.NewForbiddenError("SK sudah final. Susunan pengurus terkunci.")
	}
	list, err := s.pengurus.ListBySK(ctx, skID)
	if err != nil {
		return err
	}
	found := false
	for i := range list {
		if list[i].ID == pengurusID {
			found = true
			break
		}
	}
	if !found {
		return domain.ErrNotFound
	}
	if err := s.pengurus.Remove(ctx, pengurusID); err != nil {
		return err
	}
	meta := `{"event":"pengurus_remove","sk_id":` + strconv.Itoa(skID) + `}`
	s.audit(ctx, audit, actor, "pengurus", strconv.Itoa(pengurusID), "DELETE", &meta)
	return nil
}

func (s *kepengurusanSvc) ListPengurus(ctx context.Context, actor domain.ActorContext, level, status, masaJabatan, search string, provFilter, kabFilter *int, withTotal bool, page, limit int) ([]domain.PengurusDetail, int, error) {
	if s.pengurus == nil {
		return nil, 0, unavailable("pengurus")
	}
	// Scope dari peran TIDAK bisa dilonggarkan klien; filter klien hanya
	// berlaku bila server belum menetapkan batas (Nasional/Super).
	prov, kab := scopeForActor(actor)
	if prov == nil {
		prov = provFilter
	}
	if kab == nil {
		kab = kabFilter
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
	return s.pengurus.List(ctx, repository.PengurusFilter{
		ProvinsiID: prov, KabupatenID: kab,
		Level:  strings.ToUpper(strings.TrimSpace(level)),
		Status: status, MasaJabatan: strings.TrimSpace(masaJabatan), Search: search,
		WithTotal: withTotal, Limit: limit, Offset: (page - 1) * limit,
	})
}

// PengurusStats ringkasan jumlah pengurus aktif (ter-scope) untuk kartu dasbor.
func (s *kepengurusanSvc) PengurusStats(ctx context.Context, actor domain.ActorContext) (*domain.PengurusStats, error) {
	if s.pengurus == nil {
		return nil, unavailable("pengurus")
	}
	prov, kab := scopeForActor(actor)
	out, err := s.pengurus.Stats(ctx, repository.PengurusFilter{ProvinsiID: prov, KabupatenID: kab})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *kepengurusanSvc) UpdatePengurusStatus(ctx context.Context, id int, status domain.PengurusStatus, keterangan string, actor domain.ActorContext, audit domain.AuditContext) error {
	if id <= 0 {
		return domain.NewValidationError("ID pengurus tidak valid")
	}
	switch status {
	case domain.PengurusStatusAktif, domain.PengurusStatusDemisioner,
		domain.PengurusStatusDiberhentikan, domain.PengurusStatusMengundurkanDiri,
		domain.PengurusStatusMeninggal:
	default:
		return domain.NewValidationError("Status pengurus tidak valid")
	}
	note := strings.TrimSpace(keterangan)
	if status != domain.PengurusStatusAktif && note == "" {
		return domain.NewValidationError("Keterangan wajib diisi saat menonaktifkan pengurus")
	}
	if s.pengurus == nil {
		return unavailable("pengurus")
	}
	p, err := s.pengurus.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if isNasionalOrSuper(actor.Role) {
		// nasional: tanpa batas wilayah
	} else if !actor.CanAccessWilayah(derefInt(p.ProvinsiID), derefInt(p.KabupatenID)) {
		return domain.NewForbiddenError("Pengurus di luar wilayah kerja Anda")
	}
	if err := s.pengurus.UpdateStatus(ctx, id, status, note); err != nil {
		return err
	}
	meta := `{"event":"pengurus_status","to":"` + string(status) + `"}`
	s.audit(ctx, audit, actor, "pengurus", strconv.Itoa(id), "UPDATE", &meta)
	return nil
}

// UpdatePengurusJabatan mengganti jabatan seorang pengurus (in-place, karena
// UNIQUE(surat_keputusan_id, anggota_id)). Terkunci saat SK final; jabatan inti
// harus tunggal per SK.
func (s *kepengurusanSvc) UpdatePengurusJabatan(ctx context.Context, id int, in domain.UpdateJabatanRequest, actor domain.ActorContext, audit domain.AuditContext) (*domain.PengurusDetail, error) {
	if id <= 0 || in.JabatanID <= 0 {
		return nil, domain.NewValidationError("Data ganti jabatan tidak lengkap")
	}
	if s.pengurus == nil || s.skRepo == nil || s.jabatanRepo == nil {
		return nil, unavailable("kepengurusan")
	}
	p, err := s.pengurus.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	sk, err := s.skRepo.GetByID(ctx, p.SuratKeputusanID)
	if err != nil {
		return nil, err
	}
	if !canManageSK(actor, sk) {
		return nil, domain.NewForbiddenError("Anda tidak berwenang mengelola pengurus pada SK ini")
	}
	if sk.Status != domain.SKStatusAktif {
		return nil, domain.NewValidationError("SK tidak aktif")
	}
	if sk.ApprovalStatus == domain.SKApprovalStatusDisetujui {
		return nil, domain.NewForbiddenError("SK sudah final. Susunan pengurus terkunci.")
	}
	if p.JabatanID == in.JabatanID {
		return p, nil // tidak ada perubahan
	}
	jabatan, err := s.jabatanRepo.GetByID(ctx, in.JabatanID)
	if err != nil {
		return nil, err
	}
	if !jabatan.IsActive {
		return nil, domain.NewValidationError("Jabatan tidak aktif")
	}
	if jabatan.Level != sk.Level {
		return nil, domain.NewValidationError("Jabatan tidak sesuai tingkat SK")
	}
	if jabatan.IsInti {
		n, err := s.pengurus.CountJabatanInSK(ctx, sk.ID, in.JabatanID, p.AnggotaID)
		if err != nil {
			return nil, err
		}
		if n > 0 {
			return nil, domain.NewConflictError("Jabatan inti " + jabatan.Nama + " sudah terisi pada SK ini")
		}
	}
	if err := s.pengurus.UpdateJabatan(ctx, id, in.JabatanID); err != nil {
		return nil, err
	}
	meta := `{"event":"pengurus_ganti_jabatan","from_jabatan_id":` + strconv.Itoa(p.JabatanID) + `,"to_jabatan_id":` + strconv.Itoa(in.JabatanID) + `}`
	s.audit(ctx, audit, actor, "pengurus", strconv.Itoa(id), "UPDATE", &meta)
	out, err := s.pengurus.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// sendAppointmentEmail mengirim notifikasi pengangkatan (async best-effort).
func (s *kepengurusanSvc) sendAppointmentEmail(nama, email, nia, jabatan, nomorSK string) {
	if s.mail == nil || strings.TrimSpace(email) == "" {
		return
	}
	content := PengangkatanEmail(nama, nia, jabatan, nomorSK, publicURLFrom(s.cfg))
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := s.mail.Send(ctx, email, content.Subject, content.TextBody, content.HTMLBody); err != nil {
			log.Warn().Err(err).Str("nia", nia).Msg("Gagal mengirim email pengangkatan")
		}
	}()
}

func (s *kepengurusanSvc) audit(ctx context.Context, audit domain.AuditContext, actor domain.ActorContext, entity, entityID, action string, metadata *string) {
	id := actor.UserID
	writeAudit(ctx, s.auditRepo, audit, &id, actor.Name, string(actor.Role), entity, entityID, action, metadata)
}
