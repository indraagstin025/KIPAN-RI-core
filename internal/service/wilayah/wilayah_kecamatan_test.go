package wilayah

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newKecamatanSvc(t *testing.T, handler http.HandlerFunc) (*wilayahSvc, *int) {
	t.Helper()
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		handler(w, r)
	}))
	t.Cleanup(srv.Close)
	svc := NewWilayahService(nil).(*wilayahSvc)
	svc.upstreamBase = srv.URL
	svc.httpClient = srv.Client()
	return svc, &hits
}

func TestListKecamatanTolakKodeInvalid(t *testing.T) {
	svc := NewWilayahService(nil)
	for _, kode := range []string{"", "32", "327", "32733", "32.73", "abcd", "../../etc", "https://x/"} {
		if _, err := svc.ListKecamatan(context.Background(), kode); err == nil {
			t.Fatalf("kode %q harus ditolak", kode)
		}
	}
}

func TestListKecamatanSuksesDanNormalisasi(t *testing.T) {
	svc, _ := newKecamatanSvc(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/districts/32.73.json" {
			t.Errorf("path upstream salah: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[{"code":"32.73.01","name":"Sukasari"},{"code":"32.73.08","name":"Cidadap"}],"meta":{}}`))
	})
	items, err := svc.ListKecamatan(context.Background(), "3273")
	if err != nil {
		t.Fatalf("tidak boleh error: %v", err)
	}
	if len(items) != 2 || items[0].Kode != "327301" || items[0].Nama != "Sukasari" || items[1].Kode != "327308" {
		t.Fatalf("normalisasi salah: %+v", items)
	}
}

func TestListKecamatanCache(t *testing.T) {
	svc, hits := newKecamatanSvc(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":[{"code":"32.73.01","name":"Sukasari"}]}`))
	})
	if _, err := svc.ListKecamatan(context.Background(), "3273"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ListKecamatan(context.Background(), "3273"); err != nil {
		t.Fatal(err)
	}
	if *hits != 1 {
		t.Fatalf("harap 1 hit upstream (cache), dapat %d", *hits)
	}
}

func TestListKecamatanFailOpen(t *testing.T) {
	cases := map[string]http.HandlerFunc{
		"status-500": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		},
		"json-rusak": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{bukan json`))
		},
		"bukan-array": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"data":"zzz"}`))
		},
		"redirect": func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "http://example.com/x.json", http.StatusFound)
		},
	}
	for name, h := range cases {
		svc, _ := newKecamatanSvc(t, h)
		items, err := svc.ListKecamatan(context.Background(), "3273")
		if err != nil {
			t.Fatalf("%s: harus fail-open tanpa error: %v", name, err)
		}
		if len(items) != 0 {
			t.Fatalf("%s: harap daftar kosong, dapat %+v", name, items)
		}
	}
}

func TestListKecamatanItemTakValidDilewati(t *testing.T) {
	svc, _ := newKecamatanSvc(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":[
			{"code":"32.73.01","name":"Sukasari"},
			{"code":"salah","name":"X"},
			{"code":"32.73.02","name":""},
			{"code":"32.73.03","name":"A<b"},
			{"code":"32.73.04","name":"` + strings.Repeat("z", 101) + `"}
		]}`))
	})
	items, err := svc.ListKecamatan(context.Background(), "3273")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Kode != "327301" {
		t.Fatalf("hanya item valid yang dipakai: %+v", items)
	}
}

func TestListKecamatanTimeoutFailOpen(t *testing.T) {
	svc, _ := newKecamatanSvc(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	})
	svc.httpClient.Timeout = 200 * time.Millisecond
	items, err := svc.ListKecamatan(context.Background(), "3273")
	if err != nil {
		t.Fatalf("timeout harus fail-open: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("harap kosong saat timeout: %+v", items)
	}
}

func TestListKecamatanKosongTidakDiCache(t *testing.T) {
	svc, hits := newKecamatanSvc(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":[]}`))
	})
	for i := 0; i < 2; i++ {
		items, err := svc.ListKecamatan(context.Background(), "3273")
		if err != nil || len(items) != 0 {
			t.Fatalf("harap kosong tanpa error: %+v, %v", items, err)
		}
	}
	if *hits != 2 {
		t.Fatalf("hasil kosong jangan di-cache (harap 2 hit, dapat %d)", *hits)
	}
}
