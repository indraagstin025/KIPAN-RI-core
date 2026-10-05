package anggota

// AnggotaService melayani daftar kader resmi untuk admin (terfilter
// jurisdiction, proyeksi non-PII) dan cek publik minimal pengganti
// cek-anggota lama (tanpa NIK, alamat, kontak, maupun object key).

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/alexedwards/argon2id"
	"github.com/rs/zerolog/log"

	"github.com/kipan-indonesia/sim-kipan-core/config"
	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
	"github.com/kipan-indonesia/sim-kipan-core/internal/service/svcutil"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/keyset"
)

type AnggotaService interface {
	// ListAnggota mengembalikan daftar sesuai jurisdiction aktor.
	ListAnggota(ctx context.Context, actor domain.ActorContext, status, search string, page, limit int) ([]domain.AnggotaListItem, int, error)
	// ListAnggotaCursor varian keyset (tanpa COUNT) untuk daftar besar.
	ListAnggotaCursor(ctx context.Context, actor domain.ActorContext, status, search, cursor string, limit int) ([]domain.AnggotaListItem, string, error)
	// GetAnggotaDetail melayani admin: tolak objek di luar wilayah aktor.
	GetAnggotaDetail(ctx context.Context, id int, actor domain.ActorContext) (*domain.Anggota, error)
	// GetPublicAnggota adalah jalur publik by NIA (tanpa pencarian NIK:
	// NIK mentah dari publik adalah oracle PII — ditolak by design).
	GetPublicAnggota(ctx context.Context, nia string) (*domain.AnggotaPublicInfo, error)
	// ResetMemberPassword (T1) menerbitkan password baru untuk akun USER
	// anggota dalam yurisdiksi aktor. Password dikembalikan SEKALI; seluruh
	// sesi anggota dicabut dan aksi tercatat di audit.
	ResetMemberPassword(ctx context.Context, anggotaID int, actor domain.ActorContext, audit domain.AuditContext) (string, error)

	// CreateAnggota menambah anggota langsung (di luar alur pendaftaran).
	CreateAnggota(ctx context.Context, in domain.AnggotaCreateRequest, actor domain.ActorContext, audit domain.AuditContext) (*domain.Anggota, error)
	// UpdateAnggota menyunting data anggota (NIK/NIA tidak diubah).
	UpdateAnggota(ctx context.Context, id int, in domain.AnggotaUpdateRequest, actor domain.ActorContext, audit domain.AuditContext) (*domain.Anggota, error)
	// SetAnggotaStatus mengubah status keanggotaan (soft; mis. NONAKTIF).
	SetAnggotaStatus(ctx context.Context, id int, in domain.AnggotaStatusRequest, actor domain.ActorContext, audit domain.AuditContext) (*domain.Anggota, error)
	// ExportCSV mengekspor daftar anggota ter-scope (NIA, Nama, Pekerjaan, Riwayat, wilayah, status).
	ExportCSV(ctx context.Context, actor domain.ActorContext, status, search string) ([]byte, error)
	// AnggotaRiwayat mengembalikan timeline riwayat anggota (pendaftaran + kepengurusan).
	AnggotaRiwayat(ctx context.Context, id int, actor domain.ActorContext) ([]domain.AnggotaRiwayatItem, error)
	// AnggotaActivity mengembalikan jejak audit (activity_logs) milik anggota.
	AnggotaActivity(ctx context.Context, id int, actor domain.ActorContext) ([]domain.ActivityLog, error)
}

type AnggotaDeps struct {
	AnggotaRepo     repository.AnggotaRepository
	WilayahRepo     repository.WilayahRepository
	UserRepo        repository.UserAccountRepository
	AuditRepo       repository.AuditLogRepository
	ListRepo        repository.ListKeysetRepository
	OutboxRepo      repository.EmailOutboxRepository
	PendaftaranRepo repository.PendaftaranCoreRepository
	PengurusRepo    repository.PengurusRepository
}

