package crypto

import (
	"testing"
)

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

func TestMaskNIK(t *testing.T) {
	nik := "3204123456780001"
	masked := MaskNIK(nik)

	expected := "3204************"
	if masked != expected {
		t.Errorf("Expected %s, got %s", expected, masked)
	}
}
