package gateway

// Uji Batch 2: normalisasi nomor + FonnteGateway (httptest) + LogGateway.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNormalizeWATarget(t *testing.T) {
	cases := map[string]string{
		"081234567890":   "6281234567890",
		"6281234567890":  "6281234567890",
		"+6281234567890": "6281234567890",
		"08-12 3456-7890": "6281234567890",
		"0812":           "62812",
	}
	for in, want := range cases {
		if got := NormalizeWATarget(in); got != want {
			t.Errorf("NormalizeWATarget(%q) = %q, harap %q", in, got, want)
		}
	}
}

func TestFonnteGatewayKirimSukses(t *testing.T) {
	var gotTarget, gotMessage, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/send" {
			t.Errorf("path salah: %s", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		_ = r.ParseForm()
		gotTarget = r.Form.Get("target")
		gotMessage = r.Form.Get("message")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":true,"detail":"success"}`))
	}))
	defer srv.Close()

	g := NewFonnteGatewayWithURL("token-1", srv.URL)
	if err := g.SendMessage(context.Background(), "081234567890", "halo kader"); err != nil {
		t.Fatalf("kirim gagal: %v", err)
	}
	if gotAuth != "token-1" {
		t.Errorf("Authorization salah: %q", gotAuth)
	}
	if gotTarget != "6281234567890" {
		t.Errorf("target ternormalisasi salah: %q", gotTarget)
	}
	if gotMessage != "halo kader" {
		t.Errorf("message salah: %q", gotMessage)
	}
}

func TestFonnteGatewayStatusFalse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":false,"reason":"token tidak valid"}`))
	}))
	defer srv.Close()

	g := NewFonnteGatewayWithURL("token-1", srv.URL)
	if err := g.SendMessage(context.Background(), "081234567890", "x"); err == nil {
		t.Fatal("status=false harus menjadi error")
	}
}

func TestFonnteGatewayHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`unauthorized`))
	}))
	defer srv.Close()

	g := NewFonnteGatewayWithURL("token-1", srv.URL)
	if err := g.SendMessage(context.Background(), "081234567890", "x"); err == nil {
		t.Fatal("HTTP 401 harus menjadi error")
	}
}

func TestFonnteGatewayTokenKosong(t *testing.T) {
	g := NewFonnteGateway("")
	if err := g.SendMessage(context.Background(), "081234567890", "x"); err == nil {
		t.Fatal("token kosong harus menjadi error")
	}
}

func TestLogGatewaySelaluSukses(t *testing.T) {
	g := NewLogGateway()
	if err := g.SendMessage(context.Background(), "081234567890", "pesan uji"); err != nil {
		t.Fatalf("LogGateway tidak boleh error: %v", err)
	}
}
