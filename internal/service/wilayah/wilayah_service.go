package wilayah

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
	"github.com/kipan-indonesia/sim-kipan-core/internal/service/svcutil"
)

// WilayahService melayani daftar master wilayah untuk dropdown publik
// (form pendaftaran frontend) dan test lintas-wilayah. Read-only.
type WilayahService interface {
	ListProvinsi(ctx context.Context) ([]domain.WilayahProvinsi, error)
	ListKabupaten(ctx context.Context, provinsiID int) ([]domain.WilayahKabupaten, error)
	// ListKecamatan mengembalikan saran kecamatan dari upstream wilayah.id.
	// Fail-open: gangguan upstream menghasilkan daftar kosong (bukan error)
	// agar form tetap bisa diisi manual.
	ListKecamatan(ctx context.Context, kabupatenKode string) ([]domain.WilayahKecamatan, error)
	// ListDesa mengembalikan saran desa/kelurahan dari upstream wilayah.id.
	// Sama seperti kecamatan: fail-open, tanpa tabel baru.
	ListDesa(ctx context.Context, kecamatanKode string) ([]domain.WilayahDesa, error)
	// ListKodepos mengembalikan saran kode pos (kode saja) dari upstream
	// carikodepos.id, dicocokkan desa → kecamatan → kabupaten. Fail-open.
	ListKodepos(ctx context.Context, desa, kecamatan, kabupaten string) ([]domain.WilayahKodepos, error)
}

// Batasan upstream + cache proxy kecamatan (selaras perilaku lama: cache
// 24 jam, fail-open). Konstanta diekspor agar test bisa mempersingkat TTL.
const (
	wilayahUpstreamBase    = "https://wilayah.id/api"
	wilayahUpstreamTimeout = 8 * time.Second
	wilayahCacheTTL        = 24 * time.Hour
	wilayahMaxBody         = 1 << 20 // 1 MB
)

var (
	kabKodePattern  = regexp.MustCompile(`^\d{4}$`)
	kecKodePattern  = regexp.MustCompile(`^\d{2}\.\d{2}\.\d{2}$`)
	kecKode6        = regexp.MustCompile(`^\d{6}$`)
	desaKodePattern = regexp.MustCompile(`^\d{2}\.\d{2}\.\d{2}\.\d{4}$`)
)

type kecCacheEntry struct {
	items []domain.WilayahKecamatan
	exp   time.Time
}

type desaCacheEntry struct {
	items []domain.WilayahDesa
	exp   time.Time
}

type wilayahSvc struct {
	repo         repository.WilayahRepository
	httpClient   *http.Client
	upstreamBase string
	posBase      string
	cacheTTL     time.Duration

	kecMu    sync.Mutex
	kecCache map[string]kecCacheEntry

	desaMu    sync.Mutex
	desaCache map[string]desaCacheEntry

	posMu    sync.Mutex
	posCache map[string]posCacheEntry
}

