package storage

// Uji L8: deteksi PDF terkunci via penanda /Encrypt di trailer.

import "testing"

func TestLooksEncryptedPDF(t *testing.T) {
	encrypted := []byte("%PDF-1.7\n1 0 obj<</Pages 2 0 R/Encrypt 5 0 R>>\nstartxref\n12345\n%%EOF trailer<</Size 6/Encrypt 5 0 R/Root 1 0 R>>startxref\n999\n%%EOF")
	if !LooksEncryptedPDF(encrypted) {
		t.Fatal("PDF dengan /Encrypt di trailer harus terdeteksi")
	}

	bersih := []byte("%PDF-1.4\n1 0 obj<</Pages 2 0 R>>\nstartxref\n100\n%%EOF")
	if LooksEncryptedPDF(bersih) {
		t.Fatal("PDF normal tidak boleh ditandai terenkripsi")
	}

	// Kata "Encryption" tanpa slash bukan penanda kamus enkripsi.
	hampir := []byte("startxref\n0\n%%EOF ... Encryption is important ...")
	if LooksEncryptedPDF(hampir) {
		t.Fatal("substring tanpa slash tidak boleh cocok")
	}

	// /Encrypted (bukan /Encrypt + delimiter) bukan penanda.
	salah := []byte("startxref\n0\n%%EOF /Encrypted<<>>")
	if LooksEncryptedPDF(salah) {
		t.Fatal("/Encrypted tanpa delimiter harus ditolak")
	}

	if LooksEncryptedPDF(nil) || LooksEncryptedPDF([]byte{}) {
		t.Fatal("input kosong harus false")
	}

	// Tanpa startxref: tetap pindai seluruh ekor (PDF terpotong/deviasi).
	tanpaXref := []byte(" trailer /Encrypt 3 0 R ")
	if !LooksEncryptedPDF(tanpaXref) {
		t.Fatal("tanpa startxref tetap harus terdeteksi bila /Encrypt ada")
	}
}
