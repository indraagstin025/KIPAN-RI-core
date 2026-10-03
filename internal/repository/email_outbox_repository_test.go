package repository

import (
	"context"
	"testing"
	"time"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
)

// Integration (gated TEST_DATABASE_URL): siklus outbox enqueue → claim → sent.
func TestEmailOutboxLifecycle(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	repo := NewEmailOutboxRepository(db)

	marker := "outboxtest"
	t.Cleanup(func() {
		db.ExecContext(ctx, `DELETE FROM email_outbox WHERE to_email LIKE $1`, marker+"%@example.com")
	})

	item := &domain.EmailOutbox{
		Jenis:    domain.EmailOutboxSetPassword,
		ToEmail:  marker + "1@example.com",
		Subject:  "Set Password",
		TextBody: "klik tautan",
	}
	if err := repo.Enqueue(ctx, item); err != nil {
		t.Fatalf("Enqueue gagal: %v", err)
	}
	if item.ID == 0 || item.Status != domain.EmailOutboxPending {
		t.Fatalf("enqueue tak sesuai: %+v", item)
	}

	// List memuat baris kita.
	items, total, err := repo.List(ctx, OutboxFilter{Jenis: "SET_PASSWORD", Status: "pending", Limit: 100})
	if err != nil {
		t.Fatalf("List gagal: %v", err)
	}
	found := false
	for _, it := range items {
		if it.ID == item.ID {
			found = true
		}
	}
	if !found || total == 0 {
		t.Fatalf("baris outbox tidak ditemukan (total=%d)", total)
	}

	// Claim → attempts naik; lalu tandai terkirim.
	claimed, err := repo.ClaimNext(ctx, 100)
	if err != nil {
		t.Fatalf("ClaimNext gagal: %v", err)
	}
	var claimedOurs bool
	for _, c := range claimed {
		if c.ID == item.ID {
			claimedOurs = true
			if c.Attempts < 1 {
				t.Fatalf("attempts harus naik: %d", c.Attempts)
			}
		}
	}
	if !claimedOurs {
		t.Fatalf("baris kita tidak ter-claim")
	}
	if err := repo.MarkSent(ctx, item.ID); err != nil {
		t.Fatalf("MarkSent gagal: %v", err)
	}

	// Baris gagal (dead) → failed, lalu RetryNow → pending.
	item2 := &domain.EmailOutbox{Jenis: domain.EmailOutboxStatusDisetujui, ToEmail: marker + "2@example.com", Subject: "S", TextBody: "t"}
	if err := repo.Enqueue(ctx, item2); err != nil {
		t.Fatalf("Enqueue2 gagal: %v", err)
	}
	if err := repo.MarkFailed(ctx, item2.ID, "smtp down", time.Now(), true); err != nil {
		t.Fatalf("MarkFailed gagal: %v", err)
	}
	var st string
	if err := db.GetContext(ctx, &st, `SELECT status FROM email_outbox WHERE id=$1`, item2.ID); err != nil || st != "failed" {
		t.Fatalf("status harus failed, dapat %q err=%v", st, err)
	}
	if err := repo.RetryNow(ctx, item2.ID); err != nil {
		t.Fatalf("RetryNow gagal: %v", err)
	}
	if err := db.GetContext(ctx, &st, `SELECT status FROM email_outbox WHERE id=$1`, item2.ID); err != nil || st != "pending" {
		t.Fatalf("status harus pending, dapat %q err=%v", st, err)
	}
}