func NewWilayahService(repo repository.WilayahRepository) WilayahService {
	return &wilayahSvc{
		repo: repo,
		httpClient: &http.Client{
			Timeout: wilayahUpstreamTimeout,
			// Tolak redirect: upstream adalah host tetap; 3xx diperlakukan
			// sebagai gagal (fail-open) alih-alih mengikuti ke host asing.
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		upstreamBase: wilayahUpstreamBase,
		posBase:      "https://carikodepos.id/api",
		cacheTTL:     wilayahCacheTTL,
		kecCache:     make(map[string]kecCacheEntry),
		desaCache:    make(map[string]desaCacheEntry),
		posCache:     make(map[string]posCacheEntry),
	}
}

func (s *wilayahSvc) ListProvinsi(ctx context.Context) ([]domain.WilayahProvinsi, error) {
	if s.repo == nil {
		return nil, svcutil.Unavailable("wilayah")
	}
	return s.repo.ListProvinsi(ctx)
}

func (s *wilayahSvc) ListKabupaten(ctx context.Context, provinsiID int) ([]domain.WilayahKabupaten, error) {
	if provinsiID <= 0 {
		return nil, domain.NewValidationError("ID provinsi tidak valid")
	}
	if s.repo == nil {
		return nil, svcutil.Unavailable("wilayah")
	}
	ok, err := s.repo.ExistsProvinsi(ctx, provinsiID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain.NewNotFoundError("Provinsi")
	}
	return s.repo.ListKabupaten(ctx, provinsiID)
}

// wilayahIDList mencerminkan envelope upstream wilayah.id:
// {"data":[{"code":"32.73.01","name":"Sukasari"}],...}.
type wilayahIDList struct {
	Data []struct {
		Code string `json:"code"`
		Name string `json:"name"`
	} `json:"data"`
}

func (s *wilayahSvc) ListKecamatan(ctx context.Context, kabupatenKode string) ([]domain.WilayahKecamatan, error) {
	kode := strings.TrimSpace(kabupatenKode)
	if !kabKodePattern.MatchString(kode) {
		return nil, domain.NewValidationError("Kode kabupaten tidak valid (4 digit angka)")
	}

	// Cache dulu (hanya entri sukses yang disimpan).
	s.kecMu.Lock()
	if e, ok := s.kecCache[kode]; ok && time.Now().Before(e.exp) {
		out := append([]domain.WilayahKecamatan(nil), e.items...)
		s.kecMu.Unlock()
		return out, nil
	}
	s.kecMu.Unlock()

	items := s.fetchKecamatan(ctx, kode)

	// Hanya keberhasilan non-kosong yang di-cache: membatasi memori pada
	// wilayah nyata (≤514) dan mencegah hasil fail-open menempel 24 jam.
	if len(items) > 0 {
		s.kecMu.Lock()
		s.kecCache[kode] = kecCacheEntry{items: append([]domain.WilayahKecamatan(nil), items...), exp: time.Now().Add(s.cacheTTL)}
		s.kecMu.Unlock()
	}
	return items, nil
}

// getUpstreamList mengambil envelope wilayah.id untuk path relatif tertentu.
// Mengembalikan ok=false untuk SEMUA kegagalan (jaringan, timeout, non-200,
// JSON rusak) agar pemanggil bisa fail-open. Tidak pernah log isi body.
func (s *wilayahSvc) getUpstreamList(ctx context.Context, relPath, label, kode string) (wilayahIDList, bool) {
	var empty wilayahIDList
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(s.upstreamBase, "/")+relPath, nil)
	if err != nil {
		log.Warn().Err(err).Str("kode", kode).Msg(label + ": gagal menyiapkan permintaan upstream")
		return empty, false
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		log.Warn().Err(err).Str("kode", kode).Msg(label + ": upstream tidak terjangkau")
		return empty, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Warn().Int("status", resp.StatusCode).Str("kode", kode).Msg(label + ": upstream non-200")
		return empty, false
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, wilayahMaxBody))
	if err != nil {
		log.Warn().Err(err).Str("kode", kode).Msg(label + ": gagal membaca respons upstream")
		return empty, false
	}
	var payload wilayahIDList
	if err := json.Unmarshal(raw, &payload); err != nil {
		log.Warn().Err(err).Str("kode", kode).Msg(label + ": respons upstream tidak valid")
		return empty, false
	}
	return payload, true
}

// validNamaUpstream menolak string kosong/terlalu panjang/angle-bracket
// agar konten upstream tak tepercaya tidak menjadi stored-XSS.
func validNamaUpstream(name string) bool {
	if name == "" || len([]rune(name)) > 100 || strings.ContainsAny(name, "<>") {
		return false
	}
	return true
}

