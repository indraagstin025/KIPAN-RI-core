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
	"kep_mut": {max: 30, window: time.Minute}, // mutasi admin kepengurusan (SK/pengurus/jabatan)
	"stor_up": {max: 30, window: time.Minute}, // presign upload publik
	// otp_wa longgar per-IP (20/mnt): gerbang sebenarnya adalah batas
	// per-nomor di service (cooldown 60 dtk + 5/jam + 5x coba). Per-IP
	// ketat berisiko memblokir pengguna sah di belakang NAT yang sama.
	"otp_wa":   {max: 20, window: time.Minute},     // minta/verifikasi OTP WA
	"pw_reset": {max: 5, window: 15 * time.Minute}, // lupa kata sandi per IP
	"wil_pub":  {max: 60, window: time.Minute},     // daftar wilayah (read-only ringan)
	"wil_mut":  {max: 30, window: time.Minute},     // mutasi master wilayah admin (status/tambah)
	"usr_mut":  {max: 30, window: time.Minute},     // mutasi manajemen pengguna (Super Admin)
	"out_mut":  {max: 30, window: time.Minute},     // mutasi antrian email (retry/kirim ulang)
	"agt_pub":  {max: 30, window: time.Minute},     // cek anggota publik (anti scraping NIA)
}

// RateLimit mengembalikan limiter per-IP sesuai nama kebijakan terdaftar.
func RateLimit(rdb *redis.Client, name string) fiber.Handler {
	p, ok := ratePolicies[name]
	if !ok {
		panic("RateLimit: kebijakan rate limit tak dikenal: " + name)
	}
	return AuthRateLimiter(rdb, "rl:"+name+":ip:", p.max, p.window)
}

// MutatingRateLimit menerapkan limiter HANYA untuk metode yang mengubah data
// (POST/PUT/PATCH/DELETE). GET/HEAD/OPTIONS lolos tanpa kuota. Dipakai pada
// grup admin agar navigasi menu (GET) tidak ikut terkena 429.
func MutatingRateLimit(rdb *redis.Client, name string) fiber.Handler {
	limiter := RateLimit(rdb, name)
	return func(c *fiber.Ctx) error {
		switch c.Method() {
		case fiber.MethodGet, fiber.MethodHead, fiber.MethodOptions:
			return c.Next()
		}
		return limiter(c)
	}
}
