package validator

import (
	"testing"
)

type sampleDTO struct {
	Email    string `json:"email" validate:"required,email"`
	NIK      string `json:"nik" validate:"required,nik"`
	Phone    string `json:"phone" validate:"required,id_phone"`
	Password string `json:"password" validate:"required,password_strength"`
}

func validSample() sampleDTO {
	return sampleDTO{
		Email:    "a@b.co",
		NIK:      "3201010101010001",
		Phone:    "081234567890",
		Password: "Kuat2026!x",
	}
}

func TestValidatorAcceptsValid(t *testing.T) {
	if errs := New().ValidateStruct(validSample()); len(errs) != 0 {
		t.Fatalf("valid DTO ditolak: %v", errs)
	}
}

func TestValidatorRejectsBadFields(t *testing.T) {
	v := New()
	cases := []struct {
		name   string
		mutate func(*sampleDTO)
		field  string
	}{
		{"email tanpa @", func(s *sampleDTO) { s.Email = "asal" }, "email"},
		{"nik pendek", func(s *sampleDTO) { s.NIK = "12345" }, "nik"},
		{"nik huruf", func(s *sampleDTO) { s.NIK = "320101010101000a" }, "nik"},
		{"wa abjad", func(s *sampleDTO) { s.Phone = "abc123" }, "phone"},
		{"wa 080", func(s *sampleDTO) { s.Phone = "080000000000" }, "phone"},
		{"password lemah", func(s *sampleDTO) { s.Password = "123" }, "password"},
		{"password tanpa simbol", func(s *sampleDTO) { s.Password = "Kuat2026xx" }, "password"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := validSample()
			tc.mutate(&s)
			errs := v.ValidateStruct(s)
			if _, ok := errs[tc.field]; !ok {
				t.Fatalf("field %q harus error, dapat %v", tc.field, errs)
			}
		})
	}
}

func TestValidatorUsesJSONTagNames(t *testing.T) {
	s := validSample()
	s.Email = "asal"
	errs := New().ValidateStruct(s)
	if _, ok := errs["email"]; !ok {
		t.Fatalf("nama field harus json tag (email), dapat %v", errs)
	}
}
