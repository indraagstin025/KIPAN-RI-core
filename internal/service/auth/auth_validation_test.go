package auth

import (
	"strings"
	"testing"

	"github.com/kipan-indonesia/sim-kipan-core/pkg/validator"
)

// Password login & lama dibatasi maks 128 (selaras password_strength):
// membatasi input argon2id (anti-DoS) tanpa menolak password sah.
func TestLoginPasswordMaxLength(t *testing.T) {
	v := validator.New()
	long := strings.Repeat("A", 129) + "a1!"

	if errs := v.ValidateStruct(LoginRequest{Email: "a@b.co", Password: long}); len(errs) == 0 {
		t.Fatal("password login >128 char harus ditolak validasi")
	}
	if errs := v.ValidateStruct(LoginRequest{Email: "a@b.co", Password: strings.Repeat("A", 124) + "a1!"}); len(errs) != 0 {
		t.Fatalf("password 128 char harus lolos validasi: %v", errs)
	}
}

func TestChangePasswordOldMaxLength(t *testing.T) {
	v := validator.New()
	long := strings.Repeat("B", 200)
	req := ChangePasswordRequest{OldPassword: long, NewPassword: "Baru1234!"}
	if errs := v.ValidateStruct(req); len(errs) == 0 {
		t.Fatal("old_password >128 char harus ditolak validasi")
	}
}