type anggotaService struct {
	cfg             *config.Config
	anggotaRepo     repository.AnggotaRepository
	wilayahRepo     repository.WilayahRepository
	userRepo        repository.UserAccountRepository
	auditRepo       repository.AuditLogRepository
	listRepo        repository.ListKeysetRepository
	outboxRepo      repository.EmailOutboxRepository
	pendaftaranRepo repository.PendaftaranCoreRepository
	pengurusRepo    repository.PengurusRepository
}

func NewAnggotaService(cfg *config.Config, deps AnggotaDeps) AnggotaService {
	return &anggotaService{
		cfg:             cfg,
		anggotaRepo:     deps.AnggotaRepo,
		wilayahRepo:     deps.WilayahRepo,
		userRepo:        deps.UserRepo,
		auditRepo:       deps.AuditRepo,
		listRepo:        deps.ListRepo,
		outboxRepo:      deps.OutboxRepo,
		pendaftaranRepo: deps.PendaftaranRepo,
		pengurusRepo:    deps.PengurusRepo,
	}
}

// normalizeNIA menyeragamkan NIA (trim + uppercase) dan menolak format
// asing (422). Menerima kode provinsi huruf (warisan JB) maupun BPS
// numerik (baru 32) karena sequence migrasi data mencakup keduanya.
// Pencarian NIK publik sengaja tidak didukung (oracle PII): gunakan NIA.
func normalizeNIA(nia string) (string, error) {
	code := strings.ToUpper(strings.TrimSpace(nia))
	if code == "" || len(code) > 50 || !strings.HasPrefix(code, "KIPAN-") {
		return "", domain.NewValidationError("NIA tidak valid (gunakan Nomor Induk Anggota, cth KIPAN-IND-3204-2026-000001)")
	}
	return code, nil
}

