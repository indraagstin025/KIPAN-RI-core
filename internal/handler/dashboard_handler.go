package handler

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kipan-indonesia/sim-kipan-core/internal/service/insight"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/response"
)

// DashboardHandler melayani ringkasan analitik admin.
type DashboardHandler struct {
	service insight.DashboardService
}

func NewDashboardHandler(service insight.DashboardService) *DashboardHandler {
	return &DashboardHandler{service: service}
}

// Get mengembalikan agregat dashboard ter-scope actor (role/wilayah dari JWT).
func (h *DashboardHandler) Get(c *fiber.Ctx) error {
	actor, ok := actorOf(c)
	if !ok {
		return response.Unauthorized(c, "Tidak terotentikasi")
	}
	data, err := h.service.Load(c.Context(), actor)
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Ringkasan dashboard", data)
}
