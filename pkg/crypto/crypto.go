package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
)
 
// BlindIndex menghasilkan HMAC-SHA256 dari NIK menggunakan blind index key.
// Digunakan untuk pencarian NIK tanpa mendekripsi data.
func BlindIndex(nik, keyHex string) (string, error) {
	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return "", fmt.Errorf("blind index key tidak valid: %w", err)
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(nik))
	return hex.EncodeToString(mac.Sum(nil)), nil
}

// EncryptAESGCM mengenkripsi plaintext menggunakan AES-256-GCM.
// Format output: base64(nonce):base64(sealed) dengan sealed = ciphertext+tag
// GCM (L-8: komentar lama keliru menulis 3 segmen terpisah).
func EncryptAESGCM(plaintext, keyHex string) (string, error) {
	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return "", fmt.Errorf("AES key tidak valid: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("gagal membuat AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("gagal membuat GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("gagal generate nonce: %w", err)
	}

	// Seal menghasilkan ciphertext + auth tag
	sealed := gcm.Seal(nil, nonce, []byte(plaintext), nil)

	nonceB64 := base64.StdEncoding.EncodeToString(nonce)
	sealedB64 := base64.StdEncoding.EncodeToString(sealed)

	return nonceB64 + ":" + sealedB64, nil
}

// DecryptAESGCM mendekripsi ciphertext yang dihasilkan oleh EncryptAESGCM.
func DecryptAESGCM(encrypted, keyHex string) (string, error) {
	parts := strings.SplitN(encrypted, ":", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("format ciphertext tidak valid")
	}

	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return "", fmt.Errorf("AES key tidak valid: %w", err)
	}

	nonce, err := base64.StdEncoding.DecodeString(parts[0])
	if err != nil {
		return "", fmt.Errorf("gagal decode nonce: %w", err)
	}

	sealed, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("gagal decode ciphertext: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("gagal membuat AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("gagal membuat GCM: %w", err)
	}

	plaintext, err := gcm.Open(nil, nonce, sealed, nil)
	if err != nil {
		return "", fmt.Errorf("dekripsi gagal (data mungkin dimanipulasi): %w", err)
	}

	return string(plaintext), nil
}

// KTASignature menghasilkan HMAC-SHA256 untuk QR Code KTA.
// Input: nia:tanggalAngkat:anggotaID
func KTASignature(nia, tanggalAngkat string, anggotaID int, keyHex string) (string, error) {
	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return "", fmt.Errorf("KTA signing key tidak valid: %w", err)
	}
	input := fmt.Sprintf("%s:%s:%d", nia, tanggalAngkat, anggotaID)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(input))
	return hex.EncodeToString(mac.Sum(nil)), nil
}

// VerifyKTASignature memverifikasi signature QR Code KTA (RULES 20).
// Menghitung ulang HMAC dari data canonical yang sama dengan KTASignature
// lalu membandingkan dengan hmac.Equal (constant-time) agar tidak bocor
// informasi via timing side-channel. Semua jalur gagal mengembalikan pesan
// yang sama agar tidak menjadi oracle (publik cukup tahu VALID/TIDAK).
func VerifyKTASignature(nia, tanggalAngkat string, anggotaID int, signatureHex, keyHex string) error {
	expected, err := KTASignature(nia, tanggalAngkat, anggotaID, keyHex)
	if err != nil {
		return err
	}
	sigBytes, err := hex.DecodeString(signatureHex)
	if err != nil {
		return fmt.Errorf("signature KTA tidak valid")
	}
	expectedBytes, err := hex.DecodeString(expected)
	if err != nil {
		return fmt.Errorf("signature KTA tidak valid")
	}
	if !hmac.Equal(sigBytes, expectedBytes) {
		return fmt.Errorf("signature KTA tidak valid")
	}
	return nil
}

// MaskNIK menyembunyikan 12 digit terakhir NIK untuk tampilan UI.
// Contoh: "3204123456780001" -> "3204************"
func MaskNIK(nik string) string {
	if len(nik) <= 4 {
		return strings.Repeat("*", len(nik))
	}
	return nik[:4] + strings.Repeat("*", len(nik)-4)
}

// GenerateSecureToken menghasilkan random bytes yang di-hex encode untuk OTP/token.
func GenerateSecureToken(byteLength int) (string, error) {
	b := make([]byte, byteLength)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("gagal generate secure token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// HashToken menghasilkan SHA-256 hash dari token (untuk disimpan di DB).
func HashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