func (s *anggotaService) ListAnggota(ctx context.Context, actor domain.ActorContext, status, search string, page, limit int) ([]domain.AnggotaListItem, int, error) {
	if s.anggotaRepo == nil {
		return nil, 0, svcutil.Unavailable("anggota")
	}
	st := strings.TrimSpace(status)
	if st != "" && !domain.AnggotaStatus(st).IsValid() {
		return nil, 0, domain.NewValidationError("Filter status tidak valid")
	}
	if len([]rune(strings.TrimSpace(search))) > 100 {
		return nil, 0, domain.NewValidationError("Kata kunci pencarian maksimal 100 karakter")
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
	offset := (page - 1) * limit

	provID, kabID, err := actor.Scope()
	if err != nil {
		return nil, 0, err
	}
	items, err := s.anggotaRepo.ListAnggota(ctx, provID, kabID, st, strings.TrimSpace(search), limit, offset)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.anggotaRepo.CountAnggota(ctx, provID, kabID, st, strings.TrimSpace(search))
	if err != nil {
		return nil, 0, err
	}
	s.attachRiwayat(ctx, items)
	return items, total, nil
}

// attachRiwayat mengisi kolom RIWAYAT (TDD §5.5) untuk sejumlah item daftar.
// Bila tanpa riwayat kepengurusan, fallback ke label event pendaftaran
// terakhir agar kolom tidak kosong untuk kader baru.
func (s *anggotaService) attachRiwayat(ctx context.Context, items []domain.AnggotaListItem) {
	if s.anggotaRepo == nil || len(items) == 0 {
		return
	}
	ids := make([]int, 0, len(items))
	for i := range items {
		ids = append(ids, items[i].ID)
	}
	byID, err := s.anggotaRepo.RiwayatByAnggotaIDs(ctx, ids)
	if err != nil {
		log.Warn().Err(err).Msg("Gagal menghitung riwayat anggota; kolom RIWAYAT dikosongkan")
		return
	}
	needFallback := make([]int, 0)
	for i := range items {
		if v, ok := byID[items[i].ID]; ok && v != "-" {
			items[i].Riwayat = v
		} else {
			needFallback = append(needFallback, items[i].ID)
		}
	}
	if len(needFallback) == 0 {
		return
	}
	fb, err := s.anggotaRepo.LatestPendaftaranAksiByAnggotaIDs(ctx, needFallback)
	if err != nil {
		log.Warn().Err(err).Msg("Gagal memuat fallback riwayat pendaftaran")
		for i := range items {
			if items[i].Riwayat == "" {
				items[i].Riwayat = "-"
			}
		}
		return
	}
	for i := range items {
		if items[i].Riwayat != "" {
			continue
		}
		if aksi, ok := fb[items[i].ID]; ok {
			items[i].Riwayat = labelPendaftaranAksi(aksi)
		} else {
			items[i].Riwayat = "-"
		}
	}
}

// ListAnggotaCursor varian keyset (tanpa COUNT + tanpa OFFSET besar).
func (s *anggotaService) ListAnggotaCursor(ctx context.Context, actor domain.ActorContext, status, search, cursor string, limit int) ([]domain.AnggotaListItem, string, error) {
	if s.listRepo == nil {
		return nil, "", svcutil.Unavailable("anggota")
	}
	st := strings.TrimSpace(status)
	if st != "" && !domain.AnggotaStatus(st).IsValid() {
		return nil, "", domain.NewValidationError("Filter status tidak valid")
	}
	q := strings.TrimSpace(search)
	if len([]rune(q)) > 100 {
		return nil, "", domain.NewValidationError("Kata kunci pencarian maksimal 100 karakter")
	}
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}
	provID, kabID, err := actor.Scope()
	if err != nil {
		return nil, "", err
	}
	// Tanpa cursor: mulai dari "sekarang" (semua baris eligible) agar memakai
	// indeks komposit (created_at, id) alih-alih OFFSET.
	at := time.Now().UTC().Add(time.Hour)
	id := svcutil.MaxInt4
	if strings.TrimSpace(cursor) != "" {
		at, id, err = keyset.Decode(cursor)
		if err != nil {
			return nil, "", domain.NewValidationError("Cursor tidak valid")
		}
	}
	items, err := s.listRepo.ListAnggotaKeyset(ctx, provID, kabID, st, q, at, id, limit+1)
	if err != nil {
		return nil, "", err
	}
	next := ""
	if len(items) > limit {
		items = items[:limit]
		last := items[len(items)-1]
		next = keyset.Encode(last.CreatedAt, last.ID)
	}
	s.attachRiwayat(ctx, items)
	return items, next, nil
}

func (s *anggotaService) GetAnggotaDetail(ctx context.Context, id int, actor domain.ActorContext) (*domain.Anggota, error) {
	if id <= 0 {
		return nil, domain.NewValidationError("ID anggota tidak valid")
	}
	if s.anggotaRepo == nil {
		return nil, svcutil.Unavailable("anggota")
	}
	item, err := s.anggotaRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !actor.CanAccessWilayah(item.ProvinsiID, item.KabupatenID) {
		return nil, domain.NewForbiddenError("Data anggota di luar wilayah kerja Anda")
	}
	if m, err := s.anggotaRepo.RiwayatByAnggotaIDs(ctx, []int{id}); err == nil {
		item.Riwayat = m[id]
	}
	if s.wilayahRepo != nil {
		if prov, kab, err := s.wilayahRepo.GetNames(ctx, item.ProvinsiID, item.KabupatenID); err == nil {
			item.ProvinsiNama, item.KabupatenNama = prov, kab
		}
	}
	return item, nil
}

func (s *anggotaService) GetPublicAnggota(ctx context.Context, nia string) (*domain.AnggotaPublicInfo, error) {
	code, err := normalizeNIA(nia)
	if err != nil {
		return nil, err
	}
	if s.anggotaRepo == nil {
		return nil, svcutil.Unavailable("anggota")
	}
	item, err := s.anggotaRepo.GetByNIA(ctx, code)
	if err != nil {
		return nil, err
	}
	var provNama, kabNama string
	if s.wilayahRepo != nil {
		if prov, kab, err := s.wilayahRepo.GetNames(ctx, item.ProvinsiID, item.KabupatenID); err == nil {
			provNama, kabNama = prov, kab
		}
	}
	return &domain.AnggotaPublicInfo{
		NIA:           item.NIA,
		NamaLengkap:   item.NamaLengkap,
		Status:        string(item.Status),
		ProvinsiNama:  provNama,
		KabupatenNama: kabNama,
	}, nil
}

