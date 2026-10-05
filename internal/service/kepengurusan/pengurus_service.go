package kepengurusan

// pengurus_service.go — service fokus: kepengurusan (pengangkatan/penetapan
// pengurus pada SK) (Fase A4, SRP).

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
	"github.com/kipan-indonesia/sim-kipan-core/internal/service/mail"
	"github.com/kipan-indonesia/sim-kipan-core/internal/service/svcutil"
)

// PengurusService mengelola pengangkatan & status pengurus pada SK.
type PengurusService interface {
	AddPengurus(ctx context.Context, skID int, in domain.AddPengurusRequest, actor domain.ActorContext, audit domain.AuditContext) (*domain.PengurusDetail, error)
	RemovePengurus(ctx context.Context, skID, pengurusID int, actor domain.ActorContext, audit domain.AuditContext) error
	ListPengurus(ctx context.Context, actor domain.ActorContext, level, status, masaJabatan, search string, provFilter, kabFilter *int, withTotal bool, page, limit int) ([]domain.PengurusDetail, int, error)
	// GetPengurusDetail mengambil detail lengkap satu pengurus: baris
	// kepengurusan + biodata anggota tertaut + riwayat kepengurusannya.
	GetPengurusDetail(ctx context.Context, id int, actor domain.ActorContext) (*domain.PengurusDetailResponse, error)
	PengurusStats(ctx context.Context, actor domain.ActorContext) (*domain.PengurusStats, error)
	ListPromosi(ctx context.Context, actor domain.ActorContext, search string, limit int) ([]domain.PromosiCandidate, error)
	UpdatePengurusStatus(ctx context.Context, id int, status domain.PengurusStatus, keterangan string, actor domain.ActorContext, audit domain.AuditContext) error
	UpdatePengurusJabatan(ctx context.Context, id int, in domain.UpdateJabatanRequest, actor domain.ActorContext, audit domain.AuditContext) (*domain.PengurusDetail, error)
	// Paws mengakhiri masa bakti individual (Demisioner/Diberhentikan/
	// Mengundurkan diri/Meninggal) sesuai TDD §5.6.
	Paws(ctx context.Context, pengurusID int, in domain.PawsRequest, actor domain.ActorContext, audit domain.AuditContext) error
	// Mutasi memindahkan pengurus ke SK/jabatan tujuan (tutup lama, buka baru).
	Mutasi(ctx context.Context, pengurusID int, in domain.MutasiRequest, actor domain.ActorContext, audit domain.AuditContext) (*domain.PengurusDetail, error)
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
		return nil, svcutil.Unavailable("kepengurusan")
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
		mail.PengangkatanEmail(member.NamaLengkap, member.NIA, jabatan.Nama, sk.NomorSK, svcutil.PublicURLFrom(s.cfg)))

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
		return svcutil.Unavailable("kepengurusan")
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
		return nil, 0, svcutil.Unavailable("pengurus")
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

// GetPengurusDetail mengambil detail lengkap satu pengurus: baris kepengurusan
// (jabatan+SK+wilayah) + biodata anggota tertaut + riwayat kepengurusannya.
// Otorisasi memakai yurisdiksi aktor (konsisten dengan scoping daftar): pengurus
// level NASIONAL (prov/kab nil) hanya terlihat admin Nasional/Super.
func (s *pengurusSvc) GetPengurusDetail(ctx context.Context, id int, actor domain.ActorContext) (*domain.PengurusDetailResponse, error) {
	if id <= 0 {
		return nil, domain.NewValidationError("ID pengurus tidak valid")
	}
	if s.pengurus == nil {
		return nil, svcutil.Unavailable("pengurus")
	}
	p, err := s.pengurus.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	provID, kabID := 0, 0
	if p.ProvinsiID != nil {
		provID = *p.ProvinsiID
	}
	if p.KabupatenID != nil {
		kabID = *p.KabupatenID
	}
	if !actor.CanAccessWilayah(provID, kabID) {
		return nil, domain.NewForbiddenError("Pengurus di luar wilayah kerja Anda")
	}

	resp := &domain.PengurusDetailResponse{Pengurus: *p}
	if s.anggotaRepo != nil && p.AnggotaID > 0 {
		if member, err := s.anggotaRepo.GetByID(ctx, p.AnggotaID); err == nil && member != nil {
			if m, err := s.anggotaRepo.RiwayatByAnggotaIDs(ctx, []int{p.AnggotaID}); err == nil {
				member.Riwayat = m[p.AnggotaID]
			}
			if s.wilayahRepo != nil {
				if prov, kab, err := s.wilayahRepo.GetNames(ctx, member.ProvinsiID, member.KabupatenID); err == nil {
					member.ProvinsiNama, member.KabupatenNama = prov, kab
				}
			}
			resp.Anggota = member
		}
	}
	if riwayat, err := s.pengurus.ListByAnggota(ctx, p.AnggotaID); err == nil {
		resp.Riwayat = riwayat
	}
	return resp, nil
}