// koreksiNamaUpstream memetakan kode upstream wilayah.id yang ejaannya
// diketahui salah ke ejaan benar. Diterapkan sebelum validasi agar cache,
// dropdown, dan submit menerima nama yang benar.
// Cara menambah entri: "kode.bertitik": "Ejaan Benar" + unit test +
// laporkan ke upstream (wilayah.id/cahyadsn).
var koreksiNamaUpstream = map[string]string{
	"32.17.02.2003": "Cihanjuang Rahayu", // upstream: "Cihanjuangrahayu"
}

// terapkanKoreksi mengembalikan nama terkoreksi bila kode terdaftar.
func terapkanKoreksi(codeUpstream, name string) string {
	if fix, ok := koreksiNamaUpstream[codeUpstream]; ok {
		return fix
	}
	return name
}

// fetchKecamatan mengambil + menormalisasi daftar kecamatan. TIDAK PERNAH
// mengembalikan error ke pemanggil publik (fail-open): setiap kegagalan
// menjadi daftar kosong agar form tidak terblokir.
func (s *wilayahSvc) fetchKecamatan(ctx context.Context, kode string) []domain.WilayahKecamatan {
	empty := []domain.WilayahKecamatan{}
	dotted := kode[:2] + "." + kode[2:]
	payload, ok := s.getUpstreamList(ctx, fmt.Sprintf("/districts/%s.json", dotted), "kecamatan", kode)
	if !ok {
		return empty
	}

	out := make([]domain.WilayahKecamatan, 0, len(payload.Data))
	for _, d := range payload.Data {
		codeUp := strings.TrimSpace(d.Code)
		name := terapkanKoreksi(codeUp, strings.TrimSpace(d.Name))
		if !kecKodePattern.MatchString(codeUp) {
			continue
		}
		if !validNamaUpstream(name) {
			continue
		}
		out = append(out, domain.WilayahKecamatan{
			Kode: strings.ReplaceAll(codeUp, ".", ""),
			Nama: name,
		})
	}
	return out
}

func (s *wilayahSvc) ListDesa(ctx context.Context, kecamatanKode string) ([]domain.WilayahDesa, error) {
	kode := strings.TrimSpace(kecamatanKode)
	if !kecKode6.MatchString(kode) {
		return nil, domain.NewValidationError("Kode kecamatan tidak valid (6 digit angka)")
	}

	s.desaMu.Lock()
	if e, ok := s.desaCache[kode]; ok && time.Now().Before(e.exp) {
		out := append([]domain.WilayahDesa(nil), e.items...)
		s.desaMu.Unlock()
		return out, nil
	}
	s.desaMu.Unlock()

	items := s.fetchDesa(ctx, kode)

	if len(items) > 0 {
		s.desaMu.Lock()
		s.desaCache[kode] = desaCacheEntry{items: append([]domain.WilayahDesa(nil), items...), exp: time.Now().Add(s.cacheTTL)}
		s.desaMu.Unlock()
	}
	return items, nil
}

// fetchDesa mengambil + menormalisasi daftar desa/kelurahan (fail-open,
// pola sama seperti kecamatan).
func (s *wilayahSvc) fetchDesa(ctx context.Context, kode string) []domain.WilayahDesa {
	empty := []domain.WilayahDesa{}
	dotted := kode[:2] + "." + kode[2:4] + "." + kode[4:]
	payload, ok := s.getUpstreamList(ctx, fmt.Sprintf("/villages/%s.json", dotted), "desa", kode)
	if !ok {
		return empty
	}

	out := make([]domain.WilayahDesa, 0, len(payload.Data))
	for _, d := range payload.Data {
		codeUp := strings.TrimSpace(d.Code)
		name := terapkanKoreksi(codeUp, strings.TrimSpace(d.Name))
		if !desaKodePattern.MatchString(codeUp) {
			continue
		}
		if !validNamaUpstream(name) {
			continue
		}
		out = append(out, domain.WilayahDesa{
			Kode: strings.ReplaceAll(codeUp, ".", ""),
			Nama: name,
		})
	}
	return out
}
