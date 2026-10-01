package domain

// ErrNotFound dikembalikan saat resource tidak ditemukan di database
var ErrNotFound = &AppError{Code: 404, Message: "Data tidak ditemukan"}

// ErrForbidden dikembalikan saat user tidak punya akses ke resource
var ErrForbidden = &AppError{Code: 403, Message: "Akses ditolak"}

// ErrUnauthorized dikembalikan saat token tidak valid atau tidak ada
var ErrUnauthorized = &AppError{Code: 401, Message: "Autentikasi diperlukan"}

// ErrConflict dikembalikan saat terjadi duplikasi data
var ErrConflict = &AppError{Code: 409, Message: "Data sudah ada"}

// ErrValidation dikembalikan saat validasi input gagal
var ErrValidation = &AppError{Code: 422, Message: "Validasi gagal"}

var ErrUserNotFound = &AppError{Code: 404, Message: "Pengguna tidak ditemukan"}
var ErrInvalidToken = &AppError{Code: 401, Message: "Token tidak valid atau telah kedaluwarsa"}
var ErrInvalidCredentials = &AppError{Code: 401, Message: "Email atau kata sandi tidak sesuai"}
var ErrUserInactive = &AppError{Code: 403, Message: "Akun pengguna sedang dinonaktifkan"}

// AppError adalah custom error type dengan HTTP status code
type AppError struct {
	Code    int
	Message string
	Detail  string
} 

func (e *AppError) Error() string {
	if e.Detail != "" {
		return e.Message + ": " + e.Detail
	}
	return e.Message
}

// NewValidationError membuat AppError validasi dengan detail
func NewValidationError(detail string) *AppError {
	return &AppError{Code: 422, Message: "Validasi gagal", Detail: detail}
}

// NewNotFoundError membuat AppError not found dengan konteks
func NewNotFoundError(entity string) *AppError {
	return &AppError{Code: 404, Message: entity + " tidak ditemukan"}
}

// NewForbiddenError membuat AppError forbidden dengan konteks
func NewForbiddenError(detail string) *AppError {
	return &AppError{Code: 403, Message: "Akses ditolak", Detail: detail}
}
