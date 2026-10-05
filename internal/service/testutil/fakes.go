// Package testutil menampung fake repository & actor bersama untuk uji
// service. Fake yang dipakai lintas-domain tinggal di sini agar tiap
// subpackage service hasil split tetap bisa menggunakannya; fake yang
// spesifik satu domain tetap di package-nya masing-masing.
package testutil

import (
	"context"
	"errors"
	"fmt"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
)

// ErrFakeUnimpl adalah error baku fake yang metodenya tidak diimplementasikan.
var ErrFakeUnimpl = errors.New("fake: tidak diimplementasikan")

// FakeAnggotaRepo adalah anggotaRepo in-memory untuk uji.
type FakeAnggotaRepo struct {
	ByNIA     map[string]*domain.Anggota
	ByID      map[int]*domain.Anggota
	Links     map[int]string
	NikExists bool
}

func (f *FakeAnggotaRepo) ExistsByNikHash(_ context.Context, _ string) (bool, error) {
	return f.NikExists, nil
}

func (f *FakeAnggotaRepo) GetByNIA(_ context.Context, nia string) (*domain.Anggota, error) {
	if a, ok := f.ByNIA[nia]; ok {
		return a, nil
	}
	return nil, domain.ErrNotFound
}

func (f *FakeAnggotaRepo) GetByID(_ context.Context, id int) (*domain.Anggota, error) {
	if a, ok := f.ByID[id]; ok && a != nil {
		return a, nil
	}
	return nil, domain.ErrNotFound
}

func (f *FakeAnggotaRepo) SetKTAPDFKey(_ context.Context, _ int, _ string) error {
	return nil
}

func (f *FakeAnggotaRepo) SetStatus(_ context.Context, id int, status domain.AnggotaStatus) error {
	if a, ok := f.ByID[id]; ok && a != nil {
		a.Status = status
	}
	return nil
}

func (f *FakeAnggotaRepo) RiwayatByAnggotaIDs(_ context.Context, ids []int) (map[int]string, error) {
	out := make(map[int]string, len(ids))
	for _, id := range ids {
		out[id] = "-"
	}
	return out, nil
}

func (f *FakeAnggotaRepo) AllocateNIA(_ context.Context, _, _, year int) (string, error) {
	return fmt.Sprintf("KIPAN-IND-9999-%d-000001", year), nil
}

func (f *FakeAnggotaRepo) Create(_ context.Context, a *domain.Anggota) (*domain.Anggota, error) {
	if a.ID == 0 {
		a.ID = 100
	}
	return a, nil
}

func (f *FakeAnggotaRepo) Update(_ context.Context, _ *domain.Anggota) error {
	return nil
}

func (f *FakeAnggotaRepo) SetUserID(_ context.Context, id int, userID string) error {
	if f.Links == nil {
		f.Links = map[int]string{}
	}
	f.Links[id] = userID
	if a, ok := f.ByID[id]; ok && a != nil {
		a.UserID = &userID
	}
	return nil
}

func (f *FakeAnggotaRepo) GetByUserID(_ context.Context, userID string) (*domain.Anggota, error) {
	for _, a := range f.ByNIA {
		if a != nil && a.UserID != nil && *a.UserID == userID {
			return a, nil
		}
	}
	for id, uid := range f.Links {
		if uid == userID {
			if a, ok := f.ByID[id]; ok {
				return a, nil
			}
		}
	}
	return nil, domain.ErrNotFound
}

func (f *FakeAnggotaRepo) ListAnggota(_ context.Context, _, _ *int, _, _ string, _, _ int) ([]domain.AnggotaListItem, error) {
	return []domain.AnggotaListItem{}, nil
}

func (f *FakeAnggotaRepo) CountAnggota(_ context.Context, _, _ *int, _, _ string) (int, error) {
	return 0, nil
}

var _ repository.AnggotaRepository = (*FakeAnggotaRepo)(nil)

// FakeWilayahRepo adalah wilayahRepo in-memory untuk uji.
type FakeWilayahRepo struct {
	Prov, Kab string
	Err       error
}

func (f *FakeWilayahRepo) ExistsProvinsi(context.Context, int) (bool, error) {
	return true, nil
}

