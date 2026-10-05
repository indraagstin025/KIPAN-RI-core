package wilayah

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newKodeposSvc(t *testing.T, handler http.HandlerFunc) (*wilayahSvc, *int) {
	t.Helper()
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		handler(w, r)
	}))
	t.Cleanup(srv.Close)
	svc := NewWilayahService(nil).(*wilayahSvc)
	svc.posBase = srv.URL
	svc.httpClient = srv.Client()
	return svc, &hits
}

const kodeposSample = `{"success":true,"data":[
	{"postalCode":"40141","district":{"name":"Parongpong"},"city":{"name":"Kabupaten Bandung Barat"},"province":{"name":"Jawa Barat"},"village":{"name":"Karyawangi"}},
	{"postalCode":"40142","district":{"name":"Cidadap"},"city":{"name":"Kota Bandung"},"province":{"name":"Jawa Barat"},"village":{"name":"Ciumbuleuit"}},
	{"postalCode":"40141","district":{"name":"Parongpong"},"city":{"name":"Kabupaten Bandung Barat"},"province":{"name":"Jawa Barat"},"village":{"name":"Karyawangi Dup"}}
]}`

func TestListKodeposTolakKosong(t *testing.T) {
	svc := NewWilayahService(nil)
	for _, tc := range [][3]string{{"", "", ""}, {"", "", "x"}, {"<a>", "", ""}} {
		if _, err := svc.ListKodepos(context.Background(), tc[0], tc[1], tc[2]); err == nil {
			t.Fatalf("harus ditolak: %+v", tc)
		}
	}
}

func TestListKodeposDesaPalingCocokDulu(t *testing.T) {
	svc, _ := newKodeposSvc(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "q=Karyawangi") {
			t.Errorf("query harus pakai desa: %s", r.URL.RawQuery)
		}
		w.Write([]byte(kodeposSample))
	})
	items, err := svc.ListKodepos(context.Background(), "Karyawangi", "Parongpong", "Kabupaten Bandung Barat")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].KodePos != "40141" {
		t.Fatalf("harap hanya 40141 (cocok desa+konteks): %+v", items)
	}
}

func TestListKodeposFallbackKecamatan(t *testing.T) {
	svc, _ := newKodeposSvc(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(kodeposSample))
	})
	items, err := svc.ListKodepos(context.Background(), "TidakAda", "Cidadap", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].KodePos != "40142" {
		t.Fatalf("harap fallback kecamatan 40142: %+v", items)
	}
}

func TestListKodeposTanpaCocokKosong(t *testing.T) {
	svc, _ := newKodeposSvc(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(kodeposSample))
	})
	items, err := svc.ListKodepos(context.Background(), " antah ", " berantah ", " antah ")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("harap kosong bila tak cocok: %+v", items)
	}
}

func TestListKodeposFailOpen(t *testing.T) {
	cases := map[string]http.HandlerFunc{
		"status-500": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		},
		"json-rusak": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{bukan`))
		},
		"success-false": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"success":false,"data":[]}`))
		},
	}
	for name, h := range cases {
		svc, _ := newKodeposSvc(t, h)
		items, err := svc.ListKodepos(context.Background(), "Karyawangi", "", "")
		if err != nil {
			t.Fatalf("%s: harus fail-open: %v", name, err)
		}
		if len(items) != 0 {
			t.Fatalf("%s: harap kosong: %+v", name, items)
		}
	}
}

func TestListKodeposCacheDanTimeout(t *testing.T) {
	svc, hits := newKodeposSvc(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"success":true,"data":[{"postalCode":"40559","village":{"name":"Karyawangi"}}]}`))
	})
	for i := 0; i < 2; i++ {
		if _, err := svc.ListKodepos(context.Background(), "Karyawangi", "", ""); err != nil {
			t.Fatal(err)
		}
	}
	if *hits != 1 {
		t.Fatalf("harap 1 hit (cache), dapat %d", *hits)
	}

	slow, _ := newKodeposSvc(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	})
	slow.httpClient.Timeout = 200 * time.Millisecond
	items, err := slow.ListKodepos(context.Background(), "Karyawangi", "", "")
	if err != nil || len(items) != 0 {
		t.Fatalf("timeout harus fail-open kosong: %+v, %v", items, err)
	}
}

func TestListKodeposSpasiVsTanpaSpasi(t *testing.T) {
	// Regresi: upstream mengindeks "Cihanjuangrahayu" (tanpa spasi) sementara
	// form mengirim "Cihanjuang Rahayu" (dengan spasi, hasil koreksi wilayah.id).
	svc, _ := newKodeposSvc(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"success":true,"data":[
			{"postalCode":"40559","district":{"name":"Parongpong"},"city":{"name":"Kabupaten Bandung Barat"},"village":{"name":"Cihanjuangrahayu"}}
		]}`))
	})
	items, err := svc.ListKodepos(context.Background(), "Cihanjuang Rahayu", "Parongpong", "Kabupaten Bandung Barat")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].KodePos != "40559" {
		t.Fatalf("ejaan spasi harus cocok: %+v", items)
	}
}
