package generator

import (
	"regexp"
	"testing"
)

func TestGenerateRegistrationNumber(t *testing.T) {
	pattern := regexp.MustCompile(`^REG-\d{6}-\d{5}$`)

	for i := 0; i < 5; i++ {
		number, err := GenerateRegistrationNumber(2026, 1)
		if err != nil {
			t.Fatalf("GenerateRegistrationNumber returned error: %v", err)
		}
		if !pattern.MatchString(number) {
			t.Fatalf("expected format REG-YYYYMM-XXXXX, got %q", number)
		}
	}
}

func TestGenerateRegistrationNumberUsesExpectedPrefix(t *testing.T) {
	number, err := GenerateRegistrationNumber(2026, 1)
	if err != nil {
		t.Fatalf("GenerateRegistrationNumber returned error: %v", err)
	}

	if number[:4] != "REG-" {
		t.Fatalf("expected REG prefix, got %q", number)
	}
}
