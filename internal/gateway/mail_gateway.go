// Package gateway — pengiriman email (Batch 3).
//
// Implementasi:
//   - SMTPSender   : SMTP (Mailpit dev :1025 tanpa auth; Mailtrap sandbox
//     sandbox.smtp.mailtrap.io:2525 dengan auth; produksi
//     live.smtp.mailtrap.io:587 / provider SMTP lain).
//   - LogMailSender: dev/test — mencetak email ke log server (JANGAN produksi).
//
// Service hanya bergantung pada interface MailSender.
package gateway

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// MailSender mengirim satu email (versi teks wajib; HTML opsional).
type MailSender interface {
	Send(ctx context.Context, to, subject, textBody, htmlBody string) error
}

// MailConfig adalah parameter SMTP.
type MailConfig struct {
	Host      string
	Port      int
	Username  string
	Password  string
	FromEmail string
	FromName  string
}

// ============================================================
// LogMailSender (dev/test)
// ============================================================

type LogMailSender struct{}

func NewLogMailSender() *LogMailSender { return &LogMailSender{} }

func (s *LogMailSender) Send(_ context.Context, to, subject, textBody, _ string) error {
	log.Warn().Str("to", MaskEmail(to)).Str("subject", subject).
		Msg("DEV-EMAIL (log-only): " + textBody)
	return nil
}

// MaskEmail menyembunyikan bagian lokal alamat untuk log.
func MaskEmail(to string) string {
	at := strings.LastIndex(to, "@")
	if at <= 0 {
		return "***"
	}
	local := to[:at]
	if len(local) <= 2 {
		return "*@" + to[at+1:]
	}
	return local[:2] + "***@" + to[at+1:]
}

// ============================================================
// SMTPSender (produksi/dev dengan SMTP nyata)
// ============================================================

type SMTPSender struct {
	cfg  MailConfig
	auth smtp.Auth
}

func NewSMTPSender(cfg MailConfig) *SMTPSender {
	s := &SMTPSender{cfg: cfg}
	if strings.TrimSpace(cfg.Username) != "" {
		// PlainAuth otomatis menolak kanal non-TLS/non-localhost — aman.
		s.auth = smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
	}
	return s
}

func (s *SMTPSender) Send(ctx context.Context, to, subject, textBody, htmlBody string) error {
	from := strings.TrimSpace(s.cfg.FromEmail)
	if from == "" {
		return errors.New("MAIL_FROM_EMAIL belum dikonfigurasi")
	}
	if strings.TrimSpace(to) == "" {
		return errors.New("alamat tujuan kosong")
	}
	// Anti header-injection: alamat tujuan masuk mentah ke header To.
	// Seluruh pemanggil saat ini memakai email tervalidasi, tapi tolak
	// CRLF di sini agar kelas celah tertutup apa pun sumbernya.
	if strings.ContainsAny(to, "\r\n") {
		return errors.New("alamat tujuan tidak valid")
	}

	msg := BuildMIMEMessage(s.cfg.FromName, from, to, subject, textBody, htmlBody)

	addr := net.JoinHostPort(s.cfg.Host, strconv.Itoa(s.cfg.Port))

	// Implicit TLS untuk port 465; STARTTLS untuk port lain (587/2525/1025).
	if s.cfg.Port == 465 {
		return s.sendImplicitTLS(ctx, addr, from, to, msg)
	}
	return s.sendWithStartTLS(ctx, addr, from, to, msg)
}

func (s *SMTPSender) sendImplicitTLS(ctx context.Context, addr, from, to string, msg []byte) error {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{ServerName: s.cfg.Host})
	if err != nil {
		return fmt.Errorf("gagal koneksi TLS ke SMTP: %w", err)
	}
	client, err := smtp.NewClient(conn, s.cfg.Host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("gagal membuat klien SMTP: %w", err)
	}
	return s.deliver(ctx, client, conn, from, to, msg)
}

func (s *SMTPSender) sendWithStartTLS(ctx context.Context, addr, from, to string, msg []byte) error {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("gagal koneksi ke SMTP: %w", err)
	}
	client, err := smtp.NewClient(conn, s.cfg.Host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("gagal membuat klien SMTP: %w", err)
	}
	// Mulai STARTTLS bila server mendukung dan kita memakai kredensial.
	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(&tls.Config{ServerName: s.cfg.Host}); err != nil {
			_ = client.Close()
			return fmt.Errorf("gagal STARTTLS: %w", err)
		}
	}
	return s.deliver(ctx, client, conn, from, to, msg)
}

func (s *SMTPSender) deliver(ctx context.Context, client *smtp.Client, conn net.Conn, from, to string, msg []byte) error {
	defer func() { _ = client.Close() }()

	// Batasi seluruh operasi SMTP (dial sudah punya timeout terpisah).
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	} else {
		_ = conn.SetDeadline(time.Now().Add(15 * time.Second))
	}

	if s.auth != nil {
		if err := client.Auth(s.auth); err != nil {
			return fmt.Errorf("autentikasi SMTP gagal: %w", err)
		}
	}
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("SMTP MAIL FROM gagal: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("SMTP RCPT TO gagal: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("SMTP DATA gagal: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		_ = w.Close()
		return fmt.Errorf("gagal menulis isi email: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("gagal menutup isi email: %w", err)
	}
	return client.Quit()
}

// BuildMIMEMessage menyusun pesan RFC 5322 multipart/alternative (teks+HTML).
// Fungsi murni agar unit-testable tanpa server SMTP.
func BuildMIMEMessage(fromName, fromEmail, to, subject, textBody, htmlBody string) []byte {
	var b strings.Builder

	fromHeader := fromEmail
	if strings.TrimSpace(fromName) != "" {
		fromHeader = fmt.Sprintf("%s <%s>", mime.QEncoding.Encode("utf-8", fromName), fromEmail)
	}
	b.WriteString("From: " + fromHeader + "\r\n")
	b.WriteString("To: " + to + "\r\n")
	b.WriteString("Subject: " + mime.BEncoding.Encode("utf-8", subject) + "\r\n")
	b.WriteString("Date: " + time.Now().Format(time.RFC1123Z) + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")

	if strings.TrimSpace(htmlBody) == "" {
		b.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n")
		b.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
		b.WriteString(toCRLF(textBody))
		return []byte(b.String())
	}

	boundary := "kipan-boundary-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	b.WriteString("Content-Type: multipart/alternative; boundary=\"" + boundary + "\"\r\n\r\n")

	b.WriteString("--" + boundary + "\r\n")
	b.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n")
	b.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
	b.WriteString(toCRLF(textBody))
	b.WriteString("\r\n")

	b.WriteString("--" + boundary + "\r\n")
	b.WriteString("Content-Type: text/html; charset=\"utf-8\"\r\n")
	b.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
	b.WriteString(toCRLF(htmlBody))
	b.WriteString("\r\n")

	b.WriteString("--" + boundary + "--\r\n")
	return []byte(b.String())
}

// toCRLF menormalkan line ending body email ke CRLF.
func toCRLF(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\n", "\r\n")
}
