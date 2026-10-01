package handler

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
)

// HealthHandler mengelola endpoint status kesehatan layanan
type HealthHandler struct {
	appName string
	env     string
	db      *sqlx.DB
	rdb     *redis.Client
}

// NewHealthHandler membuat instance HealthHandler
func NewHealthHandler(appName, env string, db *sqlx.DB, rdb *redis.Client) *HealthHandler {
	return &HealthHandler{
		appName: appName,
		env:     env,
		db:      db,
		rdb:     rdb,
	}
}

// Check adalah endpoint PUBLIK minimal (BE-004): hanya status, tanpa env,
// tanpa status dependensi (itu oracle kesiapan infra untuk penyerang).
func (h *HealthHandler) Check(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status": "ok",
		"time":   time.Now().Format(time.RFC3339),
	})
}

// Detail adalah endpoint INTERNAL untuk monitoring: wajib dibatasi di
// reverse proxy (Caddy: hanya dari jaringan internal/VPN, JANGAN expose
// /internal/* ke internet). Lihat registerStorageRoutes → internal group.
func (h *HealthHandler) Detail(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status":   "ok",
		"service":  h.appName,
		"env":      h.env,
		"time":     time.Now().Format(time.RFC3339),
		"database": h.checkDB(),
		"redis":    h.checkRedis(c.Context()),
	})
}

func (h *HealthHandler) checkDB() string {
	if h.db == nil {
		return "disabled"
	}
	if err := h.db.Ping(); err != nil {
		return "disconnected"
	}
	return "connected"
}

func (h *HealthHandler) checkRedis(ctx context.Context) string {
	if h.rdb == nil {
		return "disabled"
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()
	if err := h.rdb.Ping(timeoutCtx).Err(); err != nil {
		return "disconnected"
	}
	return "connected"
}