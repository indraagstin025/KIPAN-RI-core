package domain

// Regression test keamanan (SEC-AGT-ENUM): DTO cek anggota publik hanya boleh
// memuat {nia, nama_lengkap, status, provinsi_nama, kabupaten_nama} — sesuai
// keputusan pemilik "nama pendaftar + wilayah + status saja". Field lain
// (tanggal_angkat, NIK, kontak, alamat, object key) wajib ABSEN.

import (
	"encoding/json"
	"testing"
)

func TestAnggotaPublicInfoWhitelistOnly(t *testing.T) {
	blob, err := json.Marshal(AnggotaPublicInfo{
		NIA:           "KIPAN-32-3273-2026-00001",
		NamaLengkap:   "Rizki Pratama",
		Status:        "AKTIF",
		ProvinsiNama:  "JAWA BARAT",
		KabupatenNama: "KOTA BANDUNG",
	})
	if err != nil {
		t.Fatalf("marshal gagal: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(blob, &got); err != nil {
		t.Fatalf("unmarshal gagal: %v", err)
	}

	allowed := map[string]bool{
		"nia": true, "nama_lengkap": true, "status": true,
		"provinsi_nama": true, "kabupaten_nama": true,
	}
	for k := range got {
		if !allowed[k] {
			t.Fatalf("DTO cek anggota publik memuat field tak diizinkan %q", k)
		}
	}

	for _, forbidden := range []string{
		"tanggal_angkat", "nik", "nik_hash", "nik_encrypted",
		"email", "whatsapp", "alamat", "ktp_key", "foto_key",
	} {
		if _, ok := got[forbidden]; ok {
			t.Fatalf("field %q harus ABSEN dari cek anggota publik", forbidden)
		}
	}
}
