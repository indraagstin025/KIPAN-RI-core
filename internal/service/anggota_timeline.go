package service

// anggota_timeline.go — timeline riwayat & jejak audit per anggota (detail
// dialog bergaya tab): gabungan perjalanan pendaftaran + kepengurusan.

import (
	"context"
	"sort"
	"strconv"
	"strings"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
)

// labelPendaftaranAksi memetakan aksi pendaftaran_riwayat ke label manusiawi.
func labelPendaftaranAksi(aksi string) string {
	switch strings.ToUpper(strings.TrimSpace(aksi)) {
	case "SUBMIT":
		return "Pendaftaran dikirim"
	case "VERIFIKASI":
		return "Verifikasi berkas"
	case "SETUJUI":
		return "Disetujui — diangkat sebagai anggota"
	case "PERBAIKAN":
		return "Diminta perbaikan berkas"
	case "TOLAK":
		return "Ditolak"
	default:
		return aksi
	}
}

// pengurusWilayahLabel mengembalikan label wilayah dari baris pengurus.
func pengurusWilayahLabel(p domain.PengurusDetail) string {
	switch domain.TingkatWilayah(p.Level) {
	case domain.LevelNasional:
		return "Nasional"
	case domain.LevelProvinsi:
		if p.ProvinsiNama != nil && *p.ProvinsiNama != "" {
			return *p.ProvinsiNama
		}
		return "Provinsi"
	default:
		if p.KabupatenNama != nil && *p.KabupatenNama != "" {
			return *p.KabupatenNama
		}
		return "Kabupaten/Kota"
	}
}

// AnggotaRiwayat menggabungkan riwayat pendaftaran + kepengurusan menjadi satu
// timeline (terbaru di atas), ter-scope wilayah aktor.
func (s *anggotaService) AnggotaRiwayat(ctx context.Context, id int, actor domain.ActorContext) ([]domain.AnggotaRiwayatItem, error) {
	if id <= 0 {
		return nil, domain.NewValidationError("ID anggota tidak valid")
	}
	if s.anggotaRepo == nil {
		return nil, unavailable("anggota")
	}
	member, err := s.anggotaRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !actor.CanAccessWilayah(member.ProvinsiID, member.KabupatenID) {
		return nil, domain.NewForbiddenError("Data anggota di luar wilayah kerja Anda")
	}

	items := make([]domain.AnggotaRiwayatItem, 0)

	// (1) Riwayat pendaftaran (bila anggota berasal dari pendaftaran).
	if member.PendaftaranID != nil && s.pendaftaranRepo != nil {
		if hist, err := s.pendaftaranRepo.ListHistory(ctx, *member.PendaftaranID); err == nil {
			for _, h := range hist {
				oleh := "Pendaftar"
				if h.ActorName != nil && strings.TrimSpace(*h.ActorName) != "" {
					oleh = *h.ActorName
				}
				ket := ""
				if h.Catatan != nil {
					ket = strings.TrimSpace(*h.Catatan)
				}
				items = append(items, domain.AnggotaRiwayatItem{
					Waktu: h.CreatedAt, Sumber: "PENDAFTARAN", Aksi: h.Aksi,
					Label: labelPendaftaranAksi(h.Aksi), Oleh: oleh, Keterangan: ket,
				})
			}
		}
	}

	// (2) Riwayat kepengurusan (semua status).
	if s.pengurusRepo != nil {
		if list, err := s.pengurusRepo.ListByAnggota(ctx, id); err == nil {
			for _, p := range list {
				items = append(items, domain.AnggotaRiwayatItem{
					Waktu: p.TanggalMulai, Sumber: "KEPENGURUSAN", Aksi: "DIANGKAT",
					Label: "Diangkat sebagai " + p.Jabatan, Oleh: "Sistem",
					Keterangan: strings.TrimSpace("SK " + p.NomorSK + " · " + pengurusWilayahLabel(p)),
				})
				if p.TanggalSelesai != nil || p.Status != string(domain.PengurusStatusAktif) {
					waktu := p.TanggalMulai
					if p.TanggalSelesai != nil {
						waktu = *p.TanggalSelesai
					}
					ket := p.Status
					if p.KeteranganStatus != nil && strings.TrimSpace(*p.KeteranganStatus) != "" {
						ket = *p.KeteranganStatus
					}
					items = append(items, domain.AnggotaRiwayatItem{
						Waktu: waktu, Sumber: "KEPENGURUSAN", Aksi: "BERAKHIR",
						Label: "Berakhir: " + p.Status, Oleh: "Sistem", Keterangan: ket,
					})
				}
			}
		}
	}

	sort.Slice(items, func(i, j int) bool { return items[i].Waktu.After(items[j].Waktu) })
	return items, nil
}

// AnggotaActivity mengembalikan jejak audit (activity_logs) entitas anggota,
// ter-scope wilayah aktor.
func (s *anggotaService) AnggotaActivity(ctx context.Context, id int, actor domain.ActorContext) ([]domain.ActivityLog, error) {
	if id <= 0 {
		return nil, domain.NewValidationError("ID anggota tidak valid")
	}
	if s.anggotaRepo == nil {
		return nil, unavailable("anggota")
	}
	member, err := s.anggotaRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !actor.CanAccessWilayah(member.ProvinsiID, member.KabupatenID) {
		return nil, domain.NewForbiddenError("Data anggota di luar wilayah kerja Anda")
	}
	if s.auditRepo == nil {
		return []domain.ActivityLog{}, nil
	}
	items, _, err := s.auditRepo.List(ctx, domain.AuditFilter{
		Entity: "anggota", EntityID: strconv.Itoa(id), Limit: 100, WithTotal: false,
	})
	if err != nil {
		return nil, err
	}
	return items, nil
}