func (f *FakeWilayahRepo) KabupatenInProvinsi(context.Context, int, int) (bool, error) {
	return true, nil
}

func (f *FakeWilayahRepo) ListProvinsi(context.Context) ([]domain.WilayahProvinsi, error) {
	return nil, ErrFakeUnimpl
}

func (f *FakeWilayahRepo) ListKabupaten(context.Context, int) ([]domain.WilayahKabupaten, error) {
	return nil, ErrFakeUnimpl
}

func (f *FakeWilayahRepo) GetNames(context.Context, int, int) (string, string, error) {
	return f.Prov, f.Kab, f.Err
}

var _ repository.WilayahRepository = (*FakeWilayahRepo)(nil)

// FakeMemberUserRepo menyimpan user in-memory keyed by email.
type FakeMemberUserRepo struct {
	repository.UserRepository
	ByEmail  map[string]*domain.User
	ByID     map[string]*domain.User
	Created  []*domain.User
	Revoked  []string
	LastHash string
}

func (f *FakeMemberUserRepo) GetByID(_ context.Context, id string) (*domain.User, error) {
	if u, ok := f.ByID[id]; ok && u != nil {
		return u, nil
	}
	return nil, domain.ErrUserNotFound
}

func (f *FakeMemberUserRepo) UpdatePassword(_ context.Context, id string, hash string) error {
	f.LastHash = hash
	if u, ok := f.ByID[id]; ok && u != nil {
		u.PasswordHash = hash
	}
	return nil
}

func (f *FakeMemberUserRepo) RevokeAllUserTokens(_ context.Context, id string) error {
	f.Revoked = append(f.Revoked, id)
	return nil
}

func (f *FakeMemberUserRepo) GetByEmail(_ context.Context, email string) (*domain.User, error) {
	for addr, u := range f.ByEmail {
		if addr == email {
			return u, nil
		}
	}
	return nil, domain.ErrUserNotFound
}

func (f *FakeMemberUserRepo) Create(_ context.Context, u *domain.User) error {
	if f.ByEmail == nil {
		f.ByEmail = map[string]*domain.User{}
	}
	if _, ok := f.ByEmail[u.Email]; ok {
		return domain.NewConflictError("email sudah ada")
	}
	f.ByEmail[u.Email] = u
	f.Created = append(f.Created, u)
	return nil
}

// FakeOutboxRepo merekam email yang di-enqueue (outbox).
type FakeOutboxRepo struct {
	repository.EmailOutboxRepository
	Enqueued []*domain.EmailOutbox
}

func (f *FakeOutboxRepo) Enqueue(_ context.Context, it *domain.EmailOutbox) error {
	f.Enqueued = append(f.Enqueued, it)
	return nil
}

// RevealTestKey adalah kunci AES/BlindIndex fixture untuk uji enkripsi NIK.
const RevealTestKey = "aa00112233445566778899aabbccddeeffaa00112233445566778899aabbccdd"

// IntPtr membuat pointer int inline untuk fixture test.
func IntPtr(i int) *int { return &i }

// KabActor adalah aktor Admin Kabupaten fixture (prov 32, kab 3273).
func KabActor() domain.ActorContext {
	prov, kab := 32, 3273
	return domain.ActorContext{UserID: "u-kab", Name: "Kab", Role: domain.RoleAdminKabupaten, ProvinsiID: &prov, KabupatenID: &kab}
}

// ProvActor adalah aktor Admin Provinsi fixture (prov 32).
func ProvActor() domain.ActorContext {
	prov := 32
	return domain.ActorContext{UserID: "u-prov", Name: "Prov", Role: domain.RoleAdminProvinsi, ProvinsiID: &prov}
}

// NasActor adalah aktor Admin Nasional fixture.
func NasActor() domain.ActorContext {
	return domain.ActorContext{UserID: "u-nas", Name: "Nas", Role: domain.RoleAdminNasional}
}

// SuperActor adalah aktor Super Admin fixture.
func SuperActor() domain.ActorContext {
	prov, kab := 32, 3273
	return domain.ActorContext{UserID: "admin-1", Name: "Admin", Role: domain.RoleSuperAdmin, ProvinsiID: &prov, KabupatenID: &kab}
}
