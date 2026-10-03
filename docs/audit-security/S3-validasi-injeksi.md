# S3 — Audit Validasi & Injeksi

> Tanggal: 02 Oktober 2026 · Target: backend Go dev `:8081` · Sifat probe: non-destruktif
> Acuan OWASP: Input Validation, Injection Prevention, SQL Injection
> Prevention, Query Parameterization, Database Security

---

## 1. Cakupan

Validator (`pkg/validator` + aturan per field di service), NIK (format +
segmen tanggal), kunci dokumen (allowlist traversal), search/pagination,
seluruh SQL repository (placeholder vs konkatenasi), keamanan migrasi.

## 2. Metode

Review semua query repository + aturan validasi + 13 probe live injeksi
(SQLi, traversal, XSS, pagination ekstrem, JSON rusak, NIK panjang).

## 3. Temuan

| # | Severity | Temuan | Bukti | Status |
|---|---|---|---|---|
| S3-01 | Info | Seluruh SQL memakai placeholder (`$n`); `ORDER BY` kolom hardcode; `LIMIT/OFFSET` via placeholder int; `ILIKE` pakai arg terikat | Review 4 repository + `mapDBError` (constraint → 409 ramah, tanpa bocor) | ✅ Lolos, tanpa perubahan |
| S3-02 | Info | SQLi di search → 200 + 0 hasil (terparameterisasi, tanpa error/dump) | Probe I1 (`' OR '1'='1`, `UNION`) | ✅ Lolos |
| S3-03 | Info | Path traversal & format nomor dijaga validator format | Probe I2 (`..%2F`, kutip) → 422 | ✅ Lolos |
| S3-04 | Info | Pagination negatif/raksasa di-clamp (tidak crash/bocor) | Probe I3 `page=-5&limit=99999` → 200 | ✅ Lolos |
| S3-05 | Info | ID wilayah & kode divalidasi ketat (400/404, bukan 500) | Probe I4 (abc, 99999, SQLi, kode aneh) | ✅ Lolos |
| S3-06 | Info | NIK panjang & XSS di endpoint publik ditolak validator | Probe I5 (200 digit, `<script>`) → 422 | ✅ Lolos |
| S3-07 | Info | JSON rusak → 400 (bukan 500/stack) | Probe I6 | ✅ Lolos |
| S3-08 | Info | Validasi field selaras kolom DB (panjang rune + tolak `<>` + NIK plausibel + umur + kunci allowlist + anti key ganda) | Review `ValidateSubmitRequest` + `checkObjectKey`/`checkDuplicateDocKeys` + unit test XSS/panjang | ✅ Lolos |

Tidak ada temuan Critical/High/Medium. Tidak ada perubahan kode pada Batch S3.
Catatan: karakter wildcard LIKE (`%`, `_`) di `search` hanya memperluas hasil
dalam scope admin ybs (fungsi setara list kosong) — risiko Info, tanpa perbaikan.

## 4. Verifikasi

`go build/vet/gofmt` bersih; `go test ./...` semua OK; probe ulang hijau.
