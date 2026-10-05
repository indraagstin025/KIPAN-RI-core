package handler

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kipan-indonesia/sim-kipan-core/internal/service/kta"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/response"
)

// KTAHandler melayani unduhan dokumen KTA (thin-handler, teraudit).
type KTAHandler struct {
	service kta.KTAService
}

func NewKTAHandler(service kta.KTAService) *KTAHandler {
	return &KTAHandler{service: service}
}

// DownloadKTA mengembalikan tiket baca sementara PDF KTA untuk admin
// dalam yurisdiksi anggota. Akses dicatat di audit trail.
func (h *KTAHandler) DownloadKTA(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return response.BadRequest(c, "ID anggota tidak valid")
	}
	actor, ok := actorOf(c)
	if !ok {
		return response.Unauthorized(c, "Tidak terotentikasi")
	}
	url, err := h.service.GetKTADocumentURL(c.Context(), id, actor, auditContextOf(c))
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Tiket unduh KTA diterbitkan", fiber.Map{"download_url": url})
}

// DownloadMyKTA mengembalikan tiket baca PDF KTA milik akun USER sendiri
// (Batch 2). Route dipagari RequireRoles(USER); kepemilikan ditegakkan
// di service via anggota.user_id.
func (h *KTAHandler) DownloadMyKTA(c *fiber.Ctx) error {
	actor, ok := actorOf(c)
	if !ok {
		return response.Unauthorized(c, "Tidak terotentikasi")
	}
	url, err := h.service.GetMyKTADocumentURL(c.Context(), actor.UserID, auditContextOf(c))
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Tiket unduh KTA diterbitkan", fiber.Map{"download_url": url})
}
