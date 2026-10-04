package handler

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kipan-indonesia/sim-kipan-core/internal/service"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/response"
)

// OutboxHandler melayani pemantauan antrian email (outbox) admin.
type OutboxHandler struct {
	service service.OutboxService
}

func NewOutboxHandler(service service.OutboxService) *OutboxHandler {
	return &OutboxHandler{service: service}
}

func (h *OutboxHandler) List(c *fiber.Ctx) error {
	actor, ok := actorOf(c)
	if !ok {
		return response.Unauthorized(c, "Tidak terotentikasi")
	}
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 25)
	items, total, err := h.service.List(c.Context(), actor, c.Query("jenis"), c.Query("status"), page, limit)
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Paginated(c, "Daftar antrian email", items, paginationMeta(page, limit, total))
}

func (h *OutboxHandler) Retry(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return response.BadRequest(c, "ID antrian tidak valid")
	}
	actor, ok := actorOf(c)
	if !ok {
		return response.Unauthorized(c, "Tidak terotentikasi")
	}
	if err := h.service.Retry(c.Context(), actor, int64(id)); err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Email dijadwalkan kirim ulang", nil)
}

// Send mengirim satu email antrian SEKARANG (sinkron).
func (h *OutboxHandler) Send(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return response.BadRequest(c, "ID antrian tidak valid")
	}
	actor, ok := actorOf(c)
	if !ok {
		return response.Unauthorized(c, "Tidak terotentikasi")
	}
	if err := h.service.SendNow(c.Context(), actor, int64(id)); err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Email terkirim", nil)
}

func (h *OutboxHandler) RetryPending(c *fiber.Ctx) error {
	actor, ok := actorOf(c)
	if !ok {
		return response.Unauthorized(c, "Tidak terotentikasi")
	}
	n, err := h.service.RetryPending(c.Context(), actor)
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Email pending dijadwalkan kirim", fiber.Map{"count": n})
}

func (h *OutboxHandler) RetryMany(c *fiber.Ctx) error {
	actor, ok := actorOf(c)
	if !ok {
		return response.Unauthorized(c, "Tidak terotentikasi")
	}
	var body struct {
		IDs []int64 `json:"ids"`
	}
	if err := c.BodyParser(&body); err != nil {
		return response.BadRequest(c, "Format permintaan tidak valid")
	}
	n, err := h.service.RetryMany(c.Context(), actor, body.IDs)
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Email terpilih dijadwalkan kirim ulang", fiber.Map{"count": n})
}
