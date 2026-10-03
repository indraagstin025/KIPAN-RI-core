package service

import (
	"context"
	"testing"
	"time"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
)

// ---------- fakes ----------

type fakeJabatanRepo struct {
	created       bool
	list          []domain.Jabatan
	jab           *domain.Jabatan
	pengurusCount int
}

func (f *fakeJabatanRepo) List(context.Context, bool, string) ([]domain.Jabatan, error) {
	return f.list, nil
}
func (f *fakeJabatanRepo) GetByID(context.Context, int) (*domain.Jabatan, error) {
	if f.jab != nil {
		return f.jab, nil
	}
	return nil, domain.ErrNotFound
}
func (f *fakeJabatanRepo) Create(_ context.Context, in domain.JabatanRequest) (*domain.Jabatan, error) {
	f.created = true
	return &domain.Jabatan{ID: 1, Nama: in.Nama}, nil
}
func (f *fakeJabatanRepo) Update(_ context.Context, id int, in domain.JabatanRequest) (*domain.Jabatan, error) {
	return &domain.Jabatan{ID: id, Nama: in.Nama}, nil
}
func (f *fakeJabatanRepo) CountPengurus(context.Context, int) (int, error) {
	return f.pengurusCount, nil
}

type fakeSKRepo struct {
	sk           *domain.SuratKeputusan
	created      *domain.SuratKeputusan
	updatedTo    domain.SKApprovalStatus
	updateHits   int
	finalizeHits int
}

func (f *fakeSKRepo) Create(_ context.Context, sk *domain.SuratKeputusan) (*domain.SuratKeputusan, error) {
	sk.ID = 1
	f.created = sk
	return sk, nil
}
func (f *fakeSKRepo) GetByID(context.Context, int) (*domain.SuratKeputusan, error) {
	if f.sk == nil {
		return nil, domain.ErrNotFound
	}
	return f.sk, nil
}
func (f *fakeSKRepo) List(context.Context, repository.SKFilter) ([]domain.SKListItem, int, error) {
	return nil, 0, nil
}
func (f *fakeSKRepo) UpdateApproval(_ context.Context, _ int, _, to domain.SKApprovalStatus, _ *string, _ *string, _ *time.Time) error {
	f.updatedTo, f.updateHits = to, f.updateHits+1
	return nil
}
func (f *fakeSKRepo) FinalizeSK(_ context.Context, _ int, _ domain.SKApprovalStatus, _ *string, _ *time.Time) error {
	f.finalizeHits++
	f.updatedTo = domain.SKApprovalStatusDisetujui
	return nil
}
func (f *fakeSKRepo) SetStatus(context.Context, int, domain.SKStatus) error { return nil }

type fakePengurusRepo struct {
	detail       *domain.PengurusDetail
	updatedJab   int
	jabatanCount int
	lastFilter   repository.PengurusFilter
}

func (f *fakePengurusRepo) AddWithPromotion(context.Context, repository.PromoteInput) (int, error) {
	return 1, nil
}
func (f *fakePengurusRepo) Remove(context.Context, int) error { return nil }
func (f *fakePengurusRepo) GetByID(context.Context, int) (*domain.PengurusDetail, error) {
	if f.detail == nil {
		return nil, domain.ErrNotFound
	}
	return f.detail, nil
}
func (f *fakePengurusRepo) ListBySK(context.Context, int) ([]domain.PengurusDetail, error) {
	return nil, nil
}
func (f *fakePengurusRepo) List(_ context.Context, in repository.PengurusFilter) ([]domain.PengurusDetail, int, error) {
	f.lastFilter = in
	return nil, 0, nil
}
func (f *fakePengurusRepo) Stats(_ context.Context, in repository.PengurusFilter) (domain.PengurusStats, error) {
	f.lastFilter = in
	return domain.PengurusStats{}, nil
}
func (f *fakePengurusRepo) UpdateStatus(context.Context, int, domain.PengurusStatus, string) error {
	return nil
}
func (f *fakePengurusRepo) UpdateJabatan(_ context.Context, _ int, jabatanID int) error {
	f.updatedJab = jabatanID
	return nil
}
func (f *fakePengurusRepo) ExistsInSK(context.Context, int, int) (bool, error) { return false, nil }
func (f *fakePengurusRepo) CountJabatanInSK(context.Context, int, int, int) (int, error) {
	return f.jabatanCount, nil
}

