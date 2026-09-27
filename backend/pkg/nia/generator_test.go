package nia

import (
	"regexp"
	"testing"
)

func TestGenerateNIA(t *testing.T) {
	pattern := regexp.MustCompile(`^KIPAN-[A-Za-z0-9]{1,10}-[A-Za-z0-9]{1,10}-\d{4}-\d{5}$`)

	code, err := GenerateNIA("32", "3273", 2026, 1)
	if err != nil {
		t.Fatalf("GenerateNIA returned error: %v", err)
	}

	if !pattern.MatchString(code) {
		t.Fatalf("expected NIA format KIPAN-[PROV]-[KAB]-[TAHUN]-[NO_URUT], got %q", code)
	}

	if code != "KIPAN-32-3273-2026-00001" {
		t.Fatalf("expected KIPAN-32-3273-2026-00001, got %q", code)
	}
}

func TestGenerateNIAPaddedSequence(t *testing.T) {
	code, err := GenerateNIA("32", "3273", 2026, 142)
	if err != nil {
		t.Fatalf("GenerateNIA returned error: %v", err)
	}
	if code != "KIPAN-32-3273-2026-00142" {
		t.Fatalf("expected KIPAN-32-3273-2026-00142, got %q", code)
	}
}

func TestGenerateNIARejectsInvalidInput(t *testing.T) {
	cases := []struct {
		name      string
		prov, kab string
		tahun     int
		seq       int
	}{
		{"prov kosong", "", "3273", 2026, 1},
		{"kab kosong", "32", "", 2026, 1},
		{"prov strip merusak parsing", "3-2", "3273", 2026, 1},
		{"kab colon merusak kanonis", "32", "32:73", 2026, 1},
		{"tahun nol", "32", "3273", 0, 1},
		{"seq nol", "32", "3273", 2026, 0},
		{"seq negatif", "32", "3273", 2026, -1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := GenerateNIA(tc.prov, tc.kab, tc.tahun, tc.seq); err == nil {
				t.Fatalf("expected error for prov=%q kab=%q tahun=%d seq=%d",
					tc.prov, tc.kab, tc.tahun, tc.seq)
			}
		})
	}
}
