package domain

import "testing"

func TestIsAllowedTransition(t *testing.T) {
	if !IsAllowedTransition(PendaftaranStatusDraft, PendaftaranStatusDiverifikasi, PendaftaranActionVerifikasi) {
		t.Fatal("expected DRAFT -> DIVERIFIKASI with VERIFIKASI to be allowed")
	}

	if !IsAllowedTransition(PendaftaranStatusDiverifikasi, PendaftaranStatusDisetujui, PendaftaranActionSetujui) {
		t.Fatal("expected DIVERIFIKASI -> DISETUJUI with SETUJUI to be allowed")
	}

	if !IsAllowedTransition(PendaftaranStatusDiverifikasi, PendaftaranStatusPerbaikan, PendaftaranActionPerbaikan) {
		t.Fatal("expected DIVERIFIKASI -> PERBAIKAN with PERBAIKAN to be allowed")
	}

	if IsAllowedTransition(PendaftaranStatusDraft, PendaftaranStatusDisetujui, PendaftaranActionSetujui) {
		t.Fatal("expected invalid direct approval from DRAFT to DISETUJUI to be rejected")
	}

	if IsAllowedTransition(PendaftaranStatusDiverifikasi, PendaftaranStatusDraft, PendaftaranActionVerifikasi) {
		t.Fatal("expected invalid reverse transition to be rejected")
	}
}

func TestTrackingStatusLabel(t *testing.T) {
	cases := map[PendaftaranStatus]string{
		PendaftaranStatusDraft:        "Pendaftaran Diterima",
		PendaftaranStatusDiverifikasi: "Sedang Diverifikasi",
		PendaftaranStatusPerbaikan:    "Perlu Perbaikan",
		PendaftaranStatusDitolak:      "Pendaftaran Ditolak",
		PendaftaranStatusDisetujui:    "Disetujui",
	}
	for status, want := range cases {
		if got := TrackingStatusLabel(status); got != want {
			t.Fatalf("label %q = %q, want %q", status, got, want)
		}
	}
}