// ---------- helpers ----------

func kepSvc(sk repository.SKRepository, jab repository.JabatanRepository) KepengurusanService {
	return NewKepengurusanService(nil, KepengurusanDeps{SKRepo: sk, JabatanRepo: jab})
}

func kepSvcFull(sk repository.SKRepository, jab repository.JabatanRepository, pgr repository.PengurusRepository) KepengurusanService {
	return NewKepengurusanService(nil, KepengurusanDeps{SKRepo: sk, JabatanRepo: jab, PengurusRepo: pgr})
}

func kepSvcW(sk repository.SKRepository, jab repository.JabatanRepository, w repository.WilayahRepository) KepengurusanService {
	return NewKepengurusanService(nil, KepengurusanDeps{SKRepo: sk, JabatanRepo: jab, WilayahRepo: w})
}

func kabActor() domain.ActorContext {
	prov, kab := 32, 3273
	return domain.ActorContext{UserID: "u-kab", Name: "Kab", Role: domain.RoleAdminKabupaten, ProvinsiID: &prov, KabupatenID: &kab}
}
func provActor() domain.ActorContext {
	prov := 32
	return domain.ActorContext{UserID: "u-prov", Name: "Prov", Role: domain.RoleAdminProvinsi, ProvinsiID: &prov}
}
func nasActor() domain.ActorContext {
	return domain.ActorContext{UserID: "u-nas", Name: "Nas", Role: domain.RoleAdminNasional}
}

func intPtr(i int) *int   { return &i }
func mustDate() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }
func mustEnd() time.Time  { return time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC) }
func newSKReq(level string, prov, kab *int) domain.SKCreateRequest {
	return domain.SKCreateRequest{
		NomorSK: "001/SK/2026", Judul: "SK Uji", Level: level,
		ProvinsiID: prov, KabupatenID: kab,
		TanggalTerbit: mustDate(), TanggalBerakhir: mustEnd(), FileSKKey: "uploads/sk/a.pdf",
	}
}

// ---------- tests ----------

