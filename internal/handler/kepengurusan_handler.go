package handler

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/kipan-indonesia/sim-kipan-core/internal/domain"
	"github.com/kipan-indonesia/sim-kipan-core/internal/service"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/response"
)

// KepengurusanHandler melayani SK, jabatan, dan pengurus (thin-handler).
type KepengurusanHandler struct {
	service service.KepengurusanService
}

func NewKepengurusanHandler(service service.KepengurusanService) *KepengurusanHandler {
	return &KepengurusanHandler{service: service}
}

// ============================================================
// JABATAN
// ============================================================

func (h *KepengurusanHandler) ListJabatan(c *fiber.Ctx) error {
	includeInactive := c.QueryBool("include_inactive", false)
	items, err := h.service.ListJabatan(c.Context(), includeInactive)
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Daftar jabatan", items)
}

func (h *KepengurusanHandler) CreateJabatan(c *fiber.Ctx) error {
	actor, ok := actorOf(c)
	if !ok {
		return response.Unauthorized(c, "Tidak terotentikasi")
	}
	var body domain.JabatanRequest
	if err := c.BodyParser(&body); err != nil {
		return response.BadRequest(c, "Format permintaan tidak valid")
	}
	out, err := h.service.CreateJabatan(c.Context(), body, actor, auditContextOf(c))
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Created(c, "Jabatan dibuat", out)
}

func (h *KepengurusanHandler) UpdateJabatan(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return response.BadRequest(c, "ID jabatan tidak valid")
	}
	actor, ok := actorOf(c)
	if !ok {
		return response.Unauthorized(c, "Tidak terotentikasi")
	}
	var body domain.JabatanRequest
	if err := c.BodyParser(&body); err != nil {
		return response.BadRequest(c, "Format permintaan tidak valid")
	}
	out, err := h.service.UpdateJabatan(c.Context(), id, body, actor, auditContextOf(c))
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Jabatan diperbarui", out)
}

// ============================================================
// SURAT KEPUTUSAN
// ============================================================

func (h *KepengurusanHandler) CreateSK(c *fiber.Ctx) error {
	actor, ok := actorOf(c)
	if !ok {
		return response.Unauthorized(c, "Tidak terotentikasi")
	}
	var body domain.SKCreateRequest
	if err := c.BodyParser(&body); err != nil {
		return response.BadRequest(c, "Format permintaan tidak valid")
	}
	out, err := h.service.CreateSK(c.Context(), body, actor, auditContextOf(c))
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Created(c, "Surat Keputusan dibuat", out)
}

func (h *KepengurusanHandler) ListSK(c *fiber.Ctx) error {
	actor, ok := actorOf(c)
	if !ok {
		return response.Unauthorized(c, "Tidak terotentikasi")
	}
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 25)
	items, total, err := h.service.ListSK(c.Context(), actor,
		c.Query("level"), c.Query("status"), c.Query("approval"), c.Query("search"),
		c.QueryBool("with_total", true), page, limit)
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Paginated(c, "Daftar Surat Keputusan", items, paginationMeta(page, limit, total))
}

func (h *KepengurusanHandler) GetSK(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return response.BadRequest(c, "ID SK tidak valid")
	}
	actor, ok := actorOf(c)
	if !ok {
		return response.Unauthorized(c, "Tidak terotentikasi")
	}
	detail, err := h.service.GetSK(c.Context(), id, actor)
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Detail Surat Keputusan", detail)
}

func (h *KepengurusanHandler) ApproveSK(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return response.BadRequest(c, "ID SK tidak valid")
	}
	actor, ok := actorOf(c)
	if !ok {
		return response.Unauthorized(c, "Tidak terotentikasi")
	}
	var body struct {
		Action  string `json:"action"`
		Catatan string `json:"catatan"`
	}
	if err := c.BodyParser(&body); err != nil {
		return response.BadRequest(c, "Format permintaan tidak valid")
	}
	action := domain.SKApprovalAction(strings.ToUpper(strings.TrimSpace(body.Action)))
	if err := h.service.ApproveSK(c.Context(), id, action, body.Catatan, actor, auditContextOf(c)); err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Status SK diperbarui", nil)
}

func (h *KepengurusanHandler) SetSKStatus(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return response.BadRequest(c, "ID SK tidak valid")
	}
	actor, ok := actorOf(c)
	if !ok {
		return response.Unauthorized(c, "Tidak terotentikasi")
	}
	var body struct {
		Status         string `json:"status"`
		StatusPengurus string `json:"status_pengurus"`
		Keterangan     string `json:"keterangan"`
	}
	if err := c.BodyParser(&body); err != nil {
		return response.BadRequest(c, "Format permintaan tidak valid")
	}
	status := domain.SKStatus(strings.TrimSpace(body.Status))
	pengurusStatus := domain.PengurusStatus(strings.TrimSpace(body.StatusPengurus))
	if err := h.service.SetSKStatus(c.Context(), id, status, pengurusStatus, body.Keterangan, actor, auditContextOf(c)); err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Status SK diperbarui", nil)
}

// ============================================================
// PENGURUS
// ============================================================

func (h *KepengurusanHandler) AddPengurus(c *fiber.Ctx) error {
	skID, err := c.ParamsInt("id")
	if err != nil || skID <= 0 {
		return response.BadRequest(c, "ID SK tidak valid")
	}
	actor, ok := actorOf(c)
	if !ok {
		return response.Unauthorized(c, "Tidak terotentikasi")
	}
	var body domain.AddPengurusRequest
	if err := c.BodyParser(&body); err != nil {
		return response.BadRequest(c, "Format permintaan tidak valid")
	}
	out, err := h.service.AddPengurus(c.Context(), skID, body, actor, auditContextOf(c))
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Created(c, "Pengurus ditambahkan", out)
}

