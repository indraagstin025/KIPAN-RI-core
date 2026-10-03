package handler

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kipan-indonesia/sim-kipan-core/internal/service"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/response"
)

// LaporanHandler melayani laporan & statistik ter-scope (thin-handler).
type LaporanHandler struct {
	service service.LaporanService
}

// NewLaporanHandler membangun handler laporan.
func NewLaporanHandler(service service.LaporanService) *LaporanHandler {
	return &LaporanHandler{service: service}
}

// Get mengembalikan agregat laporan (GET /admin/laporan).
func (h *LaporanHandler) Get(c *fiber.Ctx) error {
	actor, ok := actorOf(c)
	if !ok {
		return response.Unauthorized(c, "Tidak terotentikasi")
	}
	data, err := h.service.Load(c.Context(), actor)
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Laporan & statistik", data)
}

// Export mengunduh laporan sebagai CSV (GET /admin/laporan/export.csv).
func (h *LaporanHandler) Export(c *fiber.Ctx) error {
	actor, ok := actorOf(c)
	if !ok {
		return response.Unauthorized(c, "Tidak terotentikasi")
	}
	data, err := h.service.ExportCSV(c.Context(), actor)
	if err != nil {
		return response.FromError(c, err)
	}
	c.Set("Content-Type", "text/csv; charset=utf-8")
	c.Set("Content-Disposition", `attachment; filename="laporan.csv"`)
	return c.Send(data)
}