func TestCreateSKLevelPerRole(t *testing.T) {
	ctx := context.Background()
	w := &fakeWilayahRepo{}
	p, k := 32, 3273

	// Kabupaten -> KABUPATEN / DRAFT (wilayah dipaksa dari akun).
	repoKab := &fakeSKRepo{}
	out, err := kepSvc(repoKab, &fakeJabatanRepo{}).CreateSK(ctx, newSKReq("", nil, nil), kabActor(), domain.AuditContext{})
	if err != nil {
		t.Fatalf("CreateSK Kabupaten gagal: %v", err)
	}
	if out.Level != domain.LevelKabupaten || out.ApprovalStatus != domain.SKApprovalStatusDraft {
		t.Fatalf("Kab: mau KABUPATEN/DRAFT, dapat %s/%s", out.Level, out.ApprovalStatus)
	}
	if out.KabupatenID == nil || out.ProvinsiID == nil {
		t.Fatal("Kab: wilayah harus terisi")
	}

	// Provinsi -> PROVINSI / DRAFT.
	repoProv := &fakeSKRepo{}
	out, err = kepSvc(repoProv, &fakeJabatanRepo{}).CreateSK(ctx, newSKReq("", nil, nil), provActor(), domain.AuditContext{})
	if err != nil {
		t.Fatalf("CreateSK Provinsi gagal: %v", err)
	}
	if out.Level != domain.LevelProvinsi || out.ApprovalStatus != domain.SKApprovalStatusDraft {
		t.Fatalf("Prov: mau PROVINSI/DRAFT, dapat %s/%s", out.Level, out.ApprovalStatus)
	}
	if out.KabupatenID != nil || out.ProvinsiID == nil {
		t.Fatal("Prov: kabupaten harus nil, provinsi terisi")
	}

	// Nasional pilih NASIONAL -> NASIONAL / DRAFT (tidak langsung final).
	repoNas := &fakeSKRepo{}
	out, err = kepSvcW(repoNas, &fakeJabatanRepo{}, w).CreateSK(ctx, newSKReq("NASIONAL", nil, nil), nasActor(), domain.AuditContext{})
	if err != nil {
		t.Fatalf("CreateSK Nasional gagal: %v", err)
	}
	if out.Level != domain.LevelNasional || out.ApprovalStatus != domain.SKApprovalStatusDraft {
		t.Fatalf("Nas: mau NASIONAL/DRAFT, dapat %s/%s", out.Level, out.ApprovalStatus)
	}
	if out.ProvinsiID != nil || out.KabupatenID != nil {
		t.Fatal("Nas: wilayah harus nil")
	}

	// Nasional pilih KABUPATEN (wilayah valid) -> KABUPATEN / DRAFT.
	repoNasKab := &fakeSKRepo{}
	out, err = kepSvcW(repoNasKab, &fakeJabatanRepo{}, w).CreateSK(ctx, newSKReq("KABUPATEN", &p, &k), nasActor(), domain.AuditContext{})
	if err != nil {
		t.Fatalf("CreateSK Nasional KABUPATEN gagal: %v", err)
	}
	if out.Level != domain.LevelKabupaten || out.ApprovalStatus != domain.SKApprovalStatusDraft || out.TanggalBerakhir == nil {
		t.Fatalf("Nas-Kab: mau KABUPATEN/DRAFT + tanggal_berakhir, dapat %s/%s", out.Level, out.ApprovalStatus)
	}

	// Super pilih PROVINSI -> PROVINSI / DRAFT.
	repoSup := &fakeSKRepo{}
	out, err = kepSvcW(repoSup, &fakeJabatanRepo{}, w).CreateSK(ctx, newSKReq("PROVINSI", &p, nil), superActor(), domain.AuditContext{})
	if err != nil {
		t.Fatalf("CreateSK Super PROVINSI gagal: %v", err)
	}
	if out.Level != domain.LevelProvinsi {
		t.Fatalf("Super-Prov: mau PROVINSI, dapat %s", out.Level)
	}

	// Level tidak valid -> error.
	if _, err := kepSvcW(&fakeSKRepo{}, &fakeJabatanRepo{}, w).CreateSK(ctx, newSKReq("XYZ", nil, nil), nasActor(), domain.AuditContext{}); err == nil {
		t.Fatal("level tidak valid seharusnya ditolak")
	}
}

func TestCreateSKValidasiWajib(t *testing.T) {
	ctx := context.Background()
	svc := kepSvc(&fakeSKRepo{}, &fakeJabatanRepo{})

	// File SK kosong.
	req := newSKReq("", nil, nil)
	req.FileSKKey = "  "
	if _, err := svc.CreateSK(ctx, req, kabActor(), domain.AuditContext{}); err == nil {
		t.Fatal("CreateSK tanpa file seharusnya ditolak")
	}

	// Tanggal berakhir kosong.
	req = newSKReq("", nil, nil)
	req.TanggalBerakhir = time.Time{}
	if _, err := svc.CreateSK(ctx, req, kabActor(), domain.AuditContext{}); err == nil {
		t.Fatal("CreateSK tanpa tanggal berakhir seharusnya ditolak")
	}

	// Tanggal berakhir tidak setelah terbit.
	req = newSKReq("", nil, nil)
	req.TanggalBerakhir = mustDate()
	if _, err := svc.CreateSK(ctx, req, kabActor(), domain.AuditContext{}); err == nil {
		t.Fatal("CreateSK dengan berakhir <= terbit seharusnya ditolak")
	}
}

