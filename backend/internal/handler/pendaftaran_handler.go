package handler

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/repository"
	"github.com/kipan-indonesia/sim-kipan-core/internal/service"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/response"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/validator"
)

// PendaftaranHandler menangani HTTP transport untuk pendaftaran membership.
type PendaftaranHandler struct {
	service   service.PendaftaranService
	repo      repository.PendaftaranRepository
	validator *validator.CustomValidator
}

func NewPendaftaranHandler(
	service service.PendaftaranService,
	repo repository.PendaftaranRepository,
	validator *validator.CustomValidator,
) *PendaftaranHandler {
	return &PendaftaranHandler{service: service, repo: repo, validator: validator}
}

func (h *PendaftaranHandler) Submit(c *fiber.Ctx) error {
	var req domain.PendaftaranSubmitRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Format data tidak valid")
	}
	if errs := h.validator.ValidateStruct(req); len(errs) > 0 {
		return response.ValidationError(c, "Validasi gagal", errs)
	}
	if err := h.service.ValidateSubmitRequest(req); err != nil {
		return response.FromError(c, err)
	}
	result, err := h.service.CreateRegistration(c.Context(), req)
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Created(c, "Pendaftaran berhasil dikirim", result)
}

func (h *PendaftaranHandler) TrackStatus(c *fiber.Ctx) error {
	nomor := strings.TrimSpace(c.Params("nomor"))
	if nomor == "" {
		return response.BadRequest(c, "Nomor pendaftaran wajib diisi")
	}
	if h.repo == nil {
		return response.InternalServerError(c, "Repository pendaftaran belum terhubung")
	}
	item, err := h.repo.GetByNomorPendaftaran(c.Context(), nomor)
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Status pendaftaran ditemukan", item)
}

func (h *PendaftaranHandler) Detail(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return response.BadRequest(c, "ID pendaftaran tidak valid")
	}
	if h.repo == nil {
		return response.InternalServerError(c, "Repository pendaftaran belum terhubung")
	}
	item, err := h.repo.GetByID(c.Context(), id)
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Detail pendaftaran", item)
}

func (h *PendaftaranHandler) ListQueue(c *fiber.Ctx) error {
	return response.Success(c, "Daftar antrean pendaftaran", []string{})
}

func (h *PendaftaranHandler) Verify(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return response.BadRequest(c, "ID pendaftaran tidak valid")
	}
	var payload struct {
		Catatan string `json:"catatan"`
	}
	_ = c.BodyParser(&payload)
	if err := h.service.ProcessApproval(c.Context(), id, domain.PendaftaranActionVerifikasi, payload.Catatan); err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Verifikasi berhasil diproses", nil)
}

func (h *PendaftaranHandler) RequestRevision(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return response.BadRequest(c, "ID pendaftaran tidak valid")
	}
	var payload struct {
		Catatan string `json:"catatan"`
	}
	_ = c.BodyParser(&payload)
	if err := h.service.ProcessApproval(c.Context(), id, domain.PendaftaranActionPerbaikan, payload.Catatan); err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Permintaan revisi dikirim", nil)
}

func (h *PendaftaranHandler) Reject(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return response.BadRequest(c, "ID pendaftaran tidak valid")
	}
	var payload struct {
		Catatan string `json:"catatan"`
	}
	_ = c.BodyParser(&payload)
	if err := h.service.ProcessApproval(c.Context(), id, domain.PendaftaranActionTolak, payload.Catatan); err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Pendaftaran ditolak", nil)
}

func (h *PendaftaranHandler) Approve(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return response.BadRequest(c, "ID pendaftaran tidak valid")
	}
	var payload struct {
		Catatan string `json:"catatan"`
	}
	_ = c.BodyParser(&payload)
	if err := h.service.ProcessApproval(c.Context(), id, domain.PendaftaranActionSetujui, payload.Catatan); err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Pendaftaran disetujui", nil)
}

func (h *PendaftaranHandler) VerifyKTA(c *fiber.Ctx) error {
	nomor := strings.TrimSpace(c.Params("nomor"))
	if nomor == "" {
		return response.BadRequest(c, "Nomor registrasi tidak valid")
	}
	return response.Success(c, "Verifikasi KTA", fiber.Map{"nomor": nomor, "status": "valid", "signature": fmt.Sprintf("sig-%s", nomor)})
}
