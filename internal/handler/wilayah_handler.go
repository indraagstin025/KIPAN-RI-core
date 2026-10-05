package handler

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/kipan-indonesia/sim-kipan-core/internal/service/wilayah"
	"github.com/kipan-indonesia/sim-kipan-core/pkg/response"
)

// WilayahHandler melayani daftar master wilayah (publik, read-only).
type WilayahHandler struct {
	service wilayah.WilayahService
}

func NewWilayahHandler(service wilayah.WilayahService) *WilayahHandler {
	return &WilayahHandler{service: service}
}

func (h *WilayahHandler) ListProvinsi(c *fiber.Ctx) error {
	items, err := h.service.ListProvinsi(c.Context())
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Daftar provinsi", items)
}

func (h *WilayahHandler) ListKabupaten(c *fiber.Ctx) error {
	provID, err := c.ParamsInt("provinsi_id")
	if err != nil || provID <= 0 {
		return response.BadRequest(c, "ID provinsi tidak valid")
	}
	items, err := h.service.ListKabupaten(c.Context(), provID)
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Daftar kabupaten/kota", items)
}

// ListKecamatan mengembalikan saran kecamatan (proxy wilayah.id, fail-open).
// Query memakai kode BPS 4 digit tanpa titik (cth. 3273), selaras kolom kode.
func (h *WilayahHandler) ListKecamatan(c *fiber.Ctx) error {
	kode := strings.TrimSpace(c.Query("kabupaten_kode"))
	if len(kode) != 4 {
		return response.BadRequest(c, "Kode kabupaten tidak valid (4 digit angka)")
	}
	for _, r := range kode {
		if r < '0' || r > '9' {
			return response.BadRequest(c, "Kode kabupaten tidak valid (4 digit angka)")
		}
	}
	items, err := h.service.ListKecamatan(c.Context(), kode)
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Daftar kecamatan", items)
}

// ListDesa mengembalikan saran desa/kelurahan (proxy wilayah.id, fail-open).
// Query memakai kode BPS 6 digit tanpa titik (cth. 327308).
func (h *WilayahHandler) ListDesa(c *fiber.Ctx) error {
	kode := strings.TrimSpace(c.Query("kecamatan_kode"))
	if len(kode) != 6 {
		return response.BadRequest(c, "Kode kecamatan tidak valid (6 digit angka)")
	}
	for _, r := range kode {
		if r < '0' || r > '9' {
			return response.BadRequest(c, "Kode kecamatan tidak valid (6 digit angka)")
		}
	}
	items, err := h.service.ListDesa(c.Context(), kode)
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Daftar desa/kelurahan", items)
}

// ListKodepos mengembalikan saran kode pos (kode saja) yang dicocokkan
// desa → kecamatan → kabupaten. Minimal desa atau kecamatan terisi.
func (h *WilayahHandler) ListKodepos(c *fiber.Ctx) error {
	desa := strings.TrimSpace(c.Query("desa"))
	kecamatan := strings.TrimSpace(c.Query("kecamatan"))
	kabupaten := strings.TrimSpace(c.Query("kabupaten"))
	for _, v := range []string{desa, kecamatan, kabupaten} {
		if v != "" && (len([]rune(v)) > 100 || strings.ContainsAny(v, "<>")) {
			return response.BadRequest(c, "Nama wilayah tidak valid")
		}
	}
	if desa == "" && kecamatan == "" {
		return response.BadRequest(c, "Desa atau kecamatan wajib diisi untuk mencari kode pos")
	}
	items, err := h.service.ListKodepos(c.Context(), desa, kecamatan, kabupaten)
	if err != nil {
		return response.FromError(c, err)
	}
	return response.Success(c, "Daftar kode pos", items)
}
