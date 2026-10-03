package gateway

// Uji Batch 3: MIME builder + LogMailSender + masking.

import (
	"context"
	"strings"
	"testing"
)

func TestBuildMIMEMessagePlain(t *testing.T) {
	msg := string(BuildMIMEMessage("KIPAN", "no-reply@kipan.id", "user@example.com", "Halo Kader", "baris satu\nbaris dua", ""))
	for _, want := range []string{
		"From: KIPAN <no-reply@kipan.id>",
		"To: user@example.com",
		"Subject: ",
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=\"utf-8\"",
		"baris satu\r\nbaris dua",
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("pesan tidak memuat %q:\n%s", want, msg)
		}
	}
	if strings.Contains(msg, "multipart/alternative") {
		t.Fatal("tanpa HTML tidak boleh multipart")
	}
}

func TestBuildMIMEMessageMultipart(t *testing.T) {
	msg := string(BuildMIMEMessage("", "no-reply@kipan.id", "user@example.com", "Subjek", "teks", "<p>html</p>"))
	if !strings.Contains(msg, "multipart/alternative") {
		t.Fatal("dengan HTML harus multipart/alternative")
	}
	if !strings.Contains(msg, "text/html") || !strings.Contains(msg, "<p>html</p>") {
		t.Fatal("bagian HTML tidak ditemukan")
	}
	// Subjek non-ASCII di-encode (RFC 2047) agar tidak rusak.
	encoded := string(BuildMIMEMessage("", "a@b.co", "c@d.co", "Reset Kata Sandi — KIPAN", "x", ""))
	if !strings.Contains(encoded, "Subject: =?utf-8?") {
		t.Fatalf("subjek non-ASCII harus di-encode: %s", encoded)
	}
}

func TestLogMailSenderDanMaskEmail(t *testing.T) {
	if err := NewLogMailSender().Send(context.Background(), "kader@example.com", "Subjek", "isi", ""); err != nil {
		t.Fatalf("LogMailSender tidak boleh error: %v", err)
	}
	if got := MaskEmail("kader@example.com"); strings.Contains(got, "der@") {
		t.Fatalf("email tidak dimask: %s", got)
	}
	if got := MaskEmail("bukan-email"); got != "***" {
		t.Fatalf("input tanpa @ harus jadi ***: %s", got)
	}
}

func TestSMTPSenderTolakCRLFDiTujuan(t *testing.T) {
	s := NewSMTPSender(MailConfig{Host: "127.0.0.1", Port: 1025, FromEmail: "no-reply@kipan.id"})
	for _, to := range []string{"a@b.co\r\nBcc: x@y.zz", "a@b.co\nBcc: x@y.zz"} {
		if err := s.Send(context.Background(), to, "Subjek", "isi", ""); err == nil {
			t.Fatalf("tujuan mengandung CRLF harus ditolak: %q", to)
		}
	}
}
