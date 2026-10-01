package crypto

import (
	"crypto/rand"
	"encoding/hex"
	"testing"
)

func mustTestKey(t *testing.T) string {
	t.Helper()
	kb := make([]byte, 32)
	if _, err := rand.Read(kb); err != nil {
		t.Fatalf("gagal generate kunci uji: %v", err)
	}
	return hex.EncodeToString(kb)
}

func TestHashToken(t *testing.T) {
	rawToken := "test-token-secret-12345"
	h1 := HashToken(rawToken)
	h2 := HashToken(rawToken)

	if h1 == "" || len(h1) != 64 {
		t.Fatalf("Expected 64-character hex hash, got %s", h1)
	}
 
	if h1 != h2 {
		t.Errorf("Deterministic hash mismatch: %s != %s", h1, h2)
	}
}

func TestKTASignatureRoundtrip(t *testing.T) {
	keyHex := mustTestKey(t)
	sig, err := KTASignature("KIPAN-32-3273-2026-00001", "2026-09-27", 42, keyHex)
	if err != nil {
		t.Fatalf("KTASignature gagal: %v", err)
	}
	if len(sig) != 64 {
		t.Fatalf("signature harus 64 hex, dapat %d", len(sig))
	}
	if err := VerifyKTASignature("KIPAN-32-3273-2026-00001", "2026-09-27", 42, sig, keyHex); err != nil {
		t.Errorf("signature valid ditolak: %v", err)
	}
}

func TestKTASignatureDeterministic(t *testing.T) {
	keyHex := mustTestKey(t)
	s1, _ := KTASignature("KIPAN-32-3273-2026-00001", "2026-09-27", 42, keyHex)
	s2, _ := KTASignature("KIPAN-32-3273-2026-00001", "2026-09-27", 42, keyHex)
	if s1 != s2 {
		t.Errorf("signature harus deterministik: %s != %s", s1, s2)
	}
}

func TestVerifyKTASignatureTampered(t *testing.T) {
	keyHex := mustTestKey(t)
	sig, err := KTASignature("KIPAN-32-3273-2026-00001", "2026-09-27", 42, keyHex)
	if err != nil {
		t.Fatalf("KTASignature gagal: %v", err)
	}
	// Ubah 1 karakter hex terakhir (tetap valid hex agar lolos decode).
	tampered := sig[:63]
	if sig[63] == '0' {
		tampered += "1"
	} else {
		tampered += "0"
	}
	if err := VerifyKTASignature("KIPAN-32-3273-2026-00001", "2026-09-27", 42, tampered, keyHex); err == nil {
		t.Error("signature palsu DITERIMA — verifikasi bocor")
	}
}

func TestVerifyKTASignatureWrongKey(t *testing.T) {
	sig, err := KTASignature("KIPAN-32-3273-2026-00001", "2026-09-27", 42, mustTestKey(t))
	if err != nil {
		t.Fatalf("KTASignature gagal: %v", err)
	}
	if err := VerifyKTASignature("KIPAN-32-3273-2026-00001", "2026-09-27", 42, sig, mustTestKey(t)); err == nil {
		t.Error("signature dengan kunci salah DITERIMA")
	}
}

func TestVerifyKTASignatureMalformed(t *testing.T) {
	keyHex := mustTestKey(t)
	for _, bad := range []string{"", "bukan-hex!!", "00", "zzzz"} {
		if err := VerifyKTASignature("KIPAN-32-3273-2026-00001", "2026-09-27", 42, bad, keyHex); err == nil {
			t.Errorf("signature malformed %q DITERIMA", bad)
		}
	}
}

func TestMaskNIK(t *testing.T) {
	nik := "3204123456780001"
	masked := MaskNIK(nik)

	expected := "3204************"
	if masked != expected {
		t.Errorf("Expected %s, got %s", expected, masked)
	}
}
