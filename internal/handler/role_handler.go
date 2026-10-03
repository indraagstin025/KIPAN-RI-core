package handler

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kipan-indonesia/sim-kipan-core/internal/service"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/response"
)

// RoleHandler menyajikan katalog Role & Wewenang (read-only).
type RoleHandler struct {
	service service.RoleService
}

// NewRoleHandler membangun handler katalog role.
func NewRoleHandler(service service.RoleService) *RoleHandler {
	return &RoleHandler{service: service}
}

// Catalog mengembalikan katalog role + matriks wewenang (GET /admin/roles).
func (h *RoleHandler) Catalog(c *fiber.Ctx) error {
	return response.Success(c, "Katalog role & wewenang", h.service.Catalog(c.Context()))
}
