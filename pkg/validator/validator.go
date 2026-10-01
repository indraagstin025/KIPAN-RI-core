package validator

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"unicode"

	"github.com/go-playground/validator/v10"
)

var (
	nikRegex   = regexp.MustCompile(`^\d{16}$`)
	phoneRegex = regexp.MustCompile(`^(\+62|62|0)8[1-9][0-9]{6,10}$`)
)

type CustomValidator struct {
	validator *validator.Validate
}

func New() *CustomValidator {
	v := validator.New()

	// Gunakan json tag sebagai nama field di error message.
	// Ini penting agar frontend bisa langsung memetakan error ke input field.
	v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		if name == "" {
			return fld.Name
		}
		return name
	})

	_ = v.RegisterValidation("nik", func(fl validator.FieldLevel) bool {
		return nikRegex.MatchString(fl.Field().String())
	})

	_ = v.RegisterValidation("id_phone", func(fl validator.FieldLevel) bool {
		return phoneRegex.MatchString(fl.Field().String())
	})

	// password_strength: minimal 8, maksimal 128, harus ada huruf besar,
	// huruf kecil, angka, dan karakter spesial.
	_ = v.RegisterValidation("password_strength", func(fl validator.FieldLevel) bool {
		s := fl.Field().String()
		if len(s) < 8 || len(s) > 128 {
			return false
		}
		var hasUpper, hasLower, hasDigit, hasSpecial bool
		for _, ch := range s {
			switch {
			case unicode.IsUpper(ch):
				hasUpper = true
			case unicode.IsLower(ch):
				hasLower = true
			case unicode.IsDigit(ch):
				hasDigit = true
			case unicode.IsPunct(ch) || unicode.IsSymbol(ch):
				hasSpecial = true
			}
		}
		return hasUpper && hasLower && hasDigit && hasSpecial
	})

	return &CustomValidator{validator: v}
}

// ValidateStruct memvalidasi struct dan mengembalikan map field -> error message.
// Field name mengikuti json tag, bukan nama field Go.
func (cv *CustomValidator) ValidateStruct(s interface{}) map[string]string {
	err := cv.validator.Struct(s)
	if err == nil {
		return nil
	}

	validationErrs, ok := err.(validator.ValidationErrors)
	if !ok {
		return map[string]string{"_": "Validasi gagal"}
	}

	errors := make(map[string]string)
	for _, e := range validationErrs {
		field := e.Field()
		if field == "" {
			field = e.StructField()
		}

		switch e.Tag() {
		case "required":
			errors[field] = fmt.Sprintf("Field %s wajib diisi", field)
		case "email":
			errors[field] = "Format email tidak valid"
		case "nik":
			errors[field] = "NIK harus terdiri dari 16 digit angka"
		case "id_phone":
			errors[field] = "Format nomor HP tidak valid (contoh: 081234567890)"
		case "password_strength":
			errors[field] = "Kata sandi minimal 8 karakter dan harus mengandung huruf besar, huruf kecil, angka, serta simbol"
		case "min":
			errors[field] = fmt.Sprintf("Panjang minimal %s karakter", e.Param())
		case "max":
			errors[field] = fmt.Sprintf("Panjang maksimal %s karakter", e.Param())
		default:
			errors[field] = fmt.Sprintf("Field %s tidak valid", field)
		}
	}
	return errors
}