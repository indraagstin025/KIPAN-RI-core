package nia

import (
	"regexp"
	"testing"
)

func TestGenerateNIA(t *testing.T) {
	pattern := regexp.MustCompile(`^KIPAN\.\d{2}\.\d{4}\.\d{4}\.\d{4}$`)

	code, err := GenerateNIA(32, 3273, 2026, 1)
	if err != nil {
		t.Fatalf("GenerateNIA returned error: %v", err)
	}

	if !pattern.MatchString(code) {
		t.Fatalf("expected NIA format KIPAN.XX.XXXX.XXXX.XXXX, got %q", code)
	}

	if code != "KIPAN.32.3273.2026.0001" {
		t.Fatalf("expected KIPAN.32.3273.2026.0001, got %q", code)
	}
}
