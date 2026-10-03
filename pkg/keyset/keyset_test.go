package keyset

import (
	"testing"
	"time"
)

func TestRoundTrip(t *testing.T) {
	at := time.Date(2026, 10, 3, 15, 4, 5, 123456789, time.UTC)
	id := 4242
	cur := Encode(at, id)
	gotAt, gotID, err := Decode(cur)
	if err != nil {
		t.Fatalf("Decode gagal: %v", err)
	}
	if !gotAt.Equal(at) || gotID != id {
		t.Fatalf("round-trip salah: %v %d", gotAt, gotID)
	}
}

func TestDecodeInvalid(t *testing.T) {
	for _, s := range []string{"", "!!!", "bm90LWEtY3Vyc29y", "MTIzNA"} {
		if _, _, err := Decode(s); err == nil {
			t.Fatalf("cursor %q seharusnya tidak valid", s)
		}
	}
}
