package response

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
)

// ============================================================
// Response Schemas
// ============================================================

// StandardResponse — format sukses standar untuk semua endpoint.
//
// Contoh:
//
//	{
//	  "success": true,
//	  "code": "OK",
//	  "message": "Data berhasil dimuat",
//	  "data": { ... },
//	  "meta": { "page": 1, "per_page": 20, "total": 150 }
//	}
type StandardResponse struct {
	Success bool        `json:"success"`
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Meta    interface{} `json:"meta,omitempty"`
}

// FieldError — error per field (untuk response validasi).
//
// Fix M-1: sebelumnya pakai map[string]string yang tidak terurut dan
// sulit di-parse frontend. Sekarang pakai array of struct.
type FieldError struct {
	Field string `json:"field"`
	Issue string `json:"issue"`
}

// ErrorResponse — format error standar (terinspirasi RFC 7807).
type ErrorResponse struct {
	Success bool         `json:"success"`
	Code    string       `json:"code"`
	Message string       `json:"message"`
	Errors  []FieldError `json:"errors,omitempty"`
	TraceID string       `json:"trace_id,omitempty"`
}

// ============================================================
// Trace ID helper
// ============================================================

func getTraceID(c *fiber.Ctx) string {
	if tid, ok := c.Locals("requestid").(string); ok && tid != "" {
		return tid
	}
	return c.GetRespHeader("X-Request-ID")
}

// ============================================================
// Success responses
// ============================================================

// OK mengirim response sukses 200.
func OK(c *fiber.Ctx, message string, data interface{}) error {
	return c.Status(fiber.StatusOK).JSON(StandardResponse{
		Success: true,
		Code:    "OK",
		Message: message,
		Data:    data,
	})
}

// Success — alias untuk OK (nama lebih natural di handler).
func Success(c *fiber.Ctx, message string, data interface{}) error {
	return OK(c, message, data)
}

// Created mengirim response 201.
func Created(c *fiber.Ctx, message string, data interface{}) error {
	return c.Status(fiber.StatusCreated).JSON(StandardResponse{
		Success: true,
		Code:    "CREATED",
		Message: message,
		Data:    data,
	})
}

// Paginated mengirim response 200 dengan metadata paginasi.
func Paginated(c *fiber.Ctx, message string, data interface{}, meta interface{}) error {
	return c.Status(fiber.StatusOK).JSON(StandardResponse{
		Success: true,
		Code:    "OK",
		Message: message,
		Data:    data,
		Meta:    meta,
	})
}

// ============================================================
// Error responses
// ============================================================

func BadRequest(c *fiber.Ctx, message string) error {
	return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
		Success: false,
		Code:    "BAD_REQUEST",
		Message: message,
		TraceID: getTraceID(c),
	})
}

func Unauthorized(c *fiber.Ctx, message string) error {
	return c.Status(fiber.StatusUnauthorized).JSON(ErrorResponse{
		Success: false,
		Code:    "UNAUTHORIZED",
		Message: message,
		TraceID: getTraceID(c),
	})
}

func Forbidden(c *fiber.Ctx, message string) error {
	return c.Status(fiber.StatusForbidden).JSON(ErrorResponse{
		Success: false,
		Code:    "FORBIDDEN",
		Message: message,
		TraceID: getTraceID(c),
	})
}

func NotFound(c *fiber.Ctx, message string) error {
	return c.Status(fiber.StatusNotFound).JSON(ErrorResponse{
		Success: false,
		Code:    "NOT_FOUND",
		Message: message,
		TraceID: getTraceID(c),
	})
}

func Conflict(c *fiber.Ctx, message string) error {
	return c.Status(fiber.StatusConflict).JSON(ErrorResponse{
		Success: false,
		Code:    "CONFLICT",
		Message: message,
		TraceID: getTraceID(c),
	})
}

// UnprocessableEntity mengirim 422 dengan detail validasi per-field.
func UnprocessableEntity(c *fiber.Ctx, message string, errs []FieldError) error {
	return c.Status(fiber.StatusUnprocessableEntity).JSON(ErrorResponse{
		Success: false,
		Code:    "VALIDATION_ERROR",
		Message: message,
		Errors:  errs,
		TraceID: getTraceID(c),
	})
}

// ValidationError — convenience wrapper untuk validasi yang menerima
// map (dari pkg/validator) lalu dikonversi ke []FieldError.
func ValidationError(c *fiber.Ctx, message string, errMap map[string]string) error {
	// Konversi map → slice (urutan tidak deterministik, tapi untuk
	// response validasi ini OK karena frontend match by field name).
	fieldErrs := make([]FieldError, 0, len(errMap))
	for field, issue := range errMap {
		fieldErrs = append(fieldErrs, FieldError{Field: field, Issue: issue})
	}
	return UnprocessableEntity(c, message, fieldErrs)
}

func InternalServerError(c *fiber.Ctx, message string) error {
	return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
		Success: false,
		Code:    "INTERNAL_ERROR",
		Message: message,
		TraceID: getTraceID(c),
	})
}

func TooManyRequests(c *fiber.Ctx, message string) error {
	return c.Status(fiber.StatusTooManyRequests).JSON(ErrorResponse{
		Success: false,
		Code:    "TOO_MANY_REQUESTS",
		Message: message,
		TraceID: getTraceID(c),
	})
}

// ============================================================
// Error mapper
// ============================================================

// FromError memetakan domain.AppError → HTTP response dengan format standar.
//
// Fix C-4: jika AppError.Detail di-set, ia menimpa Message utama — sehingga
// pesan validasi spesifik (mis. "Kata sandi saat ini tidak sesuai") sampai
// ke klien, bukan hanya pesan generik "Validasi gagal".
func FromError(c *fiber.Ctx, err error) error {
	if err == nil {
		return OK(c, "Sukses", nil)
	}

	var appErr *domain.AppError
	if errors.As(err, &appErr) {
		// Prioritaskan Detail jika ada — ia lebih spesifik dari Message.
		message := appErr.Message
		if appErr.Detail != "" {
			message = appErr.Detail
		}

		return c.Status(appErr.Code).JSON(ErrorResponse{
			Success: false,
			Code:    errorCodeFromStatus(appErr.Code),
			Message: message,
			TraceID: getTraceID(c),
		})
	}

	// Error tidak dikenal — jangan bocorkan detail internal ke klien.
	// Log-nya sudah ditangani oleh middleware logger.
	return InternalServerError(c, "Terjadi kesalahan internal pada server")
}

// errorCodeFromStatus memetakan HTTP status code ke error code string.
// Konsisten dengan errorCodeForStatus di main.go.
func errorCodeFromStatus(status int) string {
	switch status {
	case fiber.StatusBadRequest:
		return "BAD_REQUEST"
	case fiber.StatusUnauthorized:
		return "UNAUTHORIZED"
	case fiber.StatusForbidden:
		return "FORBIDDEN"
	case fiber.StatusNotFound:
		return "NOT_FOUND"
	case fiber.StatusConflict:
		return "CONFLICT"
	case fiber.StatusUnprocessableEntity:
		return "VALIDATION_ERROR"
	case fiber.StatusTooManyRequests:
		return "TOO_MANY_REQUESTS"
	case fiber.StatusInternalServerError:
		return "INTERNAL_ERROR"
	default:
		return "ERROR"
	}
}