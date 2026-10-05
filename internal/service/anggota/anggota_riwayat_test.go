package anggota

import (
	"context"
	"testing"
	"time"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
	"github.com/kipan-indonesia/sim-kipan-core/internal/service/testutil"
)

// fakeRiwayatPengurusRepo mengembalikan riwayat kepengurusan canned.
type fakeRiwayatPengurusRepo struct {
	repository.PengurusRepository
	rows []domain.PengurusDetail
}

func (f *fakeRiwayatPengurusRepo) ListByAnggota(context.Context, int) ([]domain.PengurusDetail, error) {
	return f.rows, nil
}

// TestAnggotaRiwayatDiangkatPakaiWaktuPengangkatan mengunci: item DIANGKAT
// memakai CreatedAt (saat pengangkatan dilakukan), bukan TanggalMulai (awal
// masa bakti). Kasus nyata: tanggal_mulai 2026-10-04 (tengah malam) dengan
// pengangkatan 11:49 — item harus berwaktu 11:49 agar tidak tenggelam di
// bawah event pendaftaran 11:25-11:29.
func TestAnggotaRiwayatDiangkatPakaiWaktuPengangkatan(t *testing.T) {
	mulai := time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)
	diangkat := time.Date(2026, 10, 4, 11, 49, 11, 0, time.UTC)
	kab := "Kabupaten Bandung Barat"
	member := &domain.Anggota{ID: 74, ProvinsiID: 32, KabupatenID: 3273}
	svc := NewAnggotaService(nil, AnggotaDeps{
		AnggotaRepo: &testutil.FakeAnggotaRepo{ByID: map[int]*domain.Anggota{74: member}},
		PengurusRepo: &fakeRiwayatPengurusRepo{rows: []domain.PengurusDetail{{
			ID: 63, AnggotaID: 74, Jabatan: "Ketua", NomorSK: "17182",
			Level: "KABUPATEN", KabupatenNama: &kab, Status: "Aktif",
			TanggalMulai: mulai, CreatedAt: diangkat,
		}}},
	})

	items, err := svc.AnggotaRiwayat(context.Background(), 74, testutil.SuperActor())
	if err != nil {
		t.Fatalf("AnggotaRiwayat gagal: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("harap 1 item DIANGKAT, dapat %d", len(items))
	}
	got := items[0]
	if got.Aksi != "DIANGKAT" {
		t.Fatalf("aksi = %q, harap DIANGKAT", got.Aksi)
	}
	if !got.Waktu.Equal(diangkat) {
		t.Fatalf("waktu = %v, harap waktu pengangkatan %v", got.Waktu, diangkat)
	}
}

// stubRiwayatFallback meniru repo dengan riwayat kepengurusan "-" dan aksi
// pendaftaran terakhir terkontrol.
type stubRiwayatFallback struct {
	*testutil.FakeAnggotaRepo
	latest map[int]string
}

func (s *stubRiwayatFallback) LatestPendaftaranAksiByAnggotaIDs(context.Context, []int) (map[int]string, error) {
	return s.latest, nil
}

// TestAttachRiwayatFallbackPendaftaranTerakhir mengunci: anggota tanpa
// riwayat kepengurusan ("-") mendapat label event pendaftaran terakhir;
// yang sudah punya ringkasan kepengurusan tidak ditimpa.
func TestAttachRiwayatFallbackPendaftaranTerakhir(t *testing.T) {
	newSvc := func(latest map[int]string) AnggotaService {
		return NewAnggotaService(nil, AnggotaDeps{
			AnggotaRepo: &stubRiwayatFallback{
				FakeAnggotaRepo: &testutil.FakeAnggotaRepo{},
				latest:          latest,
			},
		})
	}

	t.Run("fallback mengisi", func(t *testing.T) {
		svc := newSvc(map[int]string{7: "SETUJUI"})
		items := []domain.AnggotaListItem{{ID: 7}}
		svc.(*anggotaService).attachRiwayat(context.Background(), items)
		if items[0].Riwayat != "Disetujui — diangkat sebagai anggota" {
			t.Fatalf("riwayat = %q", items[0].Riwayat)
		}
	})

	t.Run("tanpa jejak apa pun tetap strip", func(t *testing.T) {
		svc := newSvc(map[int]string{})
		items := []domain.AnggotaListItem{{ID: 7}}
		svc.(*anggotaService).attachRiwayat(context.Background(), items)
		if items[0].Riwayat != "-" {
			t.Fatalf("riwayat = %q, harap -", items[0].Riwayat)
		}
	})
}