func TestApproveSKAjukan(t *testing.T) {
	ctx := context.Background()

	// Kabupaten ajukan SK Kabupaten -> MENUNGGU_PROVINSI.
	kabSK := &domain.SuratKeputusan{ID: 1, Level: domain.LevelKabupaten, ProvinsiID: intPtr(32), KabupatenID: intPtr(3273), Status: domain.SKStatusAktif, ApprovalStatus: domain.SKApprovalStatusDraft}
	repo := &fakeSKRepo{sk: kabSK}
	if err := kepSvc(repo, &fakeJabatanRepo{}).ApproveSK(ctx, 1, domain.SKActionAjukan, "", kabActor(), domain.AuditContext{}); err != nil {
		t.Fatalf("ajukan kabupaten gagal: %v", err)
	}
	if repo.updatedTo != domain.SKApprovalStatusMenungguProvinsi || repo.updateHits != 1 {
		t.Fatalf("kab: mau MENUNGGU_PROVINSI, dapat %s (hits=%d)", repo.updatedTo, repo.updateHits)
	}

	// Provinsi ajukan SK Provinsi -> MENUNGGU_NASIONAL.
	provSK := &domain.SuratKeputusan{ID: 2, Level: domain.LevelProvinsi, ProvinsiID: intPtr(32), Status: domain.SKStatusAktif, ApprovalStatus: domain.SKApprovalStatusDraft}
	repoP := &fakeSKRepo{sk: provSK}
	if err := kepSvc(repoP, &fakeJabatanRepo{}).ApproveSK(ctx, 2, domain.SKActionAjukan, "", provActor(), domain.AuditContext{}); err != nil {
		t.Fatalf("ajukan provinsi gagal: %v", err)
	}
	if repoP.updatedTo != domain.SKApprovalStatusMenungguNasional {
		t.Fatalf("prov: mau MENUNGGU_NASIONAL, dapat %s", repoP.updatedTo)
	}

	// Nasional ajukan SK Nasional -> final (FinalizeSK + Single Active).
	nasSK := &domain.SuratKeputusan{ID: 3, Level: domain.LevelNasional, Status: domain.SKStatusAktif, ApprovalStatus: domain.SKApprovalStatusDraft}
	repoN := &fakeSKRepo{sk: nasSK}
	if err := kepSvc(repoN, &fakeJabatanRepo{}).ApproveSK(ctx, 3, domain.SKActionAjukan, "", nasActor(), domain.AuditContext{}); err != nil {
		t.Fatalf("ajukan nasional gagal: %v", err)
	}
	if repoN.finalizeHits != 1 {
		t.Fatalf("Nasional harus memakai FinalizeSK, hits=%d", repoN.finalizeHits)
	}

	// Provinsi TIDAK boleh ajukan SK Kabupaten.
	repoX := &fakeSKRepo{sk: &domain.SuratKeputusan{ID: 4, Level: domain.LevelKabupaten, ProvinsiID: intPtr(32), KabupatenID: intPtr(3273), ApprovalStatus: domain.SKApprovalStatusDraft}}
	if err := kepSvc(repoX, &fakeJabatanRepo{}).ApproveSK(ctx, 4, domain.SKActionAjukan, "", provActor(), domain.AuditContext{}); err == nil {
		t.Fatal("Provinsi tidak boleh ajukan SK Kabupaten")
	}

	// Ajukan saat status bukan DRAFT -> konflik.
	repoY := &fakeSKRepo{sk: &domain.SuratKeputusan{ID: 5, Level: domain.LevelKabupaten, ProvinsiID: intPtr(32), KabupatenID: intPtr(3273), ApprovalStatus: domain.SKApprovalStatusMenungguProvinsi}}
	if err := kepSvc(repoY, &fakeJabatanRepo{}).ApproveSK(ctx, 5, domain.SKActionAjukan, "", kabActor(), domain.AuditContext{}); err == nil {
		t.Fatal("ajukan saat status bukan DRAFT seharusnya ditolak")
	}
}

