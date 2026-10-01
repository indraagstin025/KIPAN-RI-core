package handler

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/kipan-indonesia/sim-kipan-core/internal/service"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/response"
)

// AnggotaHandler melayani daftar kader untuk admin dan cek publik minimal
// (thin-handler: parse request, panggil service, format response).
type AnggotaHandler struct {
	service service.AnggotaService
}

func NewAnggotaHandler(service service.AnggotaService) *AnggotaHandler {
	return &AnggotaHandler{service: service}
}

func (h *AnggotaHandler) List(c *fiber.Ctx) error {
	actor, ok := actorOf(c)
	if !ok {
		return response.Unauthorized(c, "Tidak terotentikasi")
	}
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 25)
	items, total, err := h.service.ListAnggota(c.Context(), actor, c.Query("status"), c.Query("search"), page, limit)
	if err != nil {
		return response.FromError(c, err)
	}
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}
	pages := 0
	if total > 0 {
		pages = (total + limit - 1) / limit
	}
	return response.Paginated(c, "Daftar anggota", items, fiber.Map{
		"page":        page,
		"per_page":    limit,
		"total":       total,
		"total_pages": pages,
	})
}

func (h *AnggotaHandler) Detail(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return response.BadRequest(c, "ID anggota tidak valid")
	}
	actor, ok := actorOf(c)
	if !ok {
		return response.Unauthorized(c, "Tidak terotentikasi")
	}
	item, err := h.service.GetAnggotaDetail(c.Context(), id, actor)
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Detail anggota", item)
}

// CheckPublic adalah pengganti cek-anggota lama: pencarian hanya by NIA,
// tanpa NIK, alamat, kontak, maupun object key. Rate-limit ketat anti scraping.
func (h *AnggotaHandler) CheckPublic(c *fiber.Ctx) error {
	q := strings.TrimSpace(c.Query("q"))
	if q == "" {
		return response.BadRequest(c, "Nomor identitas wajib diisi")
	}
	info, err := h.service.GetPublicAnggota(c.Context(), q)
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Data keanggotaan ditemukan", info)
}
