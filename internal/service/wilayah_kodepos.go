package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
)

var posCodePattern = regexp.MustCompile(`^\d{5}$`)

const wilayahPosLimit = 20

type posCacheEntry struct {
	items []domain.WilayahKodepos
	exp   time.Time
}

// cariKodeposItem mencerminkan satu hasil /api/search carikodepos.id.
type cariKodeposItem struct {
	PostalCode string `json:"postalCode"`
	District   struct {
		Name string `json:"name"`
	} `json:"district"`
	City struct {
		Name string `json:"name"`
	} `json:"city"`
	Province struct {
		Name string `json:"name"`
	} `json:"province"`
	Village struct {
		Name string `json:"name"`
	} `json:"village"`
}

type cariKodeposResp struct {
	Success bool              `json:"success"`
	Data    []cariKodeposItem `json:"data"`
}

// normWilayahName menyeragamkan nama untuk pencocokan toleran: huruf kecil,
// tanpa kata administratif (kabupaten/kota/kab./kota adm./administrasi),
// dan tanpa pemisah apa pun (spasi/titik/strip). "Kota Bandung" == "Bandung"
// dan "Cihanjuang Rahayu" == "Cihanjuangrahayu".
func normWilayahName(s string) string {
	t := strings.ToLower(strings.TrimSpace(s))
	repl := []string{"kota administrasi", "", "kota adm.", "", "kabupaten", "", "kota", "", "kab.", "", "administrasi", ""}
	for i := 0; i < len(repl); i += 2 {
		t = strings.ReplaceAll(t, repl[i], repl[i+1])
	}
	var b strings.Builder
	for _, r := range t {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// cocokWilayah true bila a dan b merujuk wilayah yang sama (sama persis
// atau saling mengandung setelah normalisasi). String kosong tak cocok.
func cocokWilayah(a, b string) bool {
	na, nb := normWilayahName(a), normWilayahName(b)
	if na == "" || nb == "" {
		return false
	}
	return na == nb || strings.Contains(na, nb) || strings.Contains(nb, na)
}

func validWilayahBebas(s string) bool {
	t := strings.TrimSpace(s)
	return t != "" && len([]rune(t)) <= 100 && !strings.ContainsAny(t, "<>")
}

func (s *wilayahSvc) ListKodepos(ctx context.Context, desa, kecamatan, kabupaten string) ([]domain.WilayahKodepos, error) {
	d := strings.TrimSpace(desa)
	k := strings.TrimSpace(kecamatan)
	b := strings.TrimSpace(kabupaten)
	for _, v := range []string{d, k, b} {
		if v != "" && !validWilayahBebas(v) {
			return nil, domain.NewValidationError("Nama wilayah tidak valid")
		}
	}
	if d == "" && k == "" {
		return nil, domain.NewValidationError("Desa atau kecamatan wajib diisi untuk mencari kode pos")
	}

	key := strings.ToLower(d + "|" + k + "|" + b)
	s.posMu.Lock()
	if e, ok := s.posCache[key]; ok && time.Now().Before(e.exp) {
		out := append([]domain.WilayahKodepos(nil), e.items...)
		s.posMu.Unlock()
		return out, nil
	}
	s.posMu.Unlock()

	items := s.fetchKodepos(ctx, d, k, b)

	if len(items) > 0 {
		s.posMu.Lock()
		s.posCache[key] = posCacheEntry{items: append([]domain.WilayahKodepos(nil), items...), exp: time.Now().Add(s.cacheTTL)}
		s.posMu.Unlock()
	}
	return items, nil
}

// fetchKodepos menanyakan carikodepos.id lalu memilih kode yang cocok
// (desa → kecamatan → kabupaten). Bila query utama tak menghasilkan yang
// cocok dan varian tanpa-spasi berbeda, varian itu dicoba juga lalu
// digabung (upstream mengindeks sebagian nama tanpa spasi). Fail-open.
func (s *wilayahSvc) fetchKodepos(ctx context.Context, desa, kecamatan, kabupaten string) []domain.WilayahKodepos {
	q := desa
	if q == "" {
		q = kecamatan
	}
	raw := s.queryKodeposRaw(ctx, q)
	if len(filterKodepos(raw, desa, kecamatan, kabupaten)) == 0 {
		if flat := flatQuery(q); flat != q {
			raw = append(raw, s.queryKodeposRaw(ctx, flat)...)
		}
	}
	return filterKodepos(raw, desa, kecamatan, kabupaten)
}

// flatQuery menghapus semua whitespace untuk varian pencarian (upstream
// mengindeks sebagian nama tanpa spasi, cth. "Cihanjuangrahayu").
func flatQuery(q string) string {
	return strings.Join(strings.Fields(q), "")
}

// queryKodeposRaw mengambil + parse hasil search (tanpa filter). Gagal =
// daftar kosong.
func (s *wilayahSvc) queryKodeposRaw(ctx context.Context, q string) []cariKodeposItem {
	endpoint := fmt.Sprintf("%s/search?q=%s&limit=%d", strings.TrimRight(s.posBase, "/"), url.QueryEscape(q), wilayahPosLimit)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		log.Warn().Err(err).Msg("kodepos: gagal menyiapkan permintaan upstream")
		return nil
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		log.Warn().Err(err).Msg("kodepos: upstream tidak terjangkau")
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Warn().Int("status", resp.StatusCode).Msg("kodepos: upstream non-200")
		return nil
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, wilayahMaxBody))
	if err != nil {
		log.Warn().Err(err).Msg("kodepos: gagal membaca respons upstream")
		return nil
	}
	var payload cariKodeposResp
	if err := json.Unmarshal(raw, &payload); err != nil || !payload.Success {
		log.Warn().Err(err).Msg("kodepos: respons upstream tidak valid")
		return nil
	}
	return payload.Data
}

