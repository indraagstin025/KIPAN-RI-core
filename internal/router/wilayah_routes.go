package router

// wilayah_routes.go mendaftarkan master wilayah publik (read-only) untuk
// dropdown form & pencarian lintas-wilayah.

import (
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"

	"github.com/kipan-indonesia/sim-kipan-core/internal/handler"
	"github.com/kipan-indonesia/sim-kipan-core/internal/middleware"
)

// registerWilayahRoutes mendaftarkan endpoint /wilayah (provinsi, kabupaten,
// kecamatan, desa, kodepos) dengan rate-limit ringan.
func registerWilayahRoutes(
	v1 fiber.Router,
	rdb *redis.Client,
	h *handler.WilayahHandler,
) {
	wilayahLimiter := middleware.RateLimit(rdb, "wil_pub")

	public := v1.Group("/wilayah")
	public.Use(wilayahLimiter)
	public.Get("/provinsi", h.ListProvinsi)
	public.Get("/kabupaten/:provinsi_id", h.ListKabupaten)
	public.Get("/kecamatan", h.ListKecamatan)
	public.Get("/desa", h.ListDesa)
	public.Get("/kodepos", h.ListKodepos)
}
