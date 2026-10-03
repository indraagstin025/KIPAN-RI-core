package service

// Template email (Batch 3). Semua nilai yang berasal dari data pendaftar
// di-escape sebelum masuk HTML (anti injeksi konten ke email admin/anggota).
// Fungsi murni agar unit-testable tanpa gateway.

import (
	"html"
	"strings"
)

// EmailContent adalah isi email siap kirim.
type EmailContent struct {
	Subject  string
	TextBody string
	HTMLBody string
}

const emailSignature = "\n\n— Sistem Informasi KIPAN RI\nKader Inti Pemuda Anti Narkoba"

func htmlParagraphs(lines ...string) string {
	var b strings.Builder
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			b.WriteString("<p style=\"margin:0 0 12px\">&nbsp;</p>")
			continue
		}
		b.WriteString("<p style=\"margin:0 0 12px\">" + html.EscapeString(l) + "</p>")
	}
	return b.String()
}

func emailHTML(title string, lines []string, ctaLabel, ctaURL string) string {
	var b strings.Builder
	b.WriteString("<div style=\"font-family:Arial,Helvetica,sans-serif;max-width:560px;margin:0 auto;color:#172033\">")
	b.WriteString("<h2 style=\"color:#0D3F70;margin:0 0 16px\">" + html.EscapeString(title) + "</h2>")
	b.WriteString(htmlParagraphs(lines...))
	if ctaLabel != "" && ctaURL != "" {
		b.WriteString("<p style=\"margin:20px 0\"><a href=\"" + html.EscapeString(ctaURL) +
			"\" style=\"background:#0E6CAC;color:#fff;padding:12px 20px;border-radius:8px;text-decoration:none;font-weight:bold\">" +
			html.EscapeString(ctaLabel) + "</a></p>")
		b.WriteString("<p style=\"margin:0;color:#667085;font-size:13px\">Atau salin tautan ini ke browser: " +
			html.EscapeString(ctaURL) + "</p>")
	}
	b.WriteString("<p style=\"margin:24px 0 0;color:#667085;font-size:13px\">Sistem Informasi KIPAN RI — Kader Inti Pemuda Anti Narkoba</p>")
	b.WriteString("</div>")
	return b.String()
}

// PasswordResetEmail menyusun email reset password.
func PasswordResetEmail(nama, link string) EmailContent {
	if strings.TrimSpace(nama) == "" {
		nama = "Pengguna"
	}
	lines := []string{
		"Halo " + nama + ",",
		"Kami menerima permintaan reset kata sandi untuk akun Anda.",
		"Tautan berlaku 30 menit dan hanya dapat dipakai sekali. Abaikan email ini jika Anda tidak meminta reset.",
	}
	return EmailContent{
		Subject:  "Reset Kata Sandi — Sistem Informasi KIPAN",
		TextBody: "Halo " + nama + ",\n\nReset kata sandi akun Anda melalui tautan berikut (berlaku 30 menit, sekali pakai):\n" + link + emailSignature,
		HTMLBody: emailHTML("Reset Kata Sandi", lines, "Atur Kata Sandi Baru", link),
	}
}

// RevisionTokenEmail menyusun email berisi token revisi berkas.
func RevisionTokenEmail(nama, nomor, token, publicURL string) EmailContent {
	if strings.TrimSpace(nama) == "" {
		nama = "Pendaftar"
	}
	link := publicURL + "/revisi?nomor=" + nomor
	lines := []string{
		"Halo " + nama + ",",
		"Pendaftaran " + nomor + " memerlukan perbaikan berkas.",
		"Token revisi Anda: " + token,
		"Token berlaku 24 jam dan hanya dapat dipakai sekali. Buka halaman revisi, unggah berkas pengganti, lalu masukkan token di atas.",
	}
	return EmailContent{
		Subject: "Token Revisi Berkas " + nomor + " — KIPAN",
		TextBody: "Halo " + nama + ",\n\nPendaftaran " + nomor + " memerlukan perbaikan berkas.\n\n" +
			"Token revisi: " + token + "\n" + "Berlaku 24 jam, sekali pakai.\n\n" +
			"Halaman revisi: " + link + emailSignature,
		HTMLBody: emailHTML("Token Revisi Berkas", lines, "Buka Halaman Revisi", link),
	}
}

