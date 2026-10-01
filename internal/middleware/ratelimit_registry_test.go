package middleware

// Regression test keamanan — SEC-AGT-PANIC (Fase 1-2 penyesuaian).
//
// KONTRAK: setiap nama kebijakan yang dipakai wiring rute
// (middleware.RateLimit(rdb, "X")) WAJIB terdaftar pada peta ratePolicies.
//
// Pelanggaran kontrak ini TIDAK tertangkap oleh `go build`/`go vet` karena
// ia adalah lookup map saat runtime. Dampaknya berat: RateLimit() sengaja
// fail-fast (panic) dan dipanggil di main() saat wiring rute — di luar
// middleware recover Fiber — sehingga server mati SEBELUM listen.
//
// STATUS saat ditulis: RED. routes.go memakai "agt_pub" tetapi ratePolicies
// belum mendaftarkannya, sehingga server panic saat start (lihat
// docs/pentest-fase3/LAPORAN_PENTEST_FASE3.md, temuan SEC-AGT-PANIC).
// Test ini akan hijau begitu policy didaftarkan; ia menjadi penjaga regresi
// permanen agar rute baru tidak pernah memakai nama kebijakan tak terdaftar.

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"
)

// rateLimitCallRe mencocokkan pemanggilan wiring RateLimit(<arg>, "nama").
var rateLimitCallRe = regexp.MustCompile(`RateLimit\(\s*[^,]+,\s*"([a-z0-9_]+)"\s*\)`)

func TestRateLimitRegistryCoversRouteUsage(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("tidak dapat menentukan lokasi file test")
	}
	routesPath := filepath.Join(filepath.Dir(thisFile), "..", "handler", "routes.go")

	src, err := os.ReadFile(routesPath)
	if err != nil {
		t.Fatalf("gagal membaca %s: %v", routesPath, err)
	}

	matches := rateLimitCallRe.FindAllStringSubmatch(string(src), -1)
	if len(matches) == 0 {
		t.Fatal("tidak ada pemanggilan RateLimit(...) terdeteksi di routes.go — " +
			"periksa apakah pola wiring berubah")
	}

	for _, m := range matches {
		name := m[1]
		if _, registered := ratePolicies[name]; !registered {
			t.Errorf(
				"kebijakan rate limit %q dipakai di routes.go tetapi TIDAK terdaftar "+
					"di ratePolicies; server akan PANIC saat start (RateLimit fail-fast). "+
					"Daftarkan %q pada map ratePolicies di internal/middleware/ratelimit.go.",
				name, name,
			)
		}
	}
}
