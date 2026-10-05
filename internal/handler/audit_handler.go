package handler

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kipan-indonesia/sim-kipan-core/internal/service/insight"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/response"
)

// AuditHandler melayani penelusur jejak audit (thin-handler).
type AuditHandler struct {
	service insight.AuditService
}

// NewAuditHandler membangun handler audit.
func NewAuditHandler(service insight.AuditService) *AuditHandler {
	return &AuditHandler{service: service}
}

// List menelusuri audit (GET /admin/audit?aksi=&entitas=&aktor=&dari=&ke=).
func (h *AuditHandler) List(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 25)
	f := insight.ParseAuditFilter(
		c.Query("aksi"), c.Query("entitas"), c.Query("aktor"), c.Query("dari"), c.Query("ke"),
		page, limit, true)
	items, total, err := h.service.List(c.Context(), f)
	if err != nil {
		return response.FromError(c, err)
	}
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = 25
	}
	pages := 0
	if total > 0 {
		pages = (total + limit - 1) / limit
	}
	return response.Paginated(c, "Jejak audit", items, fiber.Map{
		"page":        page,
		"per_page":    limit,
		"total":       total,
		"total_pages": pages,
	})
}

// Export mengunduh jejak audit terfilter sebagai CSV.
func (h *AuditHandler) Export(c *fiber.Ctx) error {
	f := insight.ParseAuditFilter(
		c.Query("aksi"), c.Query("entitas"), c.Query("aktor"), c.Query("dari"), c.Query("ke"),
		1, 200, false)
	data, err := h.service.ExportCSV(c.Context(), f)
	if err != nil {
		return response.FromError(c, err)
	}
	c.Set("Content-Type", "text/csv; charset=utf-8")
	c.Set("Content-Disposition", `attachment; filename="jejak-audit.csv"`)
	return c.Send(data)
}
