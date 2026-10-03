# S4 — Audit Kripto & Rahasia

> Tanggal: 02 Oktober 2026 · Target: backend Go dev `:8081` + review kode
> Acuan OWASP: Cryptographic Storage, Key Management, Secrets Management

---

## 1. Cakupan

Argon2id, AES-256-GCM (NIK), HMAC blind-index, HMAC QR-KTA, `crypto/rand`
token, JWT secret, presigned URL, penyimpanan/rotasi kunci, `.env`/git,
dependensi kripto.

## 2. Metode

Review `pkg/crypto`, `config` (validasi kunci), auth/OTP/reset flows +
probe live (reveal NIK, presign-view, KTA verify, traversal key).

## 3. Temuan

| # | Severity | Temuan | Bukti | Status |
|---|---|---|---|---|
| S4-01 | Info | Password: Argon2id 64 MB/3 iterasi/2 paralel + batas 128 char (S1) | Review + test S1 | ✅ Lolos |
| S4-02 | Info | NIK: AES-256-GCM nonce acak per enkripsi (format `nonce:cipher`), blind-index HMAC-SHA256 terpisah, kunci 64-hex unik & wajib beda | Review `crypto.go` + probe RevealNIK → **200, 16 digit** (roundtrip + audit) | ✅ Lolos |
| S4-03 | Info | Token (OTP/reset/refresh): `crypto/rand`, 256-bit, hash SHA-256 di penyimpanan, TTL + sekali pakai | Review + S1 | ✅ Lolos |
| S4-04 | Info | QR-KTA: HMAC-SHA256 `nia:tanggal:id`, verify constant-time + verdict seragam; ada kunci rotasi verify-only | Probe: sig palsu → **valid=false** (bukan error/oracle) + unit test | ✅ Lolos |
| S4-05 | Info | Presign: TTL 5 mnt + signature di URL; key traversal → 422; tanpa auth → 401; BOLA scope + audit | Probe P2 (200/422/401) | ✅ Lolos |
| S4-06 | Info | Kunci: fail-fast (64-hex, unik antar-kunci, secret ≥32, TTL ≤30 mnt); `.env` gitignored + tak pernah ter-commit; `.env.example` placeholder | Review config + `git log -- .env` kosong + `check-ignore` | ✅ Lolos |

Tidak ada temuan Critical/High/Medium. Tidak ada perubahan kode pada Batch S4.

## 4. Verifikasi

`go build/vet/gofmt` bersih; `go test ./...` semua OK; probe ulang hijau.
