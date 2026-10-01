package domain

import "testing"

func TestIsAllowedTransition(t *testing.T) {
	if !IsAllowedTransition(PendaftaranStatusDiajukan, PendaftaranStatusDiverifikasi, PendaftaranActionVerifikasi) {
		t.Fatal("expected DIAJUKAN -> DIVERIFIKASI with VERIFIKASI to be allowed")
	}

	if !IsAllowedTransition(PendaftaranStatusDiverifikasi, PendaftaranStatusDisetujui, PendaftaranActionSetujui) {
		t.Fatal("expected DIVERIFIKASI -> DISETUJUI with SETUJUI to be allowed")
	}

	if !IsAllowedTransition(PendaftaranStatusDiverifikasi, PendaftaranStatusPerbaikan, PendaftaranActionPerbaikan) {
		t.Fatal("expected DIVERIFIKASI -> PERBAIKAN with PERBAIKAN to be allowed")
	}

	if IsAllowedTransition(PendaftaranStatusDiajukan, PendaftaranStatusDisetujui, PendaftaranActionSetujui) {
		t.Fatal("expected invalid direct approval from DIAJUKAN to DISETUJUI to be rejected")
	}

	if IsAllowedTransition(PendaftaranStatusDiverifikasi, PendaftaranStatusDiajukan, PendaftaranActionVerifikasi) {
		t.Fatal("expected invalid reverse transition to be rejected")
	}
}