func TestCanManageSKPerLevel(t *testing.T) {
	kabSK := &domain.SuratKeputusan{Level: domain.LevelKabupaten, ProvinsiID: intPtr(32), KabupatenID: intPtr(3273)}
	provSK := &domain.SuratKeputusan{Level: domain.LevelProvinsi, ProvinsiID: intPtr(32)}
	nasSK := &domain.SuratKeputusan{Level: domain.LevelNasional}

	if !canManageSK(kabActor(), kabSK) {
		t.Fatal("Kabupaten harus boleh kelola SK Kabupaten sekab")
	}
	// Opsi A: Admin Provinsi boleh kelola pengurus SK Kabupaten di provinsinya.
	if !canManageSK(provActor(), kabSK) {
		t.Fatal("Opsi A: Provinsi harus boleh kelola SK Kabupaten seprov")
	}
	if canManageSK(nasActor(), kabSK) {
		t.Fatal("Nasional TIDAK boleh kelola SK Kabupaten (per level)")
	}
	if !canManageSK(provActor(), provSK) {
		t.Fatal("Provinsi harus boleh kelola SK Provinsi seprov")
	}
	if canManageSK(kabActor(), provSK) {
		t.Fatal("Kabupaten TIDAK boleh kelola SK Provinsi")
	}
	if !canManageSK(nasActor(), nasSK) {
		t.Fatal("Nasional harus boleh kelola SK Nasional")
	}
	if !canManageSK(superActor(), kabSK) || !canManageSK(superActor(), nasSK) {
		t.Fatal("Super harus oversight semua level")
	}
}

func TestAddPengurusOtorisasi(t *testing.T) {
	ctx := context.Background()
	sk := &domain.SuratKeputusan{
		ID: 1, Level: domain.LevelKabupaten, ProvinsiID: intPtr(32), KabupatenID: intPtr(3273),
		Status: domain.SKStatusAktif, ApprovalStatus: domain.SKApprovalStatusMenungguProvinsi, FileSKKey: "uploads/sk/a.pdf",
	}
	in := domain.AddPengurusRequest{AnggotaID: 1, JabatanID: 1, Konfirmasi: true}
	svc := kepSvc(&fakeSKRepo{sk: sk}, &fakeJabatanRepo{})

	if _, err := svc.AddPengurus(ctx, 1, in, provActor(), domain.AuditContext{}); err == nil {
		t.Fatal("Provinsi tidak boleh mengelola pengurus SK Kabupaten")
	}
	if _, err := svc.AddPengurus(ctx, 1, domain.AddPengurusRequest{AnggotaID: 1, JabatanID: 1, Konfirmasi: false}, kabActor(), domain.AuditContext{}); err == nil {
		t.Fatal("AddPengurus tanpa konfirmasi seharusnya ditolak")
	}
}

func TestApproveSKRantaiPeran(t *testing.T) {
	ctx := context.Background()

	// TERUSKAN oleh Nasional ditolak (harus Provinsi wilayah SK).
	repo := &fakeSKRepo{sk: &domain.SuratKeputusan{ID: 1, ProvinsiID: intPtr(32), ApprovalStatus: domain.SKApprovalStatusMenungguProvinsi}}
	svc := kepSvc(repo, &fakeJabatanRepo{})
	if err := svc.ApproveSK(ctx, 1, domain.SKActionTeruskan, "", nasActor(), domain.AuditContext{}); err == nil {
		t.Fatal("TERUSKAN oleh Nasional seharusnya ditolak")
	}

	// SAHKAN oleh Provinsi ditolak (harus Nasional/Super).
	repo2 := &fakeSKRepo{sk: &domain.SuratKeputusan{ID: 1, ProvinsiID: intPtr(32), ApprovalStatus: domain.SKApprovalStatusMenungguNasional}}
	svc2 := kepSvc(repo2, &fakeJabatanRepo{})
	if err := svc2.ApproveSK(ctx, 1, domain.SKActionSahkan, "", provActor(), domain.AuditContext{}); err == nil {
		t.Fatal("SAHKAN oleh Provinsi seharusnya ditolak")
	}

	// TOLAK oleh Nasional tanpa catatan => validasi.
	if err := svc2.ApproveSK(ctx, 1, domain.SKActionTolak, "  ", nasActor(), domain.AuditContext{}); err == nil {
		t.Fatal("TOLAK tanpa catatan seharusnya ditolak")
	}

	// SAHKAN oleh Nasional pada tahap MENUNGGU_NASIONAL => FinalizeSK (Single Active).
	if err := svc2.ApproveSK(ctx, 1, domain.SKActionSahkan, "", nasActor(), domain.AuditContext{}); err != nil {
		t.Fatalf("SAHKAN oleh Nasional gagal: %v", err)
	}
	if repo2.finalizeHits != 1 {
		t.Fatalf("FinalizeSK harus dipanggil sekali (Single Active), hits=%d", repo2.finalizeHits)
	}
}

