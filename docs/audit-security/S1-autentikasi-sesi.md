# S1 — Audit Autentikasi & Sesi

> Tanggal: 02 Oktober 2026 · Target: backend Go dev `:8081` · Sifat probe: non-destruktif
> Acuan OWASP: Authentication, Session Management, JSON Web Token, Password
> Storage, Forgot Password, Credential Stuffing Prevention, Bot Management

---

## 1. Cakupan

File yang diaudit: `internal/service/auth_service.go`,
`internal/handler/auth_handler.go`, `internal/handler/password_reset_handler.go`,
`internal/handler/otp_handler.go`, `internal/service/password_reset_service.go`,
`internal/service/otp_service.go`, `internal/middleware/auth.go`,
`internal/middleware/security.go` (limiter), `config/config.go` (auth + fail-fast).

Endpoint yang diprobe live: `POST /auth/login`, `POST /auth/refresh`,
`POST /auth/logout`, `GET /auth/me`, `PUT /auth/password`,
`POST /auth/forgot-password`, `POST /auth/reset-password`,
`POST /pendaftaran/otp/whatsapp/request|verify`.

## 2. Metode

Review checklist OWASP per file + `go test/vet/gofmt` + probe live non-destruktif
(status/message/rate-limit; tanpa tulis data: password salah, email fiktif,
nomor uji, token sampah).

## 3. Temuan

| # | Severity | Temuan | Bukti | Status |
|---|---|---|---|---|
| S1-01 | Medium | Password login & lama **tanpa batas panjang** → input argon2id tak terbatas (DoS CPU), inkonsisten dengan kebijakan `password_strength` (maks 128) | `auth_service.go` (`validate:"required"` tanpa `max`) | ✅ **Diperbaiki**: `max=128` + 2 unit test |
| S1-02 | Info | Anti-enumerasi login/forgot/OTP seragam | Probe: login unknown vs salah → **401 + pesan identik**; forgot unknown vs known → **200 identik**; OTP verify → pesan seragam; `argon2id.CreateHash` dummy + status-check setelah verifikasi | ✅ Lolos, tanpa perubahan |
| S1-03 | Info | Refresh rotation aman (reuse detection + atomic CAS + cabut family; status nonaktif ditolak) | Review `RefreshToken`/`rotateSession` + `ErrTokenAlreadyRotated` | ✅ Lolos |
| S1-04 | Info | JWT & cookie benar (HS256 enforced, jti wajib + blacklist Redis, TTL 15 mnt ≤ 30; HttpOnly, Secure dari env, SameSite Strict, Path `/api/v1/auth`; refresh hanya via cookie) | Review + probe `alg=none`/rusak → 401; refresh tanpa/sampah cookie → 401 | ✅ Lolos |
| S1-05 | Info | Logout idempoten + best-effort (tanpa oracle); ganti password verifikasi lama + cabut semua sesi + blacklist token aktif | Review + probe logout kosong → 200; ganti tanpa auth → 401 | ✅ Lolos |
| S1-06 | Info | Reset via email aman (token 256-bit, hash di Redis, TTL 30 mnt, sekali pakai Lua atomik, cabut sesi, tolak sama-dengan-lama) | Review `password_reset_service.go` | ✅ Lolos |
| S1-07 | Info | OTP WA ketat (cooldown 60 dtk, 5/jam, 5x coba lalu hangus, compare constant-time, hash, pesan seragam; `debug_code` dev-only + prod fail-fast `fonnte`) | Probe: req1 200 → req2 **429**; verify salah **422** seragam | ✅ Lolos |
| S1-08 | Info | Rate-limit berlapis benar (login 20/mnt IP + 20/15mnt email+IP anti-lockout; lupa 3+5; OTP 20/mnt + per-nomor) | Probe: 21 login cepat → **401×12 + 429×9** | ✅ Lolos |
| S1-09 | Info | Config fail-fast kuat (secret ≥32, TTL ≤30 mnt, SameSite allowlist, tolak wildcard CORS/proxy, DB TLS, kunci 64-hex unik, prod wajib fonnte/SMTP/URL) | Review `validateCryptoKeys`/`validateAuth`/`validateProduction` | ✅ Lolos |

Hasil probe ringkas: `P1 401/401 sama`, `P2 200/200 sama`, `P3 200→429→422`,
`P4 401/401/200`, `P5 401/401/401`, `P6 429 muncul`, `overlong-password → 422`.
Catatan: baseline env ±2 dtk per request (termasuk `/health`), jadi delta
timing bukan sinyal — validasi via status + unit test, bukan selisih waktu.

## 4. Perbaikan yang diterapkan (Batch S1)

1. `LoginRequest.Password` + `ChangePasswordRequest.OldPassword`: `validate:"required,max=128"`.
2. `internal/service/auth_validation_test.go` (baru): tolak >128, terima =128.
3. Verifikasi live: password 200+ char → **422** (bukan hang/500).

## 5. Verifikasi

`go build/vet/gofmt` bersih; `go test ./...` semua OK; probe ulang hijau.

## 6. Sisa risiko / tindak lanjut

- Entropi OTP 6-digit (~20 bit) diterima OWASP karena TTL 5 mnt + 5x coba + cooldown + per-nomor — pantau bila pola abuse berubah (opsi: naikkan ke 8 digit / tambah CAPTCHA, ditunda ke Fase 5/6 sesuai komentar kode).
- Sesi multi-perangkat (multi-family) by-design; pencabutan global hanya saat ganti password/reset/logout.
- Tidak ada temuan Critical/High pada Batch S1.
