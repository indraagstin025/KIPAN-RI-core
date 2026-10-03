package service

// pengurus_service.go — service fokus: kepengurusan (pengangkatan/penetapan
// pengurus pada SK) (Fase A4, SRP).

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
)

// PengurusService mengelola pengangkatan & status pengurus pada SK.
type PengurusService interface {
	AddPengurus(ctx context.Context, skID int, in domain.AddPengurusRequest, actor domain.ActorContext, audit domain.AuditContext) (*domain.PengurusDetail, error)
	RemovePengurus(ctx context.Context, skID, pengurusID int, actor domain.ActorContext, audit domain.AuditContext) error
	ListPengurus(ctx context.Context, actor domain.ActorContext, level, status, masaJabatan, search string, provFilter, kabFilter *int, withTotal bool, page, limit int) ([]domain.PengurusDetail, int, error)
	PengurusStats(ctx context.Context, actor domain.ActorContext) (*domain.PengurusStats, error)
	ListPromosi(ctx context.Context, actor domain.ActorContext, search string, limit int) ([]domain.PromosiCandidate, error)
	UpdatePengurusStatus(ctx context.Context, id int, status domain.PengurusStatus, keterangan string, actor domain.ActorContext, audit domain.AuditContext) error
	UpdatePengurusJabatan(ctx context.Context, id int, in domain.UpdateJabatanRequest, actor domain.ActorContext, audit domain.AuditContext) (*domain.PengurusDetail, error)
}

type pengurusSvc struct{ *kepengurusanBase }

// AddPengurus mengangkat kader ke SK (validasi berlapis + efek atomik).
func (s *pengurusSvc) AddPengurus(ctx context.Context, skID int, in domain.AddPengurusRequest, actor domain.ActorContext, audit domain.AuditContext) (*domain.PengurusDetail, error) {
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
	// Tanggal mulai jabatan: pakai input bila diisi, selain itu tanggal terbit SK.
	mulai := sk.TanggalTerbit
	if in.TanggalMulai != nil && !in.TanggalMulai.IsZero() {
		mulai = *in.TanggalMulai
	}
	newID, err := s.pengurus.AddWithPromotion(ctx, repository.PromoteInput{
		AnggotaID: in.AnggotaID, SKID: skID, UserID: userID,
		Level: string(sk.Level), ProvinsiID: sk.ProvinsiID, KabupatenID: sk.KabupatenID,
		JabatanID: in.JabatanID, TanggalMulai: mulai,
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
	// Notifikasi pengangkatan via outbox (konsisten dengan email lain & tahan
	// gagal sementara). userID hanya terisi bila anggota sudah punya akun.
	var uidPtr *string
	if userID != "" {
		uidPtr = &userID
	}
	s.enqueueEmail(ctx, domain.EmailOutboxPengangkatan, sk, uidPtr, member.Email,
		PengangkatanEmail(member.NamaLengkap, member.NIA, jabatan.Nama, sk.NomorSK, publicURLFrom(s.cfg)))

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

// RemovePengurus melepas pengurus dari SK (hanya bila SK belum final).
func (s *pengurusSvc) RemovePengurus(ctx context.Context, skID, pengurusID int, actor domain.ActorContext, audit domain.AuditContext) error {
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

// ListPengurus mengembalikan daftar pengurus ter-scope dengan filter lengkap.
func (s *pengurusSvc) ListPengurus(ctx context.Context, actor domain.ActorContext, level, status, masaJabatan, search string, provFilter, kabFilter *int, withTotal bool, page, limit int) ([]domain.PengurusDetail, int, error) {
	if s.pengurus == nil {
		return nil, 0, unavailable("pengurus")
	}
	// Scope dari peran TIDAK bisa dilonggarkan klien; filter klien hanya
	// berlaku bila server belum menetapkan batas (Nasional/Super).
	prov, kab, err := actor.Scope()
	if err != nil {
		return nil, 0, err
	}
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
func (s *pengurusSvc) PengurusStats(ctx context.Context, actor domain.ActorContext) (*domain.PengurusStats, error) {
	if s.pengurus == nil {
		return nil, unavailable("pengurus")
	}
	prov, kab, err := actor.Scope()
	if err != nil {
		return nil, err
	}
	out, err := s.pengurus.Stats(ctx, repository.PengurusFilter{ProvinsiID: prov, KabupatenID: kab})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// ListPromosi mengembalikan kandidat "Promosi Pengurus" (anggota AKTIF
// ber-riwayat kepengurusan yang tidak sedang aktif menjabat).
func (s *pengurusSvc) ListPromosi(ctx context.Context, actor domain.ActorContext, search string, limit int) ([]domain.PromosiCandidate, error) {
	if s.pengurus == nil {
		return nil, unavailable("pengurus")
	}
	prov, kab, err := actor.Scope()
	if err != nil {
		return nil, err
	}
	return s.pengurus.ListPromosi(ctx, prov, kab, strings.TrimSpace(search), limit)
}

// UpdatePengurusStatus mengubah status pengurus (keterangan wajib bila non-Aktif).
func (s *pengurusSvc) UpdatePengurusStatus(ctx context.Context, id int, status domain.PengurusStatus, keterangan string, actor domain.ActorContext, audit domain.AuditContext) error {
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
func (s *pengurusSvc) UpdatePengurusJabatan(ctx context.Context, id int, in domain.UpdateJabatanRequest, actor domain.ActorContext, audit domain.AuditContext) (*domain.PengurusDetail, error) {
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