func TestUpdatePengurusJabatan(t *testing.T) {
	ctx := context.Background()
	baseSK := func(final bool) *domain.SuratKeputusan {
		ap := domain.SKApprovalStatusMenungguProvinsi
		if final {
			ap = domain.SKApprovalStatusDisetujui
		}
		return &domain.SuratKeputusan{
			ID: 1, Level: domain.LevelKabupaten, ProvinsiID: intPtr(32), KabupatenID: intPtr(3273),
			Status: domain.SKStatusAktif, ApprovalStatus: ap, FileSKKey: "uploads/sk/a.pdf",
		}
	}
	detail := &domain.PengurusDetail{ID: 5, SuratKeputusanID: 1, JabatanID: 1, AnggotaID: 9,
		Level: string(domain.LevelKabupaten), ProvinsiID: intPtr(32), KabupatenID: intPtr(3273)}

	// Sukses ganti jabatan non-inti.
	pgr := &fakePengurusRepo{detail: detail}
	jab := &fakeJabatanRepo{jab: &domain.Jabatan{ID: 2, Nama: "Sekretaris", Level: domain.LevelKabupaten, IsActive: true}}
	svc := kepSvcFull(&fakeSKRepo{sk: baseSK(false)}, jab, pgr)
	if _, err := svc.UpdatePengurusJabatan(ctx, 5, domain.UpdateJabatanRequest{JabatanID: 2}, kabActor(), domain.AuditContext{}); err != nil {
		t.Fatalf("ganti jabatan gagal: %v", err)
	}
	if pgr.updatedJab != 2 {
		t.Fatalf("jabatan tidak ter-update, dapat %d", pgr.updatedJab)
	}

	// Lock final: SK DISETUJUI => 403.
	svcFinal := kepSvcFull(&fakeSKRepo{sk: baseSK(true)}, jab, &fakePengurusRepo{detail: detail})
	if _, err := svcFinal.UpdatePengurusJabatan(ctx, 5, domain.UpdateJabatanRequest{JabatanID: 2}, kabActor(), domain.AuditContext{}); err == nil {
		t.Fatal("ganti jabatan pada SK final seharusnya ditolak")
	}

	// Jabatan inti ganda => 409.
	jabInti := &fakeJabatanRepo{jab: &domain.Jabatan{ID: 3, Nama: "Ketua", Level: domain.LevelKabupaten, IsInti: true, IsActive: true}}
	pgrInti := &fakePengurusRepo{detail: detail, jabatanCount: 1}
	svcInti := kepSvcFull(&fakeSKRepo{sk: baseSK(false)}, jabInti, pgrInti)
	if _, err := svcInti.UpdatePengurusJabatan(ctx, 5, domain.UpdateJabatanRequest{JabatanID: 3}, kabActor(), domain.AuditContext{}); err == nil {
		t.Fatal("jabatan inti ganda seharusnya ditolak")
	}

	// Opsi A: Provinsi BOLEH ganti jabatan pengurus SK Kabupaten seprov.
	if _, err := svc.UpdatePengurusJabatan(ctx, 5, domain.UpdateJabatanRequest{JabatanID: 2}, provActor(), domain.AuditContext{}); err != nil {
		t.Fatalf("Opsi A: Provinsi harus boleh kelola SK Kabupaten seprov: %v", err)
	}

	// Provinsi BEDA provinsi TIDAK boleh kelola SK Kabupaten.
	otherProv := 99
	otherActor := domain.ActorContext{UserID: "u-other", Role: domain.RoleAdminProvinsi, ProvinsiID: &otherProv}
	if _, err := svc.UpdatePengurusJabatan(ctx, 5, domain.UpdateJabatanRequest{JabatanID: 2}, otherActor, domain.AuditContext{}); err == nil {
		t.Fatal("Provinsi beda wilayah tidak boleh kelola SK Kabupaten")
	}
}

