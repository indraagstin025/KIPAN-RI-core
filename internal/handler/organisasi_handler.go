package handler

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/service/platform"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/response"
)

// OrganisasiHandler melayani profil organisasi (publik + admin).
type OrganisasiHandler struct {
	service platform.OrganisasiService
}

// NewOrganisasiHandler membangun handler profil organisasi.
func NewOrganisasiHandler(service platform.OrganisasiService) *OrganisasiHandler {
	return &OrganisasiHandler{service: service}
}

// Get mengembalikan profil organisasi (publik).
func (h *OrganisasiHandler) Get(c *fiber.Ctx) error {
	out, err := h.service.Get(c.Context())
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Profil organisasi", out)
}

// Update menyimpan profil organisasi (PUT /admin/organisasi).
func (h *OrganisasiHandler) Update(c *fiber.Ctx) error {
	actor, ok := actorOf(c)
	if !ok {
		return response.Unauthorized(c, "Tidak terotentikasi")
	}
	var body domain.OrganisasiUpdateRequest
	if err := c.BodyParser(&body); err != nil {
		return response.BadRequest(c, "Format permintaan tidak valid")
	}
	out, err := h.service.Update(c.Context(), actor, auditContextOf(c), body)
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Profil organisasi diperbarui", out)
}
