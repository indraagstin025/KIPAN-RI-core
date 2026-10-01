package storage

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestNewClientRejectsEmptyConfig(t *testing.T) {
	for _, tc := range []struct {
		name                       string
		endpoint, region, ak, sk   string
	}{
		{"endpoint kosong", "", "auto", "ak", "sk"},
		{"access key kosong", "http://127.0.0.1:9000", "auto", "", "sk"},
		{"secret key kosong", "http://127.0.0.1:9000", "auto", "ak", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewClient(tc.endpoint, tc.region, tc.ak, tc.sk); err == nil {
				t.Fatal("expected error untuk konfigurasi kosong")
			}
		})
	}
}

func TestValidateMagicBytes(t *testing.T) {
	jpeg := append(append([]byte{}, magicJPEG...), 0x00, 0x01)
	png := append(append([]byte{}, magicPNG...), 0x00)
	pdf := append([]byte{}, append([]byte("%PDF-1.7\n"), 0x25)...)
	for _, tc := range []struct {
		name string
		data []byte
		mime string
		ok   bool
	}{
		{"jpeg valid", jpeg, "image/jpeg", true},
		{"png valid", png, "image/png", true},
		{"pdf valid", pdf, "application/pdf", true},
		{"jpeg sebagai png", jpeg, "image/png", false},
		{"png sebagai jpeg", png, "image/jpeg", false},
		{"teks sebagai pdf", []byte("hello world, ini bukan pdf"), "application/pdf", false},
		{"php polyglot sebagai jpeg", []byte("<?php echo 1; ?>"), "image/jpeg", false},
		{"kosong", []byte{}, "image/jpeg", false},
		{"mime tak dikenal", jpeg, "application/x-sh", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateMagicBytes(tc.data, tc.mime)
			if tc.ok && err != nil {
				t.Fatalf("expected valid, got %v", err)
			}
			if !tc.ok && err == nil {
				t.Fatal("expected rejection untuk konten mismatch")
			}
		})
	}
}

// Presign tidak memanggil jaringan (signing lokal) sehingga bisa diuji
// offline terhadap endpoint dummy.
func TestPresignOffline(t *testing.T) {
	c, err := NewClient("http://127.0.0.1:9000", "auto", "minio_admin", "minio_secret")
	if err != nil {
		t.Fatalf("NewClient gagal: %v", err)
	}
	ctx := context.Background()

	putURL, err := c.PresignPut(ctx, "kipan-uploads", "uploads/pendaftaran/202609/uuid.jpg", "image/jpeg", 10*time.Minute)
	if err != nil {
		t.Fatalf("PresignPut gagal: %v", err)
	}
	if !strings.Contains(putURL, "127.0.0.1:9000/kipan-uploads/uploads/pendaftaran/202609/uuid.jpg") {
		t.Fatalf("presign URL tidak memuat bucket+key yang benar: %s", putURL)
	}
	if !strings.Contains(putURL, "X-Amz-Signature=") {
		t.Fatalf("presign URL tanpa signature: %s", putURL)
	}

	getURL, err := c.PresignGet(ctx, "kipan-private", "uploads/pendaftaran/202609/uuid.jpg", 5*time.Minute)
	if err != nil {
		t.Fatalf("PresignGet gagal: %v", err)
	}
	if !strings.Contains(getURL, "kipan-private") || !strings.Contains(getURL, "X-Amz-Expires=300") {
		t.Fatalf("presign GET URL salah (bucket/TTL): %s", getURL)
	}
}

// Stat ke server mati harus error (bukan panic/nil) — fail-closed.
func TestStatUnreachableFailsClosed(t *testing.T) {
	c, err := NewClient("http://127.0.0.1:9", "auto", "ak", "sk")
	if err != nil {
		t.Fatalf("NewClient gagal: %v", err)
	}
	if _, err := c.Stat(context.Background(), "b", "k"); err == nil {
		t.Fatal("expected error saat storage unreachable")
	}
}