func TestCreateJabatanHanyaNasionalSuper(t *testing.T) {
	ctx := context.Background()
	in := domain.JabatanRequest{Nama: "Ketua", Level: "NASIONAL", IsInti: true, IsActive: true, Urutan: 1}

	if _, err := kepSvc(nil, &fakeJabatanRepo{}).CreateJabatan(ctx, in, provActor(), domain.AuditContext{}); err == nil {
		t.Fatal("CreateJabatan oleh Provinsi seharusnya ditolak")
	}
	repo := &fakeJabatanRepo{}
	if _, err := kepSvc(nil, repo).CreateJabatan(ctx, in, nasActor(), domain.AuditContext{}); err != nil {
		t.Fatalf("CreateJabatan oleh Nasional gagal: %v", err)
	}
	if !repo.created {
		t.Fatal("jabatan tidak tersimpan")
	}
}

func TestListPengurusScopeAndFilter(t *testing.T) {
	ctx := context.Background()
	pgr := &fakePengurusRepo{}
	svc := kepSvcFull(&fakeSKRepo{}, &fakeJabatanRepo{}, pgr)

	// Kabupaten: scope prov+kab, level di-uppercase, masa diteruskan.
	if _, _, err := svc.ListPengurus(ctx, kabActor(), "kabupaten", "", "AkanBerakhir", "budi", nil, nil, true, 1, 10); err != nil {
		t.Fatalf("ListPengurus gagal: %v", err)
	}
	if pgr.lastFilter.ProvinsiID == nil || *pgr.lastFilter.ProvinsiID != 32 ||
		pgr.lastFilter.KabupatenID == nil || *pgr.lastFilter.KabupatenID != 3273 {
		t.Fatalf("scope kabupaten salah: %+v", pgr.lastFilter)
	}
	if pgr.lastFilter.Level != "KABUPATEN" || pgr.lastFilter.MasaJabatan != "AkanBerakhir" {
		t.Fatalf("filter level/masa salah: %+v", pgr.lastFilter)
	}

	// Provinsi: hanya provinsi (kabupaten nil) untuk stats.
	if _, err := svc.PengurusStats(ctx, provActor()); err != nil {
		t.Fatalf("PengurusStats gagal: %v", err)
	}
	if pgr.lastFilter.ProvinsiID == nil || *pgr.lastFilter.ProvinsiID != 32 || pgr.lastFilter.KabupatenID != nil {
		t.Fatalf("scope stats provinsi salah: %+v", pgr.lastFilter)
	}
}

func TestScopeForActor(t *testing.T) {
	prov, kab := 32, 3273
	kabCtx := domain.ActorContext{Role: domain.RoleAdminKabupaten, ProvinsiID: &prov, KabupatenID: &kab}
	p, k := scopeForActor(kabCtx)
	if p == nil || k == nil || *p != prov || *k != kab {
		t.Fatalf("scope kabupaten salah: %v %v", p, k)
	}
	p, k = scopeForActor(provActor())
	if p == nil || *p != prov || k != nil {
		t.Fatalf("scope provinsi salah: %v %v", p, k)
	}
	p, k = scopeForActor(nasActor())
	if p != nil || k != nil {
		t.Fatalf("scope nasional harus nil: %v %v", p, k)
	}
}
