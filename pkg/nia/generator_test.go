package nia

import (
	"regexp"
	"testing"
)

func TestGenerateNIA(t *testing.T) {
	pattern := regexp.MustCompile(`^KIPAN-IND-[A-Za-z0-9]{1,10}-\d{4}-\d{6}$`)

	code, err := GenerateNIA("3204", 2026, 1)
	if err != nil {
		t.Fatalf("GenerateNIA returned error: %v", err)
	}

	if !pattern.MatchString(code) {
		t.Fatalf("expected NIA format KIPAN-IND-[KAB]-[TAHUN]-[NO_URUT], got %q", code)
	}

	if code != "KIPAN-IND-3204-2026-000001" {
		t.Fatalf("expected KIPAN-IND-3204-2026-000001, got %q", code)
	}
}

func TestGenerateNIAPaddedSequence(t *testing.T) {
	code, err := GenerateNIA("3204", 2026, 142)
	if err != nil {
		t.Fatalf("GenerateNIA returned error: %v", err)
	}
	if code != "KIPAN-IND-3204-2026-000142" {
		t.Fatalf("expected KIPAN-IND-3204-2026-000142, got %q", code)
	}
}

func TestGenerateNIARejectsInvalidInput(t *testing.T) {
	cases := []struct {
		name  string
		kab   string
		tahun int
		seq   int
	}{
		{"kab kosong", "", 2026, 1},
		{"kab colon merusak kanonis", "32:73", 2026, 1},
		{"kab terlalu panjang", "12345678901", 2026, 1},
		{"tahun nol", "3204", 0, 1},
		{"seq nol", "3204", 2026, 0},
		{"seq negatif", "3204", 2026, -1},
		{"seq melebihi 6 digit", "3204", 2026, 1000000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := GenerateNIA(tc.kab, tc.tahun, tc.seq); err == nil {
				t.Fatalf("expected error for kab=%q tahun=%d seq=%d",
					tc.kab, tc.tahun, tc.seq)
			}
		})
	}
}
