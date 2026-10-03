// Package infra menyediakan pabrik adaptor infrastruktur (gateway eksternal)
// yang dipilih berdasarkan konfigurasi. Dipakai bersama oleh composition root
// HTTP (internal/router) maupun biner worker (cmd/worker) agar pemilihan
// implementasi (dev vs produksi) hanya hidup di satu tempat.
package infra

import (
	"github.com/rs/zerolog/log"

	"github.com/kipan-indonesia/sim-kipan-core/config"
	"github.com/kipan-indonesia/sim-kipan-core/internal/gateway"
)

// WAGateway memilih implementasi WAGateway dari config: Fonnte untuk produksi,
// LogGateway untuk dev/test. Validasi config menjamin produksi tidak memakai
// LogGateway (fail-fast), jadi keputusan di sini cukup berbasis provider.
func WAGateway(cfg *config.Config) gateway.WAGateway {
	if cfg.WA.Provider == "fonnte" {
		log.Info().Msg("WA gateway aktif: Fonnte")
		return gateway.NewFonnteGateway(cfg.WA.FonnteToken)
	}
	log.Warn().Msg("WA gateway: log-only (HANYA dev/test - jangan dipakai di produksi)")
	return gateway.NewLogGateway()
}

// MailSender memilih implementasi MailSender: SMTP bila MAIL_HOST diisi
// (Mailpit dev / Mailtrap sandbox / produksi), selain itu LogMailSender
// (dev/test; produksi dijamin oleh validasi config).
func MailSender(cfg *config.Config) gateway.MailSender {
	if cfg.Mail.Enabled() {
		log.Info().
			Str("host", cfg.Mail.Host).
			Int("port", cfg.Mail.Port).
			Msg("Email gateway aktif: SMTP")
		return gateway.NewSMTPSender(gateway.MailConfig{
			Host:      cfg.Mail.Host,
			Port:      cfg.Mail.Port,
			Username:  cfg.Mail.Username,
			Password:  cfg.Mail.Password,
			FromEmail: cfg.Mail.FromEmail,
			FromName:  cfg.Mail.FromName,
		})
	}
	log.Warn().Msg("Email gateway: log-only (set MAIL_HOST untuk SMTP; HANYA dev/test)")
	return gateway.NewLogMailSender()
}
