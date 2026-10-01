package handler

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kipan-indonesia/sim-kipan-core/internal/service"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/response"
)

// NotificationHandler melayani notifikasi milik sendiri (thin-handler).
// Tidak ada parameter userId: identitas selalu dari JWT terverifikasi.
type NotificationHandler struct {
	service service.NotificationService
}

func NewNotificationHandler(service service.NotificationService) *NotificationHandler {
	return &NotificationHandler{service: service}
}

func (h *NotificationHandler) Mine(c *fiber.Ctx) error {
	actor, ok := actorOf(c)
	if !ok {
		return response.Unauthorized(c, "Tidak terotentikasi")
	}
	limit := c.QueryInt("limit", 25)
	items, err := h.service.ListMine(c.Context(), actor, limit)
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Daftar notifikasi", items)
}

func (h *NotificationHandler) MarkRead(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return response.BadRequest(c, "ID notifikasi tidak valid")
	}
	actor, ok := actorOf(c)
	if !ok {
		return response.Unauthorized(c, "Tidak terotentikasi")
	}
	if err := h.service.MarkRead(c.Context(), int64(id), actor); err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Notifikasi ditandai dibaca", nil)
}
