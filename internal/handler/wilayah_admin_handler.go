package handler

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kipan-indonesia/sim-kipan-core/internal/service"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/response"
)

// WilayahAdminHandler melayani master wilayah (Super/Nasional).
type WilayahAdminHandler struct {
	service service.WilayahAdminService
}

func NewWilayahAdminHandler(service service.WilayahAdminService) *WilayahAdminHandler {
	return &WilayahAdminHandler{service: service}
}

func (h *WilayahAdminHandler) Cards(c *fiber.Ctx) error {
	actor, ok := actorOf(c)
	if !ok {
		return response.Unauthorized(c, "Tidak terotentikasi")
	}
	out, err := h.service.Cards(c.Context(), actor)
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Ringkasan wilayah", out)
}

func (h *WilayahAdminHandler) List(c *fiber.Ctx) error {
	actor, ok := actorOf(c)
	if !ok {
		return response.Unauthorized(c, "Tidak terotentikasi")
	}
	var prov *int
	if n := c.QueryInt("provinsi_id", 0); n > 0 {
		prov = &n
	}
	items, err := h.service.List(c.Context(), actor, c.Query("type"), c.Query("search"), c.Query("status"), prov)
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Daftar wilayah", items)
}

func (h *WilayahAdminHandler) Detail(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return response.BadRequest(c, "ID wilayah tidak valid")
	}
	actor, ok := actorOf(c)
	if !ok {
		return response.Unauthorized(c, "Tidak terotentikasi")
	}
	out, err := h.service.Detail(c.Context(), actor, c.Params("type"), id)
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Detail wilayah", out)
}

func (h *WilayahAdminHandler) Pengurus(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return response.BadRequest(c, "ID wilayah tidak valid")
	}
	actor, ok := actorOf(c)
	if !ok {
		return response.Unauthorized(c, "Tidak terotentikasi")
	}
	items, err := h.service.Pengurus(c.Context(), actor, c.Params("type"), id, c.QueryBool("all", false))
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Pengurus wilayah", items)
}

func (h *WilayahAdminHandler) SetStatus(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return response.BadRequest(c, "ID wilayah tidak valid")
	}
	actor, ok := actorOf(c)
	if !ok {
		return response.Unauthorized(c, "Tidak terotentikasi")
	}
	var body struct {
		IsActive bool `json:"is_active"`
	}
	if err := c.BodyParser(&body); err != nil {
		return response.BadRequest(c, "Format permintaan tidak valid")
	}
	if err := h.service.SetStatus(c.Context(), actor, auditContextOf(c), c.Params("type"), id, body.IsActive); err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Status wilayah diperbarui", nil)
}

func (h *WilayahAdminHandler) Add(c *fiber.Ctx) error {
	actor, ok := actorOf(c)
	if !ok {
		return response.Unauthorized(c, "Tidak terotentikasi")
	}
	var body struct {
		Type        string `json:"type"`
		ProvinsiID  int    `json:"provinsi_id"`
		KabupatenID int    `json:"kabupaten_id"`
	}
	if err := c.BodyParser(&body); err != nil {
		return response.BadRequest(c, "Format permintaan tidak valid")
	}
	if err := h.service.Add(c.Context(), actor, auditContextOf(c), body.Type, body.ProvinsiID, body.KabupatenID); err != nil {
		return response.FromError(c, err)
	}
	return response.Created(c, "Wilayah diaktifkan", nil)
}