// ResetMemberPassword (T1) menerbitkan password awal baru untuk akun USER
// milik anggota. Langkah: scope wilayah → pastikan akun terhubung & jabatan
// role USER → hash Argon2id → simpan → cabut semua sesi → audit (tanpa
// password). Plaintext dikembalikan SEKALI ke pemanggil (admin) untuk
// diteruskan ke anggota via kanal resmi.
func (s *anggotaService) ResetMemberPassword(ctx context.Context, anggotaID int, actor domain.ActorContext, audit domain.AuditContext) (string, error) {
	if anggotaID <= 0 {
		return "", domain.NewValidationError("ID anggota tidak valid")
	}
	if s.anggotaRepo == nil {
		return "", svcutil.Unavailable("anggota")
	}
	if s.userRepo == nil {
		return "", svcutil.Unavailable("akun user")
	}

	member, err := s.anggotaRepo.GetByID(ctx, anggotaID)
	if err != nil {
		return "", err
	}
	if !actor.CanAccessWilayah(member.ProvinsiID, member.KabupatenID) {
		return "", domain.NewForbiddenError("Anggota di luar wilayah kerja Anda")
	}
	if member.UserID == nil || strings.TrimSpace(*member.UserID) == "" {
		return "", domain.NewConflictError("Anggota belum memiliki akun yang terhubung")
	}

	user, err := s.userRepo.GetByID(ctx, *member.UserID)
	if err != nil {
		return "", err
	}
	// Jalur ini KHUSUS akun anggota; jangan pernah reset password akun
	// admin/verifikator lewat data anggota.
	if user.Role != domain.RoleUser {
		return "", domain.NewConflictError("Akun terhubung bukan akun anggota (role " + string(user.Role) + ")")
	}

	password, err := svcutil.GenerateMemberPassword(16)
	if err != nil {
		return "", fmt.Errorf("gagal membuat password baru: %w", err)
	}
	hash, err := argon2id.CreateHash(password, svcutil.Argon2Params)
	if err != nil {
		return "", fmt.Errorf("gagal hash password baru: %w", err)
	}
	if err := s.userRepo.UpdatePassword(ctx, user.ID, hash); err != nil {
		return "", fmt.Errorf("gagal menyimpan password baru: %w", err)
	}
	if err := s.userRepo.RevokeAllUserTokens(ctx, user.ID); err != nil {
		log.Warn().Err(err).Str("user_id", user.ID).
			Msg("Gagal mencabut sesi setelah reset password anggota")
	}
	// Opsi A: kirim tautan set-password via antrian; admin TIDAK melihat password.
	if s.outboxRepo != nil {
		html := ""
		entry := &domain.EmailOutbox{
			Jenis:      domain.EmailOutboxSetPassword,
			UserID:     &user.ID,
			ToEmail:    user.Email,
			Subject:    "Buat Kata Sandi Akun KIPAN",
			TextBody:   "Buat kata sandi akun Anda melalui tautan pada email ini.",
			HTMLBody:   &html,
			ProvinsiID: &member.ProvinsiID, KabupatenID: &member.KabupatenID,
		}
		enqCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := s.outboxRepo.Enqueue(enqCtx, entry); err != nil {
			log.Warn().Err(err).Str("user_id", user.ID).Msg("Gagal enqueue email set-password")
		}
		cancel()
	}

	actorID := actor.UserID
	meta := `{"event":"member_password_reset","sessions_revoked":true,"email_queued":true}`
	svcutil.WriteAudit(ctx, s.auditRepo, audit, &actorID, actor.Name, string(actor.Role),
		"anggota", strconv.Itoa(member.ID), "PASSWORD_RESET", &meta)
	return "", nil
}