// StatusEmail menyusun email notifikasi perubahan status pendaftaran.
// status: PERBAIKAN | DISETUJUI | DITOLAK.
func StatusEmail(status, nama, nomor, catatan, nia, publicURL string) EmailContent {
	if strings.TrimSpace(nama) == "" {
		nama = "Pendaftar"
	}
	link := publicURL + "/lacak?nomor=" + nomor
	switch status {
	case "PERBAIKAN":
		lines := []string{
			"Halo " + nama + ",",
			"Pendaftaran " + nomor + " memerlukan perbaikan berkas.",
		}
		if strings.TrimSpace(catatan) != "" {
			lines = append(lines, "Catatan verifikator: "+catatan)
		}
		lines = append(lines, "Minta token revisi di halaman revisi (nomor + email + WhatsApp terdaftar), lalu unggah berkas pengganti.")
		return EmailContent{
			Subject:  "Perbaikan Berkas " + nomor + " — KIPAN",
			TextBody: "Halo " + nama + ",\n\nPendaftaran " + nomor + " memerlukan perbaikan berkas.\n" + catatanBlock(catatan) + "\nHalaman revisi: " + publicURL + "/revisi?nomor=" + nomor + emailSignature,
			HTMLBody: emailHTML("Perbaikan Berkas Diperlukan", lines, "Halaman Revisi", publicURL+"/revisi?nomor="+nomor),
		}

	case "DISETUJUI":
		lines := []string{
			"Halo " + nama + ",",
			"Selamat! Pendaftaran " + nomor + " telah DISETUJUI.",
			"Nomor Induk Anggota (NIA) Anda: " + nia,
			"Akun login Anda dibuat oleh admin. Password awal disampaikan melalui admin/kanal resmi — segera ganti setelah login pertama.",
		}
		return EmailContent{
			Subject: "Pendaftaran Disetujui — NIA " + nia,
			TextBody: "Halo " + nama + ",\n\nPendaftaran " + nomor + " telah DISETUJUI.\nNIA Anda: " + nia +
				"\n\nAkun login dibuat admin; password awal disampaikan lewat kanal resmi.\n" + emailSignature,
			HTMLBody: emailHTML("Pendaftaran Disetujui", lines, "Lacak Status", link),
		}

	case "DITOLAK":
		lines := []string{
			"Halo " + nama + ",",
			"Mohon maaf, pendaftaran " + nomor + " DITOLAK.",
		}
		if strings.TrimSpace(catatan) != "" {
			lines = append(lines, "Alasan: "+catatan)
		}
		return EmailContent{
			Subject:  "Pendaftaran Ditolak " + nomor + " — KIPAN",
			TextBody: "Halo " + nama + ",\n\nPendaftaran " + nomor + " DITOLAK.\n" + catatanBlock(catatan) + emailSignature,
			HTMLBody: emailHTML("Pendaftaran Ditolak", lines, "", ""),
		}
	}

	// Status tak dikenal: email netral (jangan kirim detail).
	return EmailContent{
		Subject:  "Pembaruan Status Pendaftaran " + nomor,
		TextBody: "Halo " + nama + ",\n\nStatus pendaftaran " + nomor + " diperbarui.\n" + emailSignature,
		HTMLBody: emailHTML("Pembaruan Status", []string{"Halo " + nama + ",", "Status pendaftaran " + nomor + " diperbarui."}, "Lacak Status", link),
	}
}

func catatanBlock(catatan string) string {
	if strings.TrimSpace(catatan) == "" {
		return ""
	}
	return "\nCatatan: " + catatan + "\n"
}

// AccountSetupEmail menyusun email tautan "buat kata sandi" untuk akun anggota
// baru (Opsi A: tanpa password plaintext di email/DB).
func AccountSetupEmail(nama, link string) EmailContent {
	if strings.TrimSpace(nama) == "" {
		nama = "Anggota"
	}
	lines := []string{
		"Halo " + nama + ",",
		"Akun keanggotaan KIPAN Anda telah aktif.",
		"Buat kata sandi Anda melalui tautan berikut (berlaku 7 hari dan hanya dapat dipakai sekali).",
	}
	return EmailContent{
		Subject:  "Buat Kata Sandi Akun KIPAN",
		TextBody: "Halo " + nama + ",\n\nAkun keanggotaan KIPAN Anda aktif.\nBuat kata sandi melalui tautan berikut (berlaku 7 hari, sekali pakai):\n" + link + emailSignature,
		HTMLBody: emailHTML("Buat Kata Sandi Akun", lines, "Buat Kata Sandi", link),
	}
}

// PengangkatanEmail menyusun email pemberitahuan pengangkatan kader menjadi
// pengurus melalui SK.
func PengangkatanEmail(nama, nia, jabatan, nomorSK, publicURL string) EmailContent {
	if strings.TrimSpace(nama) == "" {
		nama = "Pengurus"
	}
	lines := []string{
		"Halo " + nama + ",",
		"Anda diangkat sebagai " + jabatan + " dalam kepengurusan KIPAN melalui Surat Keputusan " + nomorSK + ".",
		"NIA Anda: " + nia,
		"Status akun Anda berubah menjadi Pengurus. Silakan login kembali dengan akun yang sama.",
	}
	return EmailContent{
		Subject: "Pengangkatan Pengurus — " + nomorSK,
		TextBody: "Halo " + nama + ",\n\nAnda diangkat sebagai " + jabatan +
			" melalui SK " + nomorSK + ".\nNIA: " + nia +
			"\n\nStatus akun Anda berubah menjadi Pengurus. Silakan login kembali.\n" + emailSignature,
		HTMLBody: emailHTML("Pengangkatan sebagai Pengurus", lines, "Masuk ke Akun", publicURL+"/login"),
	}
}
