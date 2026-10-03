# S6 — Audit Transport, DoS & Logging

> Tanggal: 02 Oktober 2026 · Target: backend Go dev `:8081` · Sifat probe: non-destruktif
> Acuan OWASP: REST Security, HTTP Headers, Denial of Service, Logging,
> Error Handling, User Privacy Protection

---

## 1. Cakupan

CORS + security headers, rate-limit semua grup, bentuk error, PII di log,
kelengkapan audit, anti-enumerasi publik.

## 2. Metode

Review middleware/routes/response + probe live (header, CORS preflight,
404, rate-limit track).

## 3. Temuan

| # | Severity | Temuan | Bukti | Status |
|---|---|---|---|---|
| S6-01 | Info | Header keamanan lengkap (nosniff, DENY, CSP ketat, referrer, permissions, COOP/CORP) | Probe H1 | ✅ Lolos |
| S6-02 | Info | CORS: origin asing tanpa ACAO; origin terdaftar dapat ACAO; wildcard ditolak di prod | Probe H2 + `validateProduction` | ✅ Lolos |
| S6-03 | Info | Rate-limit bekerja (track per-nomor 429 muncul; registry terpusat anti-bagi-kuota) | Probe H4 (429×3) + `ratePolicies` | ✅ Lolos |
| S6-04 | Info | Error generik + trace_id, tanpa stack/PII; unknown → 500 generik; 404 rute tak dikenal | Probe H3 + `FromError` | ✅ Lolos |
| S6-05 | Info | Tanpa PII mentah di log terstruktur (tanpa `.Str(email/nik/…)`); request log tanpa body/header/query (NIA di query tak tercatat karena hanya `Path()`); masking tersedia | Review logger + gateway mask | ✅ Lolos |
| S6-06 | Info | Audit mencakup login/refresh/logout/ganti/reset, revisi, submit, approval, NIK reveal, KTA unduh, reset anggota | Review 25 call-site `auditEvent`/`writeAudit` | ✅ Lolos |
| S6-07 | Info | Endpoint publik minimal + anti-enumerasi (track DTO tanpa PII, cek NIA-only, OTP seragam — lihat S1) | Review + probe S1 | ✅ Lolos |

Tidak ada temuan Critical/High/Medium. Tidak ada perubahan kode pada Batch S6.

## 4. Verifikasi

`go build/vet/gofmt` bersih; `go test ./...` semua OK; probe ulang hijau.
