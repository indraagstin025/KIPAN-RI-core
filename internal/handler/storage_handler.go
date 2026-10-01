package handler

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/kipan-indonesia/sim-kipan-core/internal/service"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/response"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/validator"
)

// StorageHandler menangani tiket presigned upload/view (RULES 14).
// Thin-handler: validasi DTO lalu delegasi ke StorageService.
type StorageHandler struct {
	service   *service.StorageService
	validator *validator.CustomValidator
}

func NewStorageHandler(
	service *service.StorageService,
	validator *validator.CustomValidator,
) *StorageHandler {
	return &StorageHandler{service: service, validator: validator}
}

// PresignUploadRequest meminta tiket upload langsung ke S3.
type PresignUploadRequest struct {
	Category string `json:"category" validate:"required"`
	FileName string `json:"file_name" validate:"required,max=255"`
	MIMEType string `json:"mime_type" validate:"required"`
	FileSize int64  `json:"file_size" validate:"required,gt=0"`
}

// PresignUpload menerbitkan tiket upload. Publik (pendaftar belum punya
// akun) tetapi di-rate-limit ketat di route. Nama file client tidak
// dipercaya: object key selalu server-generated.
func (h *StorageHandler) PresignUpload(c *fiber.Ctx) error {
	var req PresignUploadRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Format data tidak valid")
	}
	if errs := h.validator.ValidateStruct(req); len(errs) > 0 {
		return response.ValidationError(c, "Validasi gagal", errs)
	}
	res, err := h.service.RequestUploadPresign(c.Context(),
		strings.TrimSpace(req.Category),
		strings.TrimSpace(req.FileName),
		strings.TrimSpace(req.MIMEType),
		req.FileSize)
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Tiket upload diterbitkan", res)
}

// PresignView menerbitkan tiket baca sementara dokumen privat.
// Wajib auth; akses dicatat di audit trail oleh service.
func (h *StorageHandler) PresignView(c *fiber.Ctx) error {
	key := strings.TrimSpace(c.Query("key"))
	if key == "" {
		return response.BadRequest(c, "Object key wajib diisi")
	}
	actor, ok := actorOf(c)
	if !ok {
		return response.Unauthorized(c, "Tidak terotentikasi")
	}
	res, err := h.service.RequestViewPresign(c.Context(), key, actor, auditContextOf(c))
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Tiket baca diterbitkan", res)
}