// PengurusStats ringkasan jumlah pengurus aktif (ter-scope) untuk kartu dasbor.
func (s *pengurusSvc) PengurusStats(ctx context.Context, actor domain.ActorContext) (*domain.PengurusStats, error) {
	if s.pengurus == nil {
		return nil, svcutil.Unavailable("pengurus")
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
		return nil, svcutil.Unavailable("pengurus")
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
		return svcutil.Unavailable("pengurus")
	}
	p, err := s.pengurus.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if isNasionalOrSuper(actor.Role) {
		// nasional: tanpa batas wilayah
	} else if !actor.CanAccessWilayah(svcutil.DerefInt(p.ProvinsiID), svcutil.DerefInt(p.KabupatenID)) {
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
		return nil, svcutil.Unavailable("kepengurusan")
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

// canPaws menilai wewenang aksi PAW. Umumnya mengikuti canManageSK; khusus
// DIBERHENTIKAN dibatasi Admin Provinsi seprov atau Nasional (TDD Tabel 20).
func canPaws(actor domain.ActorContext, sk *domain.SuratKeputusan, action domain.PengurusPAWAction) bool {
	if actor.Role == domain.RoleSuperAdmin {
		return true
	}
	if action == domain.PAWDiberhentikan {
		if actor.Role == domain.RoleAdminNasional {
			return true
		}
		return actor.Role == domain.RoleAdminProvinsi &&
			actor.ProvinsiID != nil && sk.ProvinsiID != nil && *actor.ProvinsiID == *sk.ProvinsiID
	}
	return canManageSK(actor, sk)
}

// Paws mengakhiri masa bakti individual pengurus. Aksi MENINGGAL sekaligus
// mengubah status keanggotaan anggota menjadi MENINGGAL.
func (s *pengurusSvc) Paws(ctx context.Context, pengurusID int, in domain.PawsRequest, actor domain.ActorContext, audit domain.AuditContext) error {
	if pengurusID <= 0 {
		return domain.NewValidationError("ID pengurus tidak valid")
	}
	if s.pengurus == nil || s.skRepo == nil {
		return svcutil.Unavailable("kepengurusan")
	}
	action := domain.PengurusPAWAction(strings.ToUpper(strings.TrimSpace(in.Aksi)))
	status, ok := domain.MapPAWAction(action)
	if !ok {
		return domain.NewValidationError("Aksi PAW tidak valid (DEMISIONER/DIBERHENTIKAN/MENGUNDURKAN_DIRI/MENINGGAL)")
	}
	note := strings.TrimSpace(in.Keterangan)
	if note == "" {
		return domain.NewValidationError("Keterangan wajib diisi untuk aksi PAW")
	}
	p, err := s.pengurus.GetByID(ctx, pengurusID)
	if err != nil {
		return err
	}
	if p.Status != string(domain.PengurusStatusAktif) {
		return domain.NewConflictError("Pengurus tidak berstatus aktif")
	}
	sk, err := s.skRepo.GetByID(ctx, p.SuratKeputusanID)
	if err != nil {
		return err
	}
	if !canPaws(actor, sk, action) {
		return domain.NewForbiddenError("Anda tidak berwenang melakukan aksi PAW ini")
	}
	if err := s.pengurus.UpdateStatus(ctx, pengurusID, status, note); err != nil {
		return err
	}
	// Efek ke keanggotaan: meninggal dunia.
	if status == domain.PengurusStatusMeninggal && s.anggotaRepo != nil {
		if err := s.anggotaRepo.SetStatus(ctx, p.AnggotaID, domain.AnggotaStatusMeninggal); err != nil {
			return err
		}
	}
	meta := `{"event":"pengurus_paw","aksi":"` + string(action) + `","to":"` + string(status) + `"}`
	s.audit(ctx, audit, actor, "pengurus", strconv.Itoa(pengurusID), "UPDATE", &meta)
	return nil
}

// Mutasi memindahkan pengurus aktif ke SK/jabatan tujuan (tutup lama → buka
// baru, satu transaksi). Wewenang: pengelola SK tujuan (canManageSK).
func (s *pengurusSvc) Mutasi(ctx context.Context, pengurusID int, in domain.MutasiRequest, actor domain.ActorContext, audit domain.AuditContext) (*domain.PengurusDetail, error) {
	if pengurusID <= 0 || in.SKID <= 0 || in.JabatanID <= 0 {
		return nil, domain.NewValidationError("Data mutasi tidak lengkap")
	}
	if s.pengurus == nil || s.skRepo == nil || s.jabatanRepo == nil {
		return nil, svcutil.Unavailable("kepengurusan")
	}
	src, err := s.pengurus.GetByID(ctx, pengurusID)
	if err != nil {
		return nil, err
	}
	if src.Status != string(domain.PengurusStatusAktif) {
		return nil, domain.NewConflictError("Hanya pengurus aktif yang dapat dimutasi")
	}
	if src.SuratKeputusanID == in.SKID {
		return nil, domain.NewValidationError("Untuk SK yang sama gunakan Ganti Jabatan")
	}
	target, err := s.skRepo.GetByID(ctx, in.SKID)
	if err != nil {
		return nil, err
	}
	if !canManageSK(actor, target) {
		return nil, domain.NewForbiddenError("Anda tidak berwenang mengelola SK tujuan")
	}
	// Persetujuan berjenjang bila lintas tingkat (matriks §8.3): aktor harus
	// berada pada/di atas level tertinggi kedua SK.
	if !canMutasiLintasTingkat(actor, src.Level, string(target.Level)) {
		return nil, domain.NewForbiddenError("Mutasi lintas tingkat wajib diproses admin setingkat lebih tinggi")
	}
	if target.Status != domain.SKStatusAktif {
		return nil, domain.NewValidationError("SK tujuan tidak aktif")
	}
	if target.ApprovalStatus == domain.SKApprovalStatusDisetujui {
		return nil, domain.NewForbiddenError("SK tujuan sudah final. Susunan pengurus terkunci.")
	}
	if strings.TrimSpace(target.FileSKKey) == "" {
		return nil, domain.NewValidationError("SK tujuan belum memiliki file")
	}
	if exists, err := s.pengurus.ExistsInSK(ctx, in.SKID, src.AnggotaID); err != nil {
		return nil, err
	} else if exists {
		return nil, domain.NewConflictError("Anggota sudah tercantum pada SK tujuan")
	}
	jabatan, err := s.jabatanRepo.GetByID(ctx, in.JabatanID)
	if err != nil {
		return nil, err
	}
	if !jabatan.IsActive {
		return nil, domain.NewValidationError("Jabatan tidak aktif")
	}
	if jabatan.IsInti {
		n, err := s.pengurus.CountJabatanInSK(ctx, in.SKID, in.JabatanID, src.AnggotaID)
		if err != nil {
			return nil, err
		}
		if n > 0 {
			return nil, domain.NewConflictError("Jabatan inti " + jabatan.Nama + " sudah terisi pada SK tujuan")
		}
	}
	mulai := time.Now()
	if in.TanggalMulai != nil && !in.TanggalMulai.IsZero() {
		mulai = *in.TanggalMulai
	}
	reason := strings.TrimSpace(in.Keterangan)
	if reason == "" {
		reason = "Mutasi ke " + jabatan.Nama
	}
	newID, err := s.pengurus.Mutate(ctx, repository.MutateInput{
		PengurusID: pengurusID, TargetSKID: in.SKID, Level: string(target.Level),
		ProvinsiID: target.ProvinsiID, KabupatenID: target.KabupatenID,
		JabatanID: in.JabatanID, TanggalMulai: mulai, Keterangan: reason,
	})
	if err != nil {
		return nil, err
	}
	meta := `{"event":"pengurus_mutasi","from_sk":` + strconv.Itoa(src.SuratKeputusanID) + `,"to_sk":` + strconv.Itoa(in.SKID) + `,"jabatan_id":` + strconv.Itoa(in.JabatanID) + `}`
	s.audit(ctx, audit, actor, "pengurus", strconv.Itoa(newID), "UPDATE", &meta)
	out, err := s.pengurus.GetByID(ctx, newID)
	if err != nil {
		return nil, err
	}
	return out, nil
}
