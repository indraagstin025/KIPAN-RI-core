package generator

import (
	"regexp"
	"testing"
	"time"
)

func TestGenerateRegistrationNumber(t *testing.T) {
	pattern := regexp.MustCompile(`^REG-\d{6}-\d{5}$`)

	number, err := GenerateRegistrationNumber(2026, 9, 1)
	if err != nil {
		t.Fatalf("GenerateRegistrationNumber returned error: %v", err)
	}
	if !pattern.MatchString(number) {
		t.Fatalf("expected format REG-YYYYMM-XXXXX, got %q", number)
	}
	if number != "REG-202609-00001" {
		t.Fatalf("expected REG-202609-00001, got %q", number)
	}
}

func TestGenerateRegistrationNumberUsesCurrentMonth(t *testing.T) {
	now := time.Now()
	number, err := GenerateRegistrationNumber(now.Year(), int(now.Month()), 42)
	if err != nil {
		t.Fatalf("GenerateRegistrationNumber returned error: %v", err)
	}
	want := "REG-" + now.Format("200601") + "-00042"
	if number != want {
		t.Fatalf("expected %q, got %q", want, number)
	}
}

func TestGenerateRegistrationNumberRejectsInvalidInput(t *testing.T) {
	cases := []struct {
		name       string
		year       int
		month      int
		seq        int
	}{
		{"tahun nol", 0, 9, 1},
		{"tahun negatif", -2026, 9, 1},
		{"bulan nol", 2026, 0, 1},
		{"bulan 13", 2026, 13, 1},
		{"seq nol", 2026, 9, 0},
		{"seq negatif", 2026, 9, -1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := GenerateRegistrationNumber(tc.year, tc.month, tc.seq); err == nil {
				t.Fatalf("expected error for year=%d month=%d seq=%d", tc.year, tc.month, tc.seq)
			}
		})
	}
}
