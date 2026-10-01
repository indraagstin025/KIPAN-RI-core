package domain

// Regression test keamanan (SEC-TRACK-PII): DTO pelacakan publik TIDAK boleh
// memuat PII (nama/wilayah/catatan/timeline) — nomor REG bersifat sekuensial
// dan dapat dienumerasi. Test ini mengunci bentuk respons publik tanpa perlu DB.

import (
	"encoding/json"
	"testing"
)

func TestTrackingResponsePublicExposesNoPII(t *testing.T) {
	blob, err := json.Marshal(PendaftaranTrackingResponse{
		NomorPendaftaran: "REG-202610-00042",
		Status:           "DIVERIFIKASI",
		StatusLabel:      "Sedang Diverifikasi",
	})
	if err != nil {
		t.Fatalf("marshal gagal: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(blob, &got); err != nil {
		t.Fatalf("unmarshal gagal: %v", err)
	}

	allowed := map[string]bool{
		"nomor_pendaftaran": true,
		"status":            true,
		"status_label":      true,
		"created_at":        true,
		"updated_at":        true,
	}
	for k := range got {
		if !allowed[k] {
			t.Fatalf("DTO tracking publik membocorkan field %q", k)
		}
	}

	for _, forbidden := range []string{
		"nama_lengkap", "provinsi_nama", "kabupaten_nama", "catatan", "timeline",
	} {
		if _, ok := got[forbidden]; ok {
			t.Fatalf("field PII %q harus ABSEN dari respons tracking publik", forbidden)
		}
	}
}
