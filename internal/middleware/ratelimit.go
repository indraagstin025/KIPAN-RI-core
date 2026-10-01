package middleware

// Registry kebijakan rate limit per-IP (R4): seluruh kuota didefinisikan
// SEKALI di sini. Route hanya menyebut nama — prefix string manual di
// routes.go dilarang karena pernah menyebabkan berbagi kuota Redis dan
// 429 prematur. Nama tak dikenal = panic saat wiring (fail-fast di dev,
// bukan 429 misterius di production).

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
)

type ratePolicy struct {
	max    int
	window time.Duration
}

var ratePolicies = map[string]ratePolicy{
	"auth":    {max: 20, window: time.Minute}, // login/refresh/logout/password
	"mem_pub": {max: 30, window: time.Minute}, // submit/track/revisi/KTA publik
	"mem_mut": {max: 20, window: time.Minute}, // mutasi admin membership
	"stor_up": {max: 30, window: time.Minute}, // presign upload publik
	"wil_pub": {max: 60, window: time.Minute}, // daftar wilayah (read-only ringan)
	"agt_pub": {max: 30, window: time.Minute}, // cek anggota publik (anti scraping NIA)
}

// RateLimit mengembalikan limiter per-IP sesuai nama kebijakan terdaftar.
func RateLimit(rdb *redis.Client, name string) fiber.Handler {
	p, ok := ratePolicies[name]
	if !ok {
		panic("RateLimit: kebijakan rate limit tak dikenal: " + name)
	}
	return AuthRateLimiter(rdb, "rl:"+name+":ip:", p.max, p.window)
}
