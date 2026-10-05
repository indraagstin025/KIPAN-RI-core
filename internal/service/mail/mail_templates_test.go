package mail

// Uji Batch 3: template email (isi wajib + escaping HTML).

import (
	"strings"
	"testing"
)

func TestPasswordResetEmail(t *testing.T) {
	c := PasswordResetEmail("Budi", "https://sim.kipan.id/reset-password?token=abc")
	if !strings.Contains(c.Subject, "Reset") {
		t.Fatalf("subjek salah: %s", c.Subject)
	}
	for _, body := range []string{c.TextBody, c.HTMLBody} {
		if !strings.Contains(body, "Budi") || !strings.Contains(body, "token=abc") {
			t.Fatalf("body tidak memuat nama/tautan: %s", body)
		}
	}
}

func TestRevisionTokenEmail(t *testing.T) {
	c := RevisionTokenEmail("Siti", "REG-202610-00001", "rahasia-123", "https://sim.kipan.id")
	for _, body := range []string{c.TextBody, c.HTMLBody} {
		if !strings.Contains(body, "rahasia-123") || !strings.Contains(body, "REG-202610-00001") {
			t.Fatalf("body tidak memuat token/nomor: %s", body)
		}
	}
	if !strings.Contains(c.TextBody, "https://sim.kipan.id/revisi?nomor=REG-202610-00001") {
		t.Fatalf("tautan revisi salah: %s", c.TextBody)
	}
}

func TestStatusEmail(t *testing.T) {
	perbaikan := StatusEmail("PERBAIKAN", "Andi", "REG-1", "foto buram", "", "https://sim.kipan.id")
	if !strings.Contains(perbaikan.TextBody, "foto buram") || !strings.Contains(perbaikan.Subject, "Perbaikan") {
		t.Fatalf("email PERBAIKAN salah: %+v", perbaikan)
	}
	setujui := StatusEmail("DISETUJUI", "Andi", "REG-1", "", "KIPAN-IND-3273-2026-000001", "https://sim.kipan.id")
	if !strings.Contains(setujui.TextBody, "KIPAN-IND-3273-2026-000001") {
		t.Fatalf("email DISETUJUI harus memuat NIA: %s", setujui.TextBody)
	}
	tolak := StatusEmail("DITOLAK", "Andi", "REG-1", "data tidak sesuai", "", "https://sim.kipan.id")
	if !strings.Contains(tolak.TextBody, "data tidak sesuai") || !strings.Contains(tolak.Subject, "Ditolak") {
		t.Fatalf("email DITOLAK salah: %+v", tolak)
	}
}

func TestStatusEmailHTMLEscaped(t *testing.T) {
	// Nama applicant bisa berisi karakter HTML (input pengguna) — wajib
	// di-escape di body HTML, tidak boleh menjadi tag aktif.
	c := StatusEmail("DITOLAK", "<script>alert(1)</script>", "REG-1", "<img src=x onerror=alert(1)>", "", "https://x.id")
	if strings.Contains(c.HTMLBody, "<script>") {
		t.Fatal("HTML body tidak boleh memuat <script> mentah")
	}
	if !strings.Contains(c.HTMLBody, "&lt;script&gt;") {
		t.Fatalf("HTML body harus memuat versi ter-escape: %s", c.HTMLBody)
	}
	if strings.Contains(c.HTMLBody, "<img src=x") {
		t.Fatal("catatan berbahaya tidak boleh menjadi tag mentah")
	}
}
