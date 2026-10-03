package handler

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/service"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/response"
)

// UserAdminHandler melayani manajemen akun admin (Super Admin).
type UserAdminHandler struct {
	service service.UserAdminService
}

func NewUserAdminHandler(service service.UserAdminService) *UserAdminHandler {
	return &UserAdminHandler{service: service}
}

func (h *UserAdminHandler) List(c *fiber.Ctx) error {
	actor, ok := actorOf(c)
	if !ok {
		return response.Unauthorized(c, "Tidak terotentikasi")
	}
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 25)
	items, total, err := h.service.List(c.Context(), actor,
		c.Query("role"), c.Query("status"), c.Query("search"), page, limit)
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Paginated(c, "Daftar pengguna", items, paginationMeta(page, limit, total))
}

func (h *UserAdminHandler) Counts(c *fiber.Ctx) error {
	actor, ok := actorOf(c)
	if !ok {
		return response.Unauthorized(c, "Tidak terotentikasi")
	}
	out, err := h.service.Counts(c.Context(), actor)
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Ringkasan pengguna", out)
}

func (h *UserAdminHandler) Create(c *fiber.Ctx) error {
	actor, ok := actorOf(c)
	if !ok {
		return response.Unauthorized(c, "Tidak terotentikasi")
	}
	var body domain.UserCreateRequest
	if err := c.BodyParser(&body); err != nil {
		return response.BadRequest(c, "Format permintaan tidak valid")
	}
	out, err := h.service.Create(c.Context(), actor, auditContextOf(c), body)
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Created(c, "Pengguna dibuat", out)
}

func (h *UserAdminHandler) Update(c *fiber.Ctx) error {
	id := c.Params("id")
	actor, ok := actorOf(c)
	if !ok {
		return response.Unauthorized(c, "Tidak terotentikasi")
	}
	var body domain.UserUpdateRequest
	if err := c.BodyParser(&body); err != nil {
		return response.BadRequest(c, "Format permintaan tidak valid")
	}
	out, err := h.service.Update(c.Context(), actor, auditContextOf(c), id, body)
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Pengguna diperbarui", out)
}

func (h *UserAdminHandler) Delete(c *fiber.Ctx) error {
	id := c.Params("id")
	actor, ok := actorOf(c)
	if !ok {
		return response.Unauthorized(c, "Tidak terotentikasi")
	}
	if err := h.service.Delete(c.Context(), actor, auditContextOf(c), id); err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Pengguna dihapus", nil)
}