// filterKodepos memilih kode valid + cocok konteks (ketat dulu), dedup,
// potong limit. Tanpa konteks yang cocok → kosong.
func filterKodepos(items []cariKodeposItem, desa, kecamatan, kabupaten string) []domain.WilayahKodepos {
	type cand struct {
		kode     string
		village  string
		district string
		city     string
	}
	seen := make(map[string]bool)
	var cands []cand
	for _, it := range items {
		kode := strings.TrimSpace(it.PostalCode)
		if !posCodePattern.MatchString(kode) || seen[kode] {
			continue
		}
		seen[kode] = true
		cands = append(cands, cand{
			kode:     kode,
			village:  it.Village.Name,
			district: it.District.Name,
			city:     it.City.Name,
		})
	}

	// Filter berlapis (ketat dulu): nama sama di daerah lain tidak ikut.
	// Tanpa konteks yang cocok → kosong (lebih baik daripada salah).
	inContext := func(c cand) bool {
		if kecamatan != "" {
			if cocokWilayah(c.district, kecamatan) {
				return true
			}
			return kabupaten != "" && cocokWilayah(c.city, kabupaten)
		}
		return kabupaten == "" || cocokWilayah(c.city, kabupaten)
	}
	var strict, byKec, byKab []string
	for _, c := range cands {
		if desa != "" && cocokWilayah(c.village, desa) && inContext(c) {
			strict = append(strict, c.kode)
			continue
		}
		if kecamatan != "" && cocokWilayah(c.district, kecamatan) {
			byKec = append(byKec, c.kode)
			continue
		}
		if kabupaten != "" && cocokWilayah(c.city, kabupaten) {
			byKab = append(byKab, c.kode)
		}
	}
	picked := strict
	if len(picked) == 0 {
		picked = byKec
	}
	if len(picked) == 0 {
		picked = byKab
	}
	out := make([]domain.WilayahKodepos, 0, len(picked))
	for i, kode := range picked {
		if i >= wilayahPosLimit {
			break
		}
		out = append(out, domain.WilayahKodepos{KodePos: kode})
	}
	return out
}
