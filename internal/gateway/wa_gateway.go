// Package gateway mengabstraksi pengiriman WhatsApp.
//
// Dua implementasi:
//   - LogGateway  : dev/test — mencetak pesan ke log server (JANGAN produksi).
//   - FonnteGateway: produksi — HTTP API Fonnte (api.fonnte.com).
//
// Service tidak boleh bergantung pada provider konkret; hanya interface ini.
package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// WAGateway mengirim pesan teks ke nomor WhatsApp tujuan (format bebas;
// implementasi menormalisasi ke format provider).
type WAGateway interface {
	SendMessage(ctx context.Context, to, message string) error
}

// NormalizeWATarget menyeragamkan nomor Indonesia ke format 62xxxx tanpa
// simbol: buang non-digit, ubah awalan 0 → 62, biarkan 62 tetap.
func NormalizeWATarget(to string) string {
	var b strings.Builder
	for _, r := range to {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	d := b.String()
	switch {
	case strings.HasPrefix(d, "62"):
		return d
	case strings.HasPrefix(d, "0"):
		return "62" + strings.TrimPrefix(d, "0")
	default:
		return d
	}
}

// ============================================================
// LogGateway (dev/test)
// ============================================================

// LogGateway mencatat pesan ke log server. DEV ONLY — jangan dipakai di
// production (pesan bisa memuat OTP; log agregator = bocor). Produksi
// dijamin memakai provider nyata oleh validasi config.
type LogGateway struct{}

func NewLogGateway() *LogGateway { return &LogGateway{} }

func (g *LogGateway) SendMessage(_ context.Context, to, message string) error {
	// Nomor dimask; pesan utuh agar operator dev bisa membaca.
	log.Warn().Str("to", MaskWA(to)).Msg("DEV-WA (log-only): " + message)
	return nil
}

// MaskWA menyembunyikan digit tengah nomor untuk log.
func MaskWA(to string) string {
	digits := make([]byte, 0, len(to))
	for i := 0; i < len(to); i++ {
		if to[i] >= '0' && to[i] <= '9' {
			digits = append(digits, to[i])
		}
	}
	if len(digits) <= 4 {
		return "****"
	}
	return string(digits[:2]) + "****" + string(digits[len(digits)-2:])
}

// ============================================================
// FonnteGateway (produksi)
// ============================================================

// DefaultFonnteURL adalah endpoint API Fonnte.
const DefaultFonnteURL = "https://api.fonnte.com"

// FonnteGateway mengirim pesan via API Fonnte.
// Endpoint: POST {base}/send, header Authorization: <token>,
// body form-urlencoded: target, message.
type FonnteGateway struct {
	token   string
	baseURL string
	http    *http.Client
}

func NewFonnteGateway(token string) *FonnteGateway {
	return &FonnteGateway{
		token:   strings.TrimSpace(token),
		baseURL: DefaultFonnteURL,
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

// NewFonnteGatewayWithURL dipakai unit test (base URL bisa diarahkan ke httptest).
func NewFonnteGatewayWithURL(token, baseURL string) *FonnteGateway {
	g := NewFonnteGateway(token)
	if trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/"); trimmed != "" {
		g.baseURL = trimmed
	}
	return g
}

func (g *FonnteGateway) SendMessage(ctx context.Context, to, message string) error {
	if g.token == "" {
		return fmt.Errorf("token Fonnte belum dikonfigurasi")
	}
	target := NormalizeWATarget(to)
	if target == "" {
		return fmt.Errorf("nomor WhatsApp tujuan kosong")
	}

	form := url.Values{}
	form.Set("target", target)
	form.Set("message", message)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		g.baseURL+"/send", strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("gagal menyiapkan permintaan Fonnte: %w", err)
	}
	req.Header.Set("Authorization", g.token)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := g.http.Do(req)
	if err != nil {
		return fmt.Errorf("gagal menghubungi Fonnte: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("Fonnte menolak pengiriman (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	// Fonnte membalas 200 walau gagal; status boolean di body wajib dicek.
	var parsed struct {
		Status bool   `json:"status"`
		Reason string `json:"reason"`
		Detail string `json:"detail"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return fmt.Errorf("respons Fonnte tidak dikenali: %w", err)
	}
	if !parsed.Status {
		reason := parsed.Reason
		if reason == "" {
			reason = parsed.Detail
		}
		return fmt.Errorf("Fonnte gagal mengirim: %s", reason)
	}
	return nil
}
