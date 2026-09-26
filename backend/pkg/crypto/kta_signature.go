package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// VerifyKTASignature memastikan key KTA yang diterima valid untuk payload tertentu.
func VerifyKTASignature(nia, tanggalAngkat string, anggotaID int, signature, keyHex string) bool {
	expected, err := KTASignature(nia, tanggalAngkat, anggotaID, keyHex)
	if err != nil {
		return false
	}

	actual, err := hex.DecodeString(signature)
	if err != nil {
		return false
	}

	expectedBytes, err := hex.DecodeString(expected)
	if err != nil {
		return false
	}

	return hmac.Equal(actual, expectedBytes)
}

// KTASignatureV2 menghasilkan tanda tangan HMAC-SHA256 untuk payload KTA.
func KTASignatureV2(nia, tanggalAngkat string, anggotaID int, keyHex string) (string, error) {
	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return "", fmt.Errorf("KTA signing key tidak valid: %w", err)
	}

	input := fmt.Sprintf("%s:%s:%d", nia, tanggalAngkat, anggotaID)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(input))
	return hex.EncodeToString(mac.Sum(nil)), nil
}
