package handler

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kipan-indonesia/sim-kipan-core/internal/service/dokumen"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/response"
)

// BackupHandler melayani manajemen backup database (Super Admin).
type BackupHandler struct {
	service dokumen.BackupService
}

// NewBackupHandler membangun handler backup.
func NewBackupHandler(service dokumen.BackupService) *BackupHandler {
	return &BackupHandler{service: service}
}

// List menampilkan riwayat backup (GET /admin/backups).
func (h *BackupHandler) List(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 25)
	items, total, err := h.service.List(c.Context(), page, limit)
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
	return response.Paginated(c, "Riwayat backup", items, fiber.Map{
		"page": page, "per_page": limit, "total": total, "total_pages": pages,
	})
}

// Create membuat backup baru (POST /admin/backups).
func (h *BackupHandler) Create(c *fiber.Ctx) error {
	actor, ok := actorOf(c)
	if !ok {
		return response.Unauthorized(c, "Tidak terotentikasi")
	}
	out, err := h.service.Create(c.Context(), actor, auditContextOf(c))
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Created(c, "Backup dibuat", out)
}

// Download menerbitkan tiket unduh arsip backup.
func (h *BackupHandler) Download(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return response.BadRequest(c, "ID backup tidak valid")
	}
	url, err := h.service.DownloadURL(c.Context(), id)
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Tiket unduh backup", fiber.Map{"download_url": url})
}

// Delete menghapus arsip + riwayat backup.
func (h *BackupHandler) Delete(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return response.BadRequest(c, "ID backup tidak valid")
	}
	actor, ok := actorOf(c)
	if !ok {
		return response.Unauthorized(c, "Tidak terotentikasi")
	}
	if err := h.service.Delete(c.Context(), id, actor, auditContextOf(c)); err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Backup dihapus", nil)
}
