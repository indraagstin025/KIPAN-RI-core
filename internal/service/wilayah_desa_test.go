package service

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestListDesaTolakKodeInvalid(t *testing.T) {
	svc := NewWilayahService(nil)
	for _, kode := range []string{"", "3273", "32730811", "32.73.08", "abcdef", "../../etc", "https://x/"} {
		if _, err := svc.ListDesa(context.Background(), kode); err == nil {
			t.Fatalf("kode %q harus ditolak", kode)
		}
	}
}

func TestListDesaSuksesDanNormalisasi(t *testing.T) {
	svc, _ := newKecamatanSvc(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/villages/32.73.08.json" {
			t.Errorf("path upstream salah: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[{"code":"32.73.08.1001","name":"Hegarmanah"},{"code":"32.73.08.1002","name":"Ciumbuleuit"}],"meta":{}}`))
	})
	items, err := svc.ListDesa(context.Background(), "327308")
	if err != nil {
		t.Fatalf("tidak boleh error: %v", err)
	}
	if len(items) != 2 || items[0].Kode != "3273081001" || items[0].Nama != "Hegarmanah" || items[1].Kode != "3273081002" {
		t.Fatalf("normalisasi salah: %+v", items)
	}
}

func TestListDesaCache(t *testing.T) {
	svc, hits := newKecamatanSvc(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":[{"code":"32.73.08.1001","name":"Hegarmanah"}]}`))
	})
	for i := 0; i < 2; i++ {
		if _, err := svc.ListDesa(context.Background(), "327308"); err != nil {
			t.Fatal(err)
		}
	}
	if *hits != 1 {
		t.Fatalf("harap 1 hit upstream (cache), dapat %d", *hits)
	}
}

func TestListDesaFailOpen(t *testing.T) {
	cases := map[string]http.HandlerFunc{
		"status-500": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		},
		"json-rusak": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{bukan json`))
		},
		"redirect": func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "http://example.com/x.json", http.StatusFound)
		},
	}
	for name, h := range cases {
		svc, _ := newKecamatanSvc(t, h)
		items, err := svc.ListDesa(context.Background(), "327308")
		if err != nil {
			t.Fatalf("%s: harus fail-open tanpa error: %v", name, err)
		}
		if len(items) != 0 {
			t.Fatalf("%s: harap daftar kosong, dapat %+v", name, items)
		}
	}
}

func TestListDesaItemTakValidDilewati(t *testing.T) {
	svc, _ := newKecamatanSvc(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":[
			{"code":"32.73.08.1001","name":"Hegarmanah"},
			{"code":"salah","name":"X"},
			{"code":"32.73.08","name":"TerlaluPendek"},
			{"code":"32.73.08.1002","name":""},
			{"code":"32.73.08.1003","name":"A<b"},
			{"code":"32.73.08.1004","name":"` + strings.Repeat("z", 101) + `"}
		]}`))
	})
	items, err := svc.ListDesa(context.Background(), "327308")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Kode != "3273081001" {
		t.Fatalf("hanya item valid yang dipakai: %+v", items)
	}
}

func TestListDesaTimeoutFailOpen(t *testing.T) {
	svc, _ := newKecamatanSvc(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	})
	svc.httpClient.Timeout = 200 * time.Millisecond
	items, err := svc.ListDesa(context.Background(), "327308")
	if err != nil {
		t.Fatalf("timeout harus fail-open: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("harap kosong saat timeout: %+v", items)
	}
}

func TestListDesaKosongTidakDiCache(t *testing.T) {
	svc, hits := newKecamatanSvc(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":[]}`))
	})
	for i := 0; i < 2; i++ {
		items, err := svc.ListDesa(context.Background(), "327308")
		if err != nil || len(items) != 0 {
			t.Fatalf("harap kosong tanpa error: %+v, %v", items, err)
		}
	}
	if *hits != 2 {
		t.Fatalf("hasil kosong jangan di-cache (harap 2 hit, dapat %d)", *hits)
	}
}

func TestKoreksiNamaUpstream(t *testing.T) {
	svc, _ := newKecamatanSvc(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":[
			{"code":"32.17.02.2002","name":"Cihanjuang"},
			{"code":"32.17.02.2003","name":"Cihanjuangrahayu"}
		]}`))
	})
	items, err := svc.ListDesa(context.Background(), "321702")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("harap 2 item: %+v", items)
	}
	if items[0].Nama != "Cihanjuang" {
		t.Fatalf("nama benar jangan diubah: %+v", items[0])
	}
	if items[1].Kode != "3217022003" || items[1].Nama != "Cihanjuang Rahayu" {
		t.Fatalf("koreksi gagal: %+v", items[1])
	}
}
