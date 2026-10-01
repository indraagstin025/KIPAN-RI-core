package handler

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kipan-indonesia/sim-kipan-core/internal/service"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/response"
)

// WilayahHandler melayani daftar master wilayah (publik, read-only).
type WilayahHandler struct {
	service service.WilayahService
}

func NewWilayahHandler(service service.WilayahService) *WilayahHandler {
	return &WilayahHandler{service: service}
}

func (h *WilayahHandler) ListProvinsi(c *fiber.Ctx) error {
	items, err := h.service.ListProvinsi(c.Context())
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Daftar provinsi", items)
}

func (h *WilayahHandler) ListKabupaten(c *fiber.Ctx) error {
	provID, err := c.ParamsInt("provinsi_id")
	if err != nil || provID <= 0 {
		return response.BadRequest(c, "ID provinsi tidak valid")
	}
	items, err := h.service.ListKabupaten(c.Context(), provID)
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Daftar kabupaten/kota", items)
}
