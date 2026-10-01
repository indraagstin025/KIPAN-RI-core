package service

import (
	"context"
	"testing"

	"github.com/kipan-indonesia/sim-kipan-core/config"
)

// Service dengan client nil: validasi input tetap jalan (422), operasi yang
// butuh S3 gagal fail-closed 503.
func newUnconfiguredStorageService() *StorageService {
	return NewStorageService(&config.Config{}, nil, nil, nil)
}

func TestRequestUploadPresignRejectsInvalidInput(t *testing.T) {
	svc := newUnconfiguredStorageService()
	ctx := context.Background()

	for _, tc := range []struct {
		name     string
		category string
		mime     string
		size     int64
	}{
		{"kategori tak dikenal", "kk", "image/jpeg", 1000},
		{"kategori kosong", "", "image/jpeg", 1000},
		{"mime tak diizinkan untuk foto", "foto", "application/pdf", 1000},
		{"mime exe", "ktp", "application/x-msdownload", 1000},
		{"size nol", "foto", "image/jpeg", 0},
		{"size negatif", "foto", "image/jpeg", -5},
		{"foto 3MB", "foto", "image/jpeg", 3 << 20},
		{"ktp 5MB", "ktp", "image/jpeg", 5 << 20},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := svc.RequestUploadPresign(ctx, tc.category, "f.jpg", tc.mime, tc.size); err == nil {
				t.Fatal("expected rejection")
			}
		})
	}
}

func TestRequestUploadPresignFailClosedWithoutClient(t *testing.T) {
	svc := newUnconfiguredStorageService()
	if svc.Configured() {
		t.Fatal("expected Configured()==false tanpa client")
	}
	_, err := svc.RequestUploadPresign(context.Background(), "foto", "f.jpg", "image/jpeg", 1000)
	if err == nil {
		t.Fatal("expected 503 tanpa storage client")
	}
	if appErr, ok := err.(interface{ Error() string }); !ok || appErr == nil {
		t.Fatal("expected error non-nil")
	}
}

func TestVerifySubmittedObjectRejectsBadInput(t *testing.T) {
	svc := newUnconfiguredStorageService()
	ctx := context.Background()

	for _, tc := range []struct {
		name     string
		key      string
		category string
	}{
		{"kategori tak dikenal", "uploads/pendaftaran/202609/x.jpg", "kk"},
		{"key traversal", "../../etc/passwd", "foto"},
		{"key karakter ilegal", "uploads/ktp?.jpg", "ktp"},
		{"key kosong", "", "foto"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := svc.VerifySubmittedObject(ctx, tc.key, tc.category); err == nil {
				t.Fatal("expected rejection")
			}
		})
	}
}

func TestVerifySubmittedObjectFailClosedWithoutClient(t *testing.T) {
	svc := newUnconfiguredStorageService()
	err := svc.VerifySubmittedObject(context.Background(), "uploads/pendaftaran/202609/x.jpg", "foto")
	if err == nil {
		t.Fatal("expected 503 tanpa storage client")
	}
}

func TestCategoryPoliciesCoverAllSubmitKeys(t *testing.T) {
	// Setiap key submit pendaftaran wajib punya kategori kebijakan.
	for _, cat := range []string{"foto", "ktp", "cv", "sk", "surat_pernyataan", "surat_sehat"} {
		if _, err := policyFor(cat); err != nil {
			t.Fatalf("kategori %q tanpa kebijakan: %v", cat, err)
		}
	}
}
