package pendaftaran

import (
	"context"
	"testing"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
	"github.com/kipan-indonesia/sim-kipan-core/internal/service/svcutil"
)

// fakeTrackingOutbox meniru EmailOutboxRepository untuk sub-status kredensial.
type fakeTrackingOutbox struct {
	repository.EmailOutboxRepository
	row *domain.EmailOutbox
	err error
}

func (f *fakeTrackingOutbox) LatestByPendaftaran(context.Context, int, ...string) (*domain.EmailOutbox, error) {
	return f.row, f.err
}

// TestFillKredensialStatus memverifikasi pemetaan status outbox → sub-status
// tracking dan penyamaran email penerima kredensial.
func TestFillKredensialStatus(t *testing.T) {
	cases := []struct {
		name       string
		outboxStat domain.EmailOutboxStatus
		want       string
	}{
		{"sent", domain.EmailOutboxSent, "terkirim"},
		{"failed", domain.EmailOutboxFailed, "gagal"},
		{"pending", domain.EmailOutboxPending, "menunggu"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := &pendaftaranQuerySvc{pendaftaranBase: &pendaftaranBase{
				outboxRepo: &fakeTrackingOutbox{row: &domain.EmailOutbox{
					Status: tc.outboxStat, ToEmail: "indrawan@gmail.com",
				}},
			}}
			resp := &domain.PendaftaranTrackingResponse{}
			svc.fillKredensialStatus(context.Background(), &domain.Pendaftaran{Email: "fallback@kipan.id"}, resp)
			if resp.KredensialStatus != tc.want {
				t.Fatalf("KredensialStatus = %q, want %q", resp.KredensialStatus, tc.want)
			}
			if resp.KredensialEmail != "ind***@gmail.com" {
				t.Fatalf("KredensialEmail = %q, want tersamar", resp.KredensialEmail)
			}
		})
	}

	t.Run("belum ada baris outbox", func(t *testing.T) {
		svc := &pendaftaranQuerySvc{pendaftaranBase: &pendaftaranBase{
			outboxRepo: &fakeTrackingOutbox{err: domain.ErrNotFound},
		}}
		resp := &domain.PendaftaranTrackingResponse{}
		svc.fillKredensialStatus(context.Background(), &domain.Pendaftaran{Email: "pendaftar@def.com"}, resp)
		if resp.KredensialStatus != "menunggu" {
			t.Fatalf("KredensialStatus = %q, want menunggu", resp.KredensialStatus)
		}
		if resp.KredensialEmail != "pen***@def.com" {
			t.Fatalf("KredensialEmail = %q, want fallback email pendaftaran tersamar", resp.KredensialEmail)
		}
	})

	t.Run("outboxRepo nihil (repository tidak terpasang)", func(t *testing.T) {
		svc := &pendaftaranQuerySvc{pendaftaranBase: &pendaftaranBase{}}
		resp := &domain.PendaftaranTrackingResponse{}
		svc.fillKredensialStatus(context.Background(), &domain.Pendaftaran{Email: "abc@def.com"}, resp)
		if resp.KredensialStatus != "menunggu" {
			t.Fatalf("KredensialStatus = %q, want menunggu", resp.KredensialStatus)
		}
	})
}

// TestMaskEmail memverifikasi penyamaran alamat email untuk tampilan publik.
func TestMaskEmail(t *testing.T) {
	cases := map[string]string{
		"indrawan@gmail.com": "ind***@gmail.com",
		"ab@gmail.com":       "***@gmail.com",
		"noatsign":           "noatsign",
		"":                   "",
	}
	for in, want := range cases {
		if got := svcutil.MaskEmail(in); got != want {
			t.Fatalf("svcutil.MaskEmail(%q) = %q, want %q", in, got, want)
		}
	}
}
