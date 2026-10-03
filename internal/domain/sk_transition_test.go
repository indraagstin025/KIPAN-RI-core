package domain

import "testing"

// TestResolveSKApprovalTransitionAJUKANPerLevel mengunci transisi AJUKAN per tingkat.
func TestResolveSKApprovalTransitionAJUKANPerLevel(t *testing.T) {
	cases := []struct {
		level    TingkatWilayah
		wantFrom SKApprovalStatus
		wantTo   SKApprovalStatus
	}{
		{LevelKabupaten, SKApprovalStatusDraft, SKApprovalStatusMenungguProvinsi},
		{LevelProvinsi, SKApprovalStatusDraft, SKApprovalStatusMenungguNasional},
		{LevelNasional, SKApprovalStatusDraft, SKApprovalStatusDisetujui},
	}
	for _, tc := range cases {
		from, to, ok := ResolveSKApprovalTransition(SKActionAjukan, tc.level)
		if !ok || from != tc.wantFrom || to != tc.wantTo {
			t.Fatalf("AJUKAN %s: mau %s->%s ok, dapat %s->%s ok=%v",
				tc.level, tc.wantFrom, tc.wantTo, from, to, ok)
		}
	}
}

// TestResolveSKApprovalTransitionChainUmum memastikan rantai lanjutan tidak
// bergantung tingkat SK.
func TestResolveSKApprovalTransitionChainUmum(t *testing.T) {
	for _, level := range []TingkatWilayah{LevelKabupaten, LevelProvinsi, LevelNasional} {
		if from, to, ok := ResolveSKApprovalTransition(SKActionTeruskan, level); !ok ||
			from != SKApprovalStatusMenungguProvinsi || to != SKApprovalStatusMenungguNasional {
			t.Fatalf("TERUSKAN %s salah: %s->%s ok=%v", level, from, to, ok)
		}
		if from, to, ok := ResolveSKApprovalTransition(SKActionSahkan, level); !ok ||
			from != SKApprovalStatusMenungguNasional || to != SKApprovalStatusDisetujui {
			t.Fatalf("SAHKAN %s salah: %s->%s ok=%v", level, from, to, ok)
		}
		if from, to, ok := ResolveSKApprovalTransition(SKActionTolak, level); !ok ||
			from != SKApprovalStatusMenungguNasional || to != SKApprovalStatusDitolak {
			t.Fatalf("TOLAK %s salah: %s->%s ok=%v", level, from, to, ok)
		}
	}
}

// TestResolveSKApprovalTransitionInvalid memastikan kombinasi tak dikenal ditolak.
func TestResolveSKApprovalTransitionInvalid(t *testing.T) {
	if _, _, ok := ResolveSKApprovalTransition(SKActionAjukan, TingkatWilayah("XYZ")); ok {
		t.Fatal("AJUKAN dengan level tak dikenal harus false")
	}
	if _, _, ok := ResolveSKApprovalTransition(SKApprovalAction("NGAWUR"), LevelNasional); ok {
		t.Fatal("aksi tak dikenal harus false")
	}
}

// TestSKApprovalTransitionRulesNoDuplicate memastikan tabel konsisten: tidak ada
// (aksi, level) ganda dan transisi tidak trivial.
func TestSKApprovalTransitionRulesNoDuplicate(t *testing.T) {
	seen := map[string]bool{}
	for _, r := range SKApprovalTransitionRules {
		key := string(r.Action) + "|" + string(r.Level)
		if seen[key] {
			t.Fatalf("aturan ganda untuk %s", key)
		}
		seen[key] = true
		if r.From == "" || r.To == "" || r.From == r.To {
			t.Fatalf("transisi tidak valid: %+v", r)
		}
	}
}
