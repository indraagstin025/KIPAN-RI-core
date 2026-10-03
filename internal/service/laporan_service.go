package service

// laporan_service.go — laporan & statistik ter-scope (TDD §6.8): ringkasan,
// demografi, tren, distribusi wilayah, dan anomali NIA (NIK di-dekripsi hanya
// di server untuk dibandingkan dengan kode wilayah domisili).

import (
	"bytes"
	"context"
	"encoding/csv"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/kipan-indonesia/sim-kipan-core/config"
	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/crypto"
)

// LaporanService menghitung laporan ter-scope.
type LaporanService interface {
	Load(ctx context.Context, actor domain.ActorContext) (*domain.LaporanData, error)
	ExportCSV(ctx context.Context, actor domain.ActorContext) ([]byte, error)
}

type laporanService struct {
	cfg  *config.Config
	repo repository.LaporanRepository
}

// NewLaporanService membangun service laporan.
func NewLaporanService(cfg *config.Config, repo repository.LaporanRepository) LaporanService {
	return &laporanService{cfg: cfg, repo: repo}
}

func (s *laporanService) Load(ctx context.Context, actor domain.ActorContext) (*domain.LaporanData, error) {
	if s.repo == nil {
		return nil, unavailable("laporan")
	}
	prov, kab, err := actor.Scope()
	if err != nil {
		return nil, err
	}
	data, err := s.repo.Load(ctx, prov, kab)
	if err != nil {
		return nil, err
	}
	s.detectAnomaliNIA(ctx, prov, kab, data)
	return data, nil
}

// detectAnomaliNIA mendekripsi NIK kandidat (best-effort) lalu menandai yang
// 4 digit awalnya berbeda dari kode kabupaten domisili.
func (s *laporanService) detectAnomaliNIA(ctx context.Context, prov, kab *int, data *domain.LaporanData) {
	rows, err := s.repo.ListAnomaliCandidates(ctx, prov, kab, 3000)
	if err != nil {
		log.Warn().Err(err).Msg("Laporan: gagal memuat kandidat anomali NIA")
		return
	}
	key, err := aesKey(s.cfg)
	if err != nil || len(rows) == 0 {
		return
	}
	for i := range rows {
		nik, err := crypto.DecryptAESGCM(rows[i].NIKEncrypted, key)
		if err != nil || len(nik) < 4 {
			continue
		}
		kodeDom := strings.TrimSpace(rows[i].KodeDomisili)
		if len(kodeDom) < 4 {
			continue
		}
		if nik[:4] != kodeDom[:4] {
			data.AnomaliNIA = append(data.AnomaliNIA, domain.LaporanAnomali{
				NIA:          rows[i].NIA,
				Nama:         rows[i].Nama,
				KodeNIK:      nik[:4],
				KodeDomisili: kodeDom[:4],
			})
		}
	}
	data.AnomaliCount = len(data.AnomaliNIA)
}

// ExportCSV mengekspor ringkasan + demografi + wilayah + tren sebagai CSV.
func (s *laporanService) ExportCSV(ctx context.Context, actor domain.ActorContext) ([]byte, error) {
	data, err := s.Load(ctx, actor)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	write := func(row ...string) { _ = w.Write(row) }

	write("Bagian", "Label", "Jumlah")
	write("Ringkasan", "Total Anggota", strconv.Itoa(data.Summary.TotalAnggota))
	write("Ringkasan", "Anggota Aktif", strconv.Itoa(data.Summary.AnggotaAktif))
	write("Ringkasan", "Menunggu Verifikasi", strconv.Itoa(data.Summary.MenungguVerif))
	write("Ringkasan", "Total Pengurus Aktif", strconv.Itoa(data.Summary.TotalPengurus))
	write("Ringkasan", "Total SK Aktif", strconv.Itoa(data.Summary.TotalSKAktif))
	for _, c := range data.AnggotaByStatus {
		write("Anggota/Status", c.Label, strconv.Itoa(c.Jumlah))
	}
	for _, c := range data.PendaftaranByStatus {
		write("Pendaftaran/Status", c.Label, strconv.Itoa(c.Jumlah))
	}
	for _, c := range data.PengurusByLevel {
		write("Pengurus/Level", c.Label, strconv.Itoa(c.Jumlah))
	}
	for _, c := range data.DemografiUsia {
		write("Demografi/Usia", c.Label, strconv.Itoa(c.Jumlah))
	}
	for _, c := range data.DemografiPendidikan {
		write("Demografi/Pendidikan", c.Label, strconv.Itoa(c.Jumlah))
	}
	for _, c := range data.DemografiPekerjaan {
		write("Demografi/Pekerjaan", c.Label, strconv.Itoa(c.Jumlah))
	}
	for _, c := range data.Wilayah {
		write("Wilayah/"+data.WilayahLabel, c.Label, strconv.Itoa(c.Jumlah))
	}
	for _, t := range data.Tren {
		write("Tren/Bulanan", t.Bulan, strconv.Itoa(t.Jumlah))
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
