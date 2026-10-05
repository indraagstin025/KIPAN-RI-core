package service

import (
	"context"
	"testing"
	"time"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
	"github.com/kipan-indonesia/sim-kipan-core/internal/service/testutil"
)

// ---------- fakes ----------

type fakeJabatanRepo struct {
	created       bool
	list          []domain.Jabatan
	jab           *domain.Jabatan
	pengurusCount int
}

func (f *fakeJabatanRepo) List(context.Context, bool) ([]domain.Jabatan, error) {
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
func (f *fakeSKRepo) SetStatusWithDemotion(context.Context, int, domain.SKStatus, domain.PengurusStatus, string) error {
	return nil
}

type fakePengurusRepo struct {
	detail       *domain.PengurusDetail
	byAnggota    []domain.PengurusDetail
	updatedJab   int
	jabatanCount int
	lastFilter   repository.PengurusFilter
	lastStatus   domain.PengurusStatus
}

func (f *fakePengurusRepo) AddWithPromotion(context.Context, repository.PromoteInput) (int, error) {
	return 1, nil
}
func (f *fakePengurusRepo) Mutate(context.Context, repository.MutateInput) (int, error) {
	return 2, nil
}
func (f *fakePengurusRepo) CloseExpiredAppointments(context.Context) ([]domain.ExpiredAppointment, error) {
	return nil, nil
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
func (f *fakePengurusRepo) ListByAnggota(context.Context, int) ([]domain.PengurusDetail, error) {
	return f.byAnggota, nil
}
func (f *fakePengurusRepo) List(_ context.Context, in repository.PengurusFilter) ([]domain.PengurusDetail, int, error) {
	f.lastFilter = in
	return nil, 0, nil
}
func (f *fakePengurusRepo) Stats(_ context.Context, in repository.PengurusFilter) (domain.PengurusStats, error) {
	f.lastFilter = in
	return domain.PengurusStats{}, nil
}
func (f *fakePengurusRepo) ListPromosi(context.Context, *int, *int, string, int) ([]domain.PromosiCandidate, error) {
	return nil, nil
}
func (f *fakePengurusRepo) UpdateStatus(_ context.Context, _ int, status domain.PengurusStatus, _ string) error {
	f.lastStatus = status
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
	w := &testutil.FakeWilayahRepo{}
	p, k := 32, 3273

	// Kabupaten -> KABUPATEN / DRAFT (wilayah dipaksa dari akun).
	repoKab := &fakeSKRepo{}
	out, err := kepSvc(repoKab, &fakeJabatanRepo{}).CreateSK(ctx, newSKReq("", nil, nil), testutil.KabActor(), domain.AuditContext{})
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
	out, err = kepSvc(repoProv, &fakeJabatanRepo{}).CreateSK(ctx, newSKReq("", nil, nil), testutil.ProvActor(), domain.AuditContext{})
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
	out, err = kepSvcW(repoNas, &fakeJabatanRepo{}, w).CreateSK(ctx, newSKReq("NASIONAL", nil, nil), testutil.NasActor(), domain.AuditContext{})
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
	out, err = kepSvcW(repoNasKab, &fakeJabatanRepo{}, w).CreateSK(ctx, newSKReq("KABUPATEN", &p, &k), testutil.NasActor(), domain.AuditContext{})
	if err != nil {
		t.Fatalf("CreateSK Nasional KABUPATEN gagal: %v", err)
	}
	if out.Level != domain.LevelKabupaten || out.ApprovalStatus != domain.SKApprovalStatusDraft || out.TanggalBerakhir == nil {
		t.Fatalf("Nas-Kab: mau KABUPATEN/DRAFT + tanggal_berakhir, dapat %s/%s", out.Level, out.ApprovalStatus)
	}

	// Super pilih PROVINSI -> PROVINSI / DRAFT.
	repoSup := &fakeSKRepo{}
	out, err = kepSvcW(repoSup, &fakeJabatanRepo{}, w).CreateSK(ctx, newSKReq("PROVINSI", &p, nil), testutil.SuperActor(), domain.AuditContext{})
	if err != nil {
		t.Fatalf("CreateSK Super PROVINSI gagal: %v", err)
	}
	if out.Level != domain.LevelProvinsi {
		t.Fatalf("Super-Prov: mau PROVINSI, dapat %s", out.Level)
	}

	// Level tidak valid -> error.
	if _, err := kepSvcW(&fakeSKRepo{}, &fakeJabatanRepo{}, w).CreateSK(ctx, newSKReq("XYZ", nil, nil), testutil.NasActor(), domain.AuditContext{}); err == nil {
		t.Fatal("level tidak valid seharusnya ditolak")
	}
}

func TestCreateSKValidasiWajib(t *testing.T) {
	ctx := context.Background()
	svc := kepSvc(&fakeSKRepo{}, &fakeJabatanRepo{})

	// File SK kosong.
	req := newSKReq("", nil, nil)
	req.FileSKKey = "  "
	if _, err := svc.CreateSK(ctx, req, testutil.KabActor(), domain.AuditContext{}); err == nil {
		t.Fatal("CreateSK tanpa file seharusnya ditolak")
	}

	// Tanggal berakhir kosong.
	req = newSKReq("", nil, nil)
	req.TanggalBerakhir = time.Time{}
	if _, err := svc.CreateSK(ctx, req, testutil.KabActor(), domain.AuditContext{}); err == nil {
		t.Fatal("CreateSK tanpa tanggal berakhir seharusnya ditolak")
	}

	// Tanggal berakhir tidak setelah terbit.
	req = newSKReq("", nil, nil)
	req.TanggalBerakhir = mustDate()
	if _, err := svc.CreateSK(ctx, req, testutil.KabActor(), domain.AuditContext{}); err == nil {
		t.Fatal("CreateSK dengan berakhir <= terbit seharusnya ditolak")
	}
}

func TestApproveSKAjukan(t *testing.T) {
	ctx := context.Background()

	// Kabupaten ajukan SK Kabupaten -> MENUNGGU_PROVINSI.
	kabSK := &domain.SuratKeputusan{ID: 1, Level: domain.LevelKabupaten, ProvinsiID: testutil.IntPtr(32), KabupatenID: testutil.IntPtr(3273), Status: domain.SKStatusAktif, ApprovalStatus: domain.SKApprovalStatusDraft}
	repo := &fakeSKRepo{sk: kabSK}
	if err := kepSvc(repo, &fakeJabatanRepo{}).ApproveSK(ctx, 1, domain.SKActionAjukan, "", testutil.KabActor(), domain.AuditContext{}); err != nil {
		t.Fatalf("ajukan kabupaten gagal: %v", err)
	}
	if repo.updatedTo != domain.SKApprovalStatusMenungguProvinsi || repo.updateHits != 1 {
		t.Fatalf("kab: mau MENUNGGU_PROVINSI, dapat %s (hits=%d)", repo.updatedTo, repo.updateHits)
	}

	// Provinsi ajukan SK Provinsi -> MENUNGGU_NASIONAL.
	provSK := &domain.SuratKeputusan{ID: 2, Level: domain.LevelProvinsi, ProvinsiID: testutil.IntPtr(32), Status: domain.SKStatusAktif, ApprovalStatus: domain.SKApprovalStatusDraft}
	repoP := &fakeSKRepo{sk: provSK}
	if err := kepSvc(repoP, &fakeJabatanRepo{}).ApproveSK(ctx, 2, domain.SKActionAjukan, "", testutil.ProvActor(), domain.AuditContext{}); err != nil {
		t.Fatalf("ajukan provinsi gagal: %v", err)
	}
	if repoP.updatedTo != domain.SKApprovalStatusMenungguNasional {
		t.Fatalf("prov: mau MENUNGGU_NASIONAL, dapat %s", repoP.updatedTo)
	}

	// Nasional ajukan SK Nasional -> final (FinalizeSK + Single Active).
	nasSK := &domain.SuratKeputusan{ID: 3, Level: domain.LevelNasional, Status: domain.SKStatusAktif, ApprovalStatus: domain.SKApprovalStatusDraft}
	repoN := &fakeSKRepo{sk: nasSK}
	if err := kepSvc(repoN, &fakeJabatanRepo{}).ApproveSK(ctx, 3, domain.SKActionAjukan, "", testutil.NasActor(), domain.AuditContext{}); err != nil {
		t.Fatalf("ajukan nasional gagal: %v", err)
	}
	if repoN.finalizeHits != 1 {
		t.Fatalf("Nasional harus memakai FinalizeSK, hits=%d", repoN.finalizeHits)
	}

	// Provinsi TIDAK boleh ajukan SK Kabupaten.
	repoX := &fakeSKRepo{sk: &domain.SuratKeputusan{ID: 4, Level: domain.LevelKabupaten, ProvinsiID: testutil.IntPtr(32), KabupatenID: testutil.IntPtr(3273), ApprovalStatus: domain.SKApprovalStatusDraft}}
	if err := kepSvc(repoX, &fakeJabatanRepo{}).ApproveSK(ctx, 4, domain.SKActionAjukan, "", testutil.ProvActor(), domain.AuditContext{}); err == nil {
		t.Fatal("Provinsi tidak boleh ajukan SK Kabupaten")
	}

	// Ajukan saat status bukan DRAFT -> konflik.
	repoY := &fakeSKRepo{sk: &domain.SuratKeputusan{ID: 5, Level: domain.LevelKabupaten, ProvinsiID: testutil.IntPtr(32), KabupatenID: testutil.IntPtr(3273), ApprovalStatus: domain.SKApprovalStatusMenungguProvinsi}}
	if err := kepSvc(repoY, &fakeJabatanRepo{}).ApproveSK(ctx, 5, domain.SKActionAjukan, "", testutil.KabActor(), domain.AuditContext{}); err == nil {
		t.Fatal("ajukan saat status bukan DRAFT seharusnya ditolak")
	}
}

func TestCanManageSKPerLevel(t *testing.T) {
	kabSK := &domain.SuratKeputusan{Level: domain.LevelKabupaten, ProvinsiID: testutil.IntPtr(32), KabupatenID: testutil.IntPtr(3273)}
	provSK := &domain.SuratKeputusan{Level: domain.LevelProvinsi, ProvinsiID: testutil.IntPtr(32)}
	nasSK := &domain.SuratKeputusan{Level: domain.LevelNasional}

	if !canManageSK(testutil.KabActor(), kabSK) {
		t.Fatal("Kabupaten harus boleh kelola SK Kabupaten sekab")
	}
	// Opsi A: Admin Provinsi boleh kelola pengurus SK Kabupaten di provinsinya.
	if !canManageSK(testutil.ProvActor(), kabSK) {
		t.Fatal("Opsi A: Provinsi harus boleh kelola SK Kabupaten seprov")
	}
	if canManageSK(testutil.NasActor(), kabSK) {
		t.Fatal("Nasional TIDAK boleh kelola SK Kabupaten (per level)")
	}
	if !canManageSK(testutil.ProvActor(), provSK) {
		t.Fatal("Provinsi harus boleh kelola SK Provinsi seprov")
	}
	if canManageSK(testutil.KabActor(), provSK) {
		t.Fatal("Kabupaten TIDAK boleh kelola SK Provinsi")
	}
	if !canManageSK(testutil.NasActor(), nasSK) {
		t.Fatal("Nasional harus boleh kelola SK Nasional")
	}
	if !canManageSK(testutil.SuperActor(), kabSK) || !canManageSK(testutil.SuperActor(), nasSK) {
		t.Fatal("Super harus oversight semua level")
	}
}

func TestAddPengurusOtorisasi(t *testing.T) {
	ctx := context.Background()
	sk := &domain.SuratKeputusan{
		ID: 1, Level: domain.LevelKabupaten, ProvinsiID: testutil.IntPtr(32), KabupatenID: testutil.IntPtr(3273),
		Status: domain.SKStatusAktif, ApprovalStatus: domain.SKApprovalStatusMenungguProvinsi, FileSKKey: "uploads/sk/a.pdf",
	}
	in := domain.AddPengurusRequest{AnggotaID: 1, JabatanID: 1, Konfirmasi: true}
	svc := kepSvc(&fakeSKRepo{sk: sk}, &fakeJabatanRepo{})

	if _, err := svc.AddPengurus(ctx, 1, in, testutil.ProvActor(), domain.AuditContext{}); err == nil {
		t.Fatal("Provinsi tidak boleh mengelola pengurus SK Kabupaten")
	}
	if _, err := svc.AddPengurus(ctx, 1, domain.AddPengurusRequest{AnggotaID: 1, JabatanID: 1, Konfirmasi: false}, testutil.KabActor(), domain.AuditContext{}); err == nil {
		t.Fatal("AddPengurus tanpa konfirmasi seharusnya ditolak")
	}
}

func TestApproveSKRantaiPeran(t *testing.T) {
	ctx := context.Background()

	// TERUSKAN oleh Nasional ditolak (harus Provinsi wilayah SK).
	repo := &fakeSKRepo{sk: &domain.SuratKeputusan{ID: 1, ProvinsiID: testutil.IntPtr(32), ApprovalStatus: domain.SKApprovalStatusMenungguProvinsi}}
	svc := kepSvc(repo, &fakeJabatanRepo{})
	if err := svc.ApproveSK(ctx, 1, domain.SKActionTeruskan, "", testutil.NasActor(), domain.AuditContext{}); err == nil {
		t.Fatal("TERUSKAN oleh Nasional seharusnya ditolak")
	}

	// SAHKAN oleh Provinsi ditolak (harus Nasional/Super).
	repo2 := &fakeSKRepo{sk: &domain.SuratKeputusan{ID: 1, ProvinsiID: testutil.IntPtr(32), ApprovalStatus: domain.SKApprovalStatusMenungguNasional}}
	svc2 := kepSvc(repo2, &fakeJabatanRepo{})
	if err := svc2.ApproveSK(ctx, 1, domain.SKActionSahkan, "", testutil.ProvActor(), domain.AuditContext{}); err == nil {
		t.Fatal("SAHKAN oleh Provinsi seharusnya ditolak")
	}

	// TOLAK oleh Nasional tanpa catatan => validasi.
	if err := svc2.ApproveSK(ctx, 1, domain.SKActionTolak, "  ", testutil.NasActor(), domain.AuditContext{}); err == nil {
		t.Fatal("TOLAK tanpa catatan seharusnya ditolak")
	}

	// SAHKAN oleh Nasional pada tahap MENUNGGU_NASIONAL => FinalizeSK (Single Active).
	if err := svc2.ApproveSK(ctx, 1, domain.SKActionSahkan, "", testutil.NasActor(), domain.AuditContext{}); err != nil {
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
			ID: 1, Level: domain.LevelKabupaten, ProvinsiID: testutil.IntPtr(32), KabupatenID: testutil.IntPtr(3273),
			Status: domain.SKStatusAktif, ApprovalStatus: ap, FileSKKey: "uploads/sk/a.pdf",
		}
	}
	detail := &domain.PengurusDetail{ID: 5, SuratKeputusanID: 1, JabatanID: 1, AnggotaID: 9,
		Level: string(domain.LevelKabupaten), ProvinsiID: testutil.IntPtr(32), KabupatenID: testutil.IntPtr(3273)}

	// Sukses ganti jabatan non-inti.
	pgr := &fakePengurusRepo{detail: detail}
	jab := &fakeJabatanRepo{jab: &domain.Jabatan{ID: 2, Nama: "Sekretaris", IsActive: true}}
	svc := kepSvcFull(&fakeSKRepo{sk: baseSK(false)}, jab, pgr)
	if _, err := svc.UpdatePengurusJabatan(ctx, 5, domain.UpdateJabatanRequest{JabatanID: 2}, testutil.KabActor(), domain.AuditContext{}); err != nil {
		t.Fatalf("ganti jabatan gagal: %v", err)
	}
	if pgr.updatedJab != 2 {
		t.Fatalf("jabatan tidak ter-update, dapat %d", pgr.updatedJab)
	}

	// Lock final: SK DISETUJUI => 403.
	svcFinal := kepSvcFull(&fakeSKRepo{sk: baseSK(true)}, jab, &fakePengurusRepo{detail: detail})
	if _, err := svcFinal.UpdatePengurusJabatan(ctx, 5, domain.UpdateJabatanRequest{JabatanID: 2}, testutil.KabActor(), domain.AuditContext{}); err == nil {
		t.Fatal("ganti jabatan pada SK final seharusnya ditolak")
	}

	// Jabatan inti ganda => 409.
	jabInti := &fakeJabatanRepo{jab: &domain.Jabatan{ID: 3, Nama: "Ketua", IsInti: true, IsActive: true}}
	pgrInti := &fakePengurusRepo{detail: detail, jabatanCount: 1}
	svcInti := kepSvcFull(&fakeSKRepo{sk: baseSK(false)}, jabInti, pgrInti)
	if _, err := svcInti.UpdatePengurusJabatan(ctx, 5, domain.UpdateJabatanRequest{JabatanID: 3}, testutil.KabActor(), domain.AuditContext{}); err == nil {
		t.Fatal("jabatan inti ganda seharusnya ditolak")
	}

	// Opsi A: Provinsi BOLEH ganti jabatan pengurus SK Kabupaten seprov.
	if _, err := svc.UpdatePengurusJabatan(ctx, 5, domain.UpdateJabatanRequest{JabatanID: 2}, testutil.ProvActor(), domain.AuditContext{}); err != nil {
		t.Fatalf("Opsi A: Provinsi harus boleh kelola SK Kabupaten seprov: %v", err)
	}

	// Provinsi BEDA provinsi TIDAK boleh kelola SK Kabupaten.
	otherProv := 99
	otherActor := domain.ActorContext{UserID: "u-other", Role: domain.RoleAdminProvinsi, ProvinsiID: &otherProv}
	if _, err := svc.UpdatePengurusJabatan(ctx, 5, domain.UpdateJabatanRequest{JabatanID: 2}, otherActor, domain.AuditContext{}); err == nil {
		t.Fatal("Provinsi beda wilayah tidak boleh kelola SK Kabupaten")
	}
}

func TestCreateJabatanSemuaAdmin(t *testing.T) {
	ctx := context.Background()
	in := domain.JabatanRequest{Nama: "Ketua", IsInti: true, IsActive: true, Urutan: 1}

	// TDD D14: semua admin boleh menambah jabatan (kab/prov butuh jabatan sendiri).
	for _, tc := range []struct {
		name  string
		actor domain.ActorContext
	}{
		{"kabupaten", testutil.KabActor()},
		{"provinsi", testutil.ProvActor()},
		{"nasional", testutil.NasActor()},
		{"super", testutil.SuperActor()},
	} {
		repo := &fakeJabatanRepo{}
		if _, err := kepSvc(nil, repo).CreateJabatan(ctx, in, tc.actor, domain.AuditContext{}); err != nil {
			t.Fatalf("CreateJabatan oleh %s gagal: %v", tc.name, err)
		}
		if !repo.created {
			t.Fatalf("jabatan tidak tersimpan untuk %s", tc.name)
		}
	}

	// USER tidak boleh menambah jabatan.
	if _, err := kepSvc(nil, &fakeJabatanRepo{}).CreateJabatan(ctx, in,
		domain.ActorContext{UserID: "u-user", Name: "User", Role: domain.RoleUser},
		domain.AuditContext{}); err == nil {
		t.Fatal("USER tidak boleh menambah jabatan")
	}
}

func TestListPengurusScopeAndFilter(t *testing.T) {
	ctx := context.Background()
	pgr := &fakePengurusRepo{}
	svc := kepSvcFull(&fakeSKRepo{}, &fakeJabatanRepo{}, pgr)

	// Kabupaten: scope prov+kab, level di-uppercase, masa diteruskan.
	if _, _, err := svc.ListPengurus(ctx, testutil.KabActor(), "kabupaten", "", "AkanBerakhir", "budi", nil, nil, true, 1, 10); err != nil {
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
	if _, err := svc.PengurusStats(ctx, testutil.ProvActor()); err != nil {
		t.Fatalf("PengurusStats gagal: %v", err)
	}
	if pgr.lastFilter.ProvinsiID == nil || *pgr.lastFilter.ProvinsiID != 32 || pgr.lastFilter.KabupatenID != nil {
		t.Fatalf("scope stats provinsi salah: %+v", pgr.lastFilter)
	}
}

// TestAddPengurusEnqueuePengangkatan memastikan A6: pengangkatan pengurus
// menulis email ke outbox (jenis PENGANGKATAN), bukan kirim goroutine langsung.
func TestAddPengurusEnqueuePengangkatan(t *testing.T) {
	ctx := context.Background()
	sk := &domain.SuratKeputusan{
		ID: 1, Level: domain.LevelKabupaten, ProvinsiID: testutil.IntPtr(32), KabupatenID: testutil.IntPtr(3273),
		Status: domain.SKStatusAktif, ApprovalStatus: domain.SKApprovalStatusMenungguProvinsi,
		FileSKKey: "uploads/sk/a.pdf", NomorSK: "001/SK/2026",
	}
	member := &domain.Anggota{
		ID: 9, NIA: "KIPAN-IND-3273-2026-000001", NamaLengkap: "Budi Kader",
		Email: "budi@example.com", Status: domain.AnggotaStatusAktif,
		ProvinsiID: 32, KabupatenID: 3273,
	}
	jab := &domain.Jabatan{ID: 1, Nama: "Ketua", IsActive: true}
	outbox := &testutil.FakeOutboxRepo{}
	svc := NewKepengurusanService(nil, KepengurusanDeps{
		SKRepo: sk2Repo(sk), JabatanRepo: &fakeJabatanRepo{jab: jab},
		PengurusRepo: &fakePengurusRepo{}, AnggotaRepo: &testutil.FakeAnggotaRepo{ByID: map[int]*domain.Anggota{9: member}},
		OutboxRepo: outbox,
	})

	if _, err := svc.AddPengurus(ctx, 1, domain.AddPengurusRequest{AnggotaID: 9, JabatanID: 1, Konfirmasi: true}, testutil.KabActor(), domain.AuditContext{}); err != nil {
		t.Fatalf("AddPengurus gagal: %v", err)
	}
	if len(outbox.Enqueued) != 1 {
		t.Fatalf("harus ada 1 email outbox, dapat %d", len(outbox.Enqueued))
	}
	got := outbox.Enqueued[0]
	if got.Jenis != domain.EmailOutboxPengangkatan {
		t.Fatalf("jenis email salah: %s", got.Jenis)
	}
	if got.ToEmail != member.Email {
		t.Fatalf("penerima salah: %s", got.ToEmail)
	}
	if got.KabupatenID == nil || *got.KabupatenID != 3273 {
		t.Fatalf("cakupan wilayah outbox salah: %+v", got.KabupatenID)
	}
}

// sk2Repo membungkus satu SK untuk fakeSKRepo.
func sk2Repo(sk *domain.SuratKeputusan) *fakeSKRepo { return &fakeSKRepo{sk: sk} }

func TestActorScope(t *testing.T) {
	prov, kab := 32, 3273
	kabCtx := domain.ActorContext{Role: domain.RoleAdminKabupaten, ProvinsiID: &prov, KabupatenID: &kab}
	p, k, err := kabCtx.Scope()
	if err != nil || p == nil || k == nil || *p != prov || *k != kab {
		t.Fatalf("scope kabupaten salah: %v %v %v", p, k, err)
	}
	p, k, err = testutil.ProvActor().Scope()
	if err != nil || p == nil || *p != prov || k != nil {
		t.Fatalf("scope provinsi salah: %v %v %v", p, k, err)
	}
	p, k, err = testutil.NasActor().Scope()
	if err != nil || p != nil || k != nil {
		t.Fatalf("scope nasional harus nil: %v %v %v", p, k, err)
	}
	// Role USER tidak diizinkan mengakses scope wilayah.
	if _, _, err := (domain.ActorContext{Role: domain.RoleUser}).Scope(); err == nil {
		t.Fatal("role USER harus ditolak Scope()")
	}
}

// ---- PAW & Mutasi (A3) ----

func activePengurus() *domain.PengurusDetail {
	return &domain.PengurusDetail{
		ID: 5, SuratKeputusanID: 1, AnggotaID: 9, JabatanID: 1,
		Level: "KABUPATEN", ProvinsiID: testutil.IntPtr(32), KabupatenID: testutil.IntPtr(3273), Status: "Aktif",
	}
}

func activeKabSK() *domain.SuratKeputusan {
	return &domain.SuratKeputusan{
		ID: 1, Level: domain.LevelKabupaten, ProvinsiID: testutil.IntPtr(32), KabupatenID: testutil.IntPtr(3273),
		Status: domain.SKStatusAktif, ApprovalStatus: domain.SKApprovalStatusDisetujui, FileSKKey: "uploads/sk/a.pdf",
	}
}

func TestPawsMeninggalUbahStatus(t *testing.T) {
	ctx := context.Background()
	pgr := &fakePengurusRepo{detail: activePengurus()}
	member := &domain.Anggota{ID: 9, Status: domain.AnggotaStatusAktif}
	svc := NewKepengurusanService(nil, KepengurusanDeps{
		SKRepo: &fakeSKRepo{sk: activeKabSK()}, PengurusRepo: pgr,
		AnggotaRepo: &testutil.FakeAnggotaRepo{ByID: map[int]*domain.Anggota{9: member}},
	})
	if err := svc.Paws(ctx, 5, domain.PawsRequest{Aksi: "MENINGGAL", Keterangan: "Wafat"}, testutil.KabActor(), domain.AuditContext{}); err != nil {
		t.Fatalf("PAW meninggal gagal: %v", err)
	}
	if pgr.lastStatus != domain.PengurusStatusMeninggal {
		t.Fatalf("status pengurus harus Meninggal, dapat %q", pgr.lastStatus)
	}
	if member.Status != domain.AnggotaStatusMeninggal {
		t.Fatalf("status anggota harus MENINGGAL, dapat %q", member.Status)
	}
}

func TestPawsValidasiDanOtorisasi(t *testing.T) {
	ctx := context.Background()
	svcKab := NewKepengurusanService(nil, KepengurusanDeps{
		SKRepo: &fakeSKRepo{sk: activeKabSK()}, PengurusRepo: &fakePengurusRepo{detail: activePengurus()},
	})
	// Aksi tak dikenal.
	if err := svcKab.Paws(ctx, 5, domain.PawsRequest{Aksi: "NGAWUR", Keterangan: "x"}, testutil.KabActor(), domain.AuditContext{}); err == nil {
		t.Fatal("aksi PAW tak dikenal harus ditolak")
	}
	// Keterangan kosong.
	if err := svcKab.Paws(ctx, 5, domain.PawsRequest{Aksi: "DEMISIONER"}, testutil.KabActor(), domain.AuditContext{}); err == nil {
		t.Fatal("keterangan kosong harus ditolak")
	}
	// DIBERHENTIKAN oleh Kabupaten ditolak (TDD: Provinsi/Nasional).
	if err := svcKab.Paws(ctx, 5, domain.PawsRequest{Aksi: "DIBERHENTIKAN", Keterangan: "sanksi"}, testutil.KabActor(), domain.AuditContext{}); err == nil {
		t.Fatal("DIBERHENTIKAN oleh Kabupaten harus ditolak")
	}
	// DIBERHENTIKAN oleh Provinsi seprov: boleh.
	if err := svcKab.Paws(ctx, 5, domain.PawsRequest{Aksi: "DIBERHENTIKAN", Keterangan: "sanksi"}, testutil.ProvActor(), domain.AuditContext{}); err != nil {
		t.Fatalf("DIBERHENTIKAN oleh Provinsi seprov harus boleh: %v", err)
	}
	// Non-aktif → konflik.
	pgrNon := &fakePengurusRepo{detail: &domain.PengurusDetail{ID: 5, SuratKeputusanID: 1, AnggotaID: 9, Status: "Demisioner"}}
	svcNon := NewKepengurusanService(nil, KepengurusanDeps{SKRepo: &fakeSKRepo{sk: activeKabSK()}, PengurusRepo: pgrNon})
	if err := svcNon.Paws(ctx, 5, domain.PawsRequest{Aksi: "DEMISIONER", Keterangan: "x"}, testutil.KabActor(), domain.AuditContext{}); err == nil {
		t.Fatal("pengurus non-aktif harus ditolak")
	}
}

func TestMutasi(t *testing.T) {
	ctx := context.Background()
	src := activePengurus() // SK 1
	target := &domain.SuratKeputusan{
		ID: 2, Level: domain.LevelKabupaten, ProvinsiID: testutil.IntPtr(32), KabupatenID: testutil.IntPtr(3273),
		Status: domain.SKStatusAktif, ApprovalStatus: domain.SKApprovalStatusMenungguProvinsi, FileSKKey: "uploads/sk/b.pdf",
	}
	jab := &domain.Jabatan{ID: 2, Nama: "Sekretaris", IsActive: true}
	deps := KepengurusanDeps{
		SKRepo: &fakeSKRepo{sk: target}, PengurusRepo: &fakePengurusRepo{detail: src}, JabatanRepo: &fakeJabatanRepo{jab: jab},
	}

	// Sumber == tujuan → tolak.
	svc := NewKepengurusanService(nil, deps)
	if _, err := svc.Mutasi(ctx, 5, domain.MutasiRequest{SKID: 1, JabatanID: 2}, testutil.KabActor(), domain.AuditContext{}); err == nil {
		t.Fatal("mutasi ke SK yang sama harus ditolak")
	}
	// Sukses (SK tujuan beda, belum final).
	if out, err := svc.Mutasi(ctx, 5, domain.MutasiRequest{SKID: 2, JabatanID: 2}, testutil.KabActor(), domain.AuditContext{}); err != nil || out == nil {
		t.Fatalf("mutasi sah harus sukses: out=%v err=%v", out, err)
	}

	// SK tujuan final → tolak.
	targetFinal := *target
	targetFinal.ApprovalStatus = domain.SKApprovalStatusDisetujui
	svcFinal := NewKepengurusanService(nil, KepengurusanDeps{
		SKRepo: &fakeSKRepo{sk: &targetFinal}, PengurusRepo: &fakePengurusRepo{detail: src}, JabatanRepo: &fakeJabatanRepo{jab: jab},
	})
	if _, err := svcFinal.Mutasi(ctx, 5, domain.MutasiRequest{SKID: 2, JabatanID: 2}, testutil.KabActor(), domain.AuditContext{}); err == nil {
		t.Fatal("mutasi ke SK final harus ditolak")
	}
}

// TestGetPengurusDetail memverifikasi komposisi detail (pengurus + anggota +
// riwayat) dan otorisasi yurisdiksi (kabupaten/provinsi/nasional).
func TestGetPengurusDetail(t *testing.T) {
	ctx := context.Background()
	prov, kab := 32, 3273
	p := &domain.PengurusDetail{
		ID: 5, AnggotaID: 7, NIA: "KIPAN-IND-3273-2026-000001", NamaLengkap: "Budi",
		Jabatan: "Ketua Umum", Level: "KABUPATEN", Status: "Aktif",
		ProvinsiID: &prov, KabupatenID: &kab,
	}
	member := &domain.Anggota{ID: 7, NIA: p.NIA, NamaLengkap: "Budi", Status: domain.AnggotaStatusAktif, ProvinsiID: prov, KabupatenID: kab}
	riwayat := []domain.PengurusDetail{*p}
	pgr := &fakePengurusRepo{detail: p, byAnggota: riwayat}
	angg := &testutil.FakeAnggotaRepo{ByID: map[int]*domain.Anggota{7: member}}
	wil := &testutil.FakeWilayahRepo{Prov: "JAWA BARAT", Kab: "KOTA BANDUNG"}
	svc := NewKepengurusanService(nil, KepengurusanDeps{
		PengurusRepo: pgr, AnggotaRepo: angg, WilayahRepo: wil,
	})

	// Kabupaten seyurisdiksi → sukses, komponen lengkap.
	out, err := svc.GetPengurusDetail(ctx, 5, testutil.KabActor())
	if err != nil {
		t.Fatalf("detail pengurus seyurisdiksi harus sukses: %v", err)
	}
	if out.Pengurus.ID != 5 || out.Anggota == nil || out.Anggota.NamaLengkap != "Budi" {
		t.Fatalf("komposisi detail tidak lengkap: %+v", out)
	}
	if out.Anggota.ProvinsiNama != "JAWA BARAT" || out.Anggota.KabupatenNama != "KOTA BANDUNG" {
		t.Fatalf("nama wilayah anggota tidak terisi: %q/%q", out.Anggota.ProvinsiNama, out.Anggota.KabupatenNama)
	}
	if len(out.Riwayat) != 1 {
		t.Fatalf("riwayat kepengurusan mau 1 baris, dapat %d", len(out.Riwayat))
	}

	// Provinsi lain → 403.
	otherProv := 33
	stranger := domain.ActorContext{UserID: "u-x", Name: "X", Role: domain.RoleAdminProvinsi, ProvinsiID: &otherProv}
	if _, err := svc.GetPengurusDetail(ctx, 5, stranger); err == nil {
		t.Fatal("pengurus di luar yurisdiksi harus ditolak")
	}

	// Pengurus level NASIONAL (prov/kab nil): Nasional boleh, Kabupaten tolak.
	pNas := &domain.PengurusDetail{ID: 6, AnggotaID: 8, Level: "NASIONAL", Status: "Aktif"}
	svcNas := NewKepengurusanService(nil, KepengurusanDeps{
		PengurusRepo: &fakePengurusRepo{detail: pNas}, AnggotaRepo: angg, WilayahRepo: wil,
	})
	if _, err := svcNas.GetPengurusDetail(ctx, 6, testutil.NasActor()); err != nil {
		t.Fatalf("Nasional harus boleh melihat pengurus Nasional: %v", err)
	}
	if _, err := svcNas.GetPengurusDetail(ctx, 6, testutil.KabActor()); err == nil {
		t.Fatal("Kabupaten tidak boleh melihat pengurus Nasional")
	}
}
