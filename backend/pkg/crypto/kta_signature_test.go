package crypto

import "testing"

func TestKTASignatureRoundTrip(t *testing.T) {
	const secret = "00112233445566778899aabbccddeeff"
	const nia = "KIPAN.32.3273.2026.0001"
	const tanggalAngkat = "2026-09-27"
	const anggotaID = 42

	sig, err := KTASignature(nia, tanggalAngkat, anggotaID, secret)
	if err != nil {
		t.Fatalf("KTASignature returned error: %v", err)
	}

	if sig == "" {
		t.Fatal("signature should not be empty")
	}

	if !VerifyKTASignature(nia, tanggalAngkat, anggotaID, sig, secret) {
		t.Fatal("expected valid KTA signature to verify")
	}

	if VerifyKTASignature(nia, tanggalAngkat, anggotaID+1, sig, secret) {
		t.Fatal("signature should fail when payload changes")
	}
}