func (h *KepengurusanHandler) RemovePengurus(c *fiber.Ctx) error {
	skID, err := c.ParamsInt("id")
	if err != nil || skID <= 0 {
		return response.BadRequest(c, "ID SK tidak valid")
	}
	pengurusID, err := c.ParamsInt("pengurusId")
	if err != nil || pengurusID <= 0 {
		return response.BadRequest(c, "ID pengurus tidak valid")
	}
	actor, ok := actorOf(c)
	if !ok {
		return response.Unauthorized(c, "Tidak terotentikasi")
	}
	if err := h.service.RemovePengurus(c.Context(), skID, pengurusID, actor, auditContextOf(c)); err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Pengurus dilepas dari SK", nil)
}

// PengurusStats mengembalikan ringkasan jumlah pengurus (ter-scope).
func (h *KepengurusanHandler) PengurusStats(c *fiber.Ctx) error {
	actor, ok := actorOf(c)
	if !ok {
		return response.Unauthorized(c, "Tidak terotentikasi")
	}
	out, err := h.service.PengurusStats(c.Context(), actor)
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Statistik pengurus", out)
}

// ListPromosi mengembalikan kandidat promosi pengurus (ter-scope).
func (h *KepengurusanHandler) ListPromosi(c *fiber.Ctx) error {
	actor, ok := actorOf(c)
	if !ok {
		return response.Unauthorized(c, "Tidak terotentikasi")
	}
	items, err := h.service.ListPromosi(c.Context(), actor, c.Query("search"), c.QueryInt("limit", 20))
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Kandidat promosi pengurus", items)
}

func (h *KepengurusanHandler) ListPengurus(c *fiber.Ctx) error {
	actor, ok := actorOf(c)
	if !ok {
		return response.Unauthorized(c, "Tidak terotentikasi")
	}
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 25)
	items, total, err := h.service.ListPengurus(c.Context(), actor,
		c.Query("level"), c.Query("status"), c.Query("masa_jabatan"), c.Query("search"),
		optionalQueryInt(c, "provinsi_id"), optionalQueryInt(c, "kabupaten_id"),
		c.QueryBool("with_total", true), page, limit)
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Paginated(c, "Daftar pengurus", items, paginationMeta(page, limit, total))
}

// optionalQueryInt mengembalikan pointer int bila query ada dan > 0.
func optionalQueryInt(c *fiber.Ctx, key string) *int {
	if n := c.QueryInt(key, 0); n > 0 {
		return &n
	}
	return nil
}

func (h *KepengurusanHandler) UpdatePengurusStatus(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return response.BadRequest(c, "ID pengurus tidak valid")
	}
	actor, ok := actorOf(c)
	if !ok {
		return response.Unauthorized(c, "Tidak terotentikasi")
	}
	var body struct {
		Status     string `json:"status"`
		Keterangan string `json:"keterangan"`
	}
	if err := c.BodyParser(&body); err != nil {
		return response.BadRequest(c, "Format permintaan tidak valid")
	}
	status := domain.PengurusStatus(strings.TrimSpace(body.Status))
	if err := h.service.UpdatePengurusStatus(c.Context(), id, status, body.Keterangan, actor, auditContextOf(c)); err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Status pengurus diperbarui", nil)
}

// UpdatePengurusJabatan mengganti jabatan pengurus (PATCH /admin/pengurus/:id/jabatan).
func (h *KepengurusanHandler) UpdatePengurusJabatan(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return response.BadRequest(c, "ID pengurus tidak valid")
	}
	actor, ok := actorOf(c)
	if !ok {
		return response.Unauthorized(c, "Tidak terotentikasi")
	}
	var body domain.UpdateJabatanRequest
	if err := c.BodyParser(&body); err != nil {
		return response.BadRequest(c, "Format permintaan tidak valid")
	}
	out, err := h.service.UpdatePengurusJabatan(c.Context(), id, body, actor, auditContextOf(c))
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Jabatan pengurus diperbarui", out)
}

// Paws mengakhiri masa bakti individual (PUT /admin/pengurus/:id/paw).
func (h *KepengurusanHandler) Paws(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return response.BadRequest(c, "ID pengurus tidak valid")
	}
	actor, ok := actorOf(c)
	if !ok {
		return response.Unauthorized(c, "Tidak terotentikasi")
	}
	var body domain.PawsRequest
	if err := c.BodyParser(&body); err != nil {
		return response.BadRequest(c, "Format permintaan tidak valid")
	}
	if err := h.service.Paws(c.Context(), id, body, actor, auditContextOf(c)); err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Aksi PAW berhasil", nil)
}

// Mutasi memindahkan pengurus ke SK/jabatan tujuan (PUT /admin/pengurus/:id/mutasi).
func (h *KepengurusanHandler) Mutasi(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return response.BadRequest(c, "ID pengurus tidak valid")
	}
	actor, ok := actorOf(c)
	if !ok {
		return response.Unauthorized(c, "Tidak terotentikasi")
	}
	var body domain.MutasiRequest
	if err := c.BodyParser(&body); err != nil {
		return response.BadRequest(c, "Format permintaan tidak valid")
	}
	out, err := h.service.Mutasi(c.Context(), id, body, actor, auditContextOf(c))
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Pengurus berhasil dimutasi", out)
}

// paginationMeta menghitung meta pagination standar.
func paginationMeta(page, limit, total int) fiber.Map {
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
	return fiber.Map{
		"page":        page,
		"per_page":    limit,
		"total":       total,
		"total_pages": pages,
	}
}
