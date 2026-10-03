# S5 — Audit File, SSRF & Pihak Ketiga

> Tanggal: 02 Oktober 2026 · Target: backend Go dev `:8081` · Sifat probe: non-destruktif
> Acuan OWASP: File Upload, Server Side Request Forgery Prevention

---

## 1. Cakupan

Presign-upload (policy, key), verifikasi dokumen (HeadObject, magic bytes,
PDF terkunci), SMTP (MIME + TLS), Fonnte, S3 client, proxy upstream
(wilayah.id, carikodepos.id).

## 2. Metode

Review service/gateway + 6 probe live upload + unit test + verifikasi ulang
proxy wilayah/kodepos (sudah dikeraskan di Batch 1–3 wilayah).

## 3. Temuan

| # | Severity | Temuan | Bukti | Status |
|---|---|---|---|---|
| S5-01 | Low | `To:` email dimasukkan mentah ke header MIME (tanpa tolak CRLF di `Send`; saat ini aman karena semua pemanggil memakai email tervalidasi) | Review `BuildMIMEMessage`/`Send` | ✅ **Diperbaiki**: tolak `\r\n` + unit test |
| S5-02 | Info | Upload aman berlapis: key server-generated (nama client diabaikan), MIME allowlist + ekstensi dari MIME (bukan nama file), `content-length-range` keras di storage | Probe: kategori/MIME/ukuran → 422; `ktp.jpg.php` → key `.jpg`; traversal filename → key aman | ✅ Lolos |
| S5-03 | Info | Verifikasi submit: pola key allowlist, HeadObject, batas ukuran, content-type allowlist, magic bytes 512B, tolak PDF terkunci | Review + unit test (`key traversal`, PDF lock) | ✅ Lolos |
| S5-04 | Info | SMTP: subjek/from-name di-encode RFC 2047, TLS (implicit 465 / STARTTLS), timeout 10+15 dtk, PlainAuth aman | Review + unit test MIME | ✅ Lolos |
| S5-05 | Info | Fonnte: token wajib, normalisasi target, cek status HTTP + boolean body (200-palsu ditolak), timeout 10 dtk | Review + unit test httptest | ✅ Lolos |
| S5-06 | Info | Proxy upstream (wilayah/kodepos): allowlist input, host konstanta, tanpa redirect, timeout+cap, cache, fail-open, rate-limit | Batch wilayah 1–3 + smoke ulang | ✅ Lolos |

Tidak ada temuan Critical/High/Medium.

## 4. Verifikasi

`go build/vet/gofmt` bersih; `go test ./...` semua OK; probe ulang hijau.
