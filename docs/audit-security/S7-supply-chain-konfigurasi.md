# S7 — Audit Supply Chain & Konfigurasi (+ Konsolidasi Akhir)

> Tanggal: 02 Oktober 2026 · Sifat: review + `govulncheck`
> Acuan OWASP: Vulnerable Dependency Management, Secrets Management,
> Database Security, Docker Security (bagian dev)

---

## 1. Cakupan

`go.mod`/`go.sum` (CVE via `govulncheck`), `vendor/`, Dockerfile, CI,
secret di repo/docker, least-privilege DB.

## 2. Metode

`govulncheck ./...` + inspeksi compose/docker/CI + `git log` secret.

## 3. Temuan

| # | Severity | Temuan | Bukti | Status |
|---|---|---|---|---|
| S7-01 | Info | `govulncheck`: **0 vuln menyentuh kode**; 2 tak-terjangkau (`GO-2026-4950` fasthttp, `GO-2026-5932` x/crypto, keduanya indirect) | Output scan | ✅ Lolos; tindak lanjut: `go get -u` + scan berkala |
| S7-02 | Low | Password dev plaintext di `docker-compose.dev.yml` (ter-commit; jelas `_dev_` + localhost, prod fail-fast menolak lokal/kosong) | Review compose | ⚠️ Dilaporkan; remediasi: secret staging/prod via vault/env, jangan reuse kredensial dev |
| S7-03 | Info | Redis dev tanpa password; Postgres dev superuser; tanpa `Dockerfile` produksi; tanpa CI (`.github` nihil) | Review compose/repo | ⚠️ Rekomendasi: role DB least-privilege utk runtime prod; Dockerfile multi-stage non-root; CI (`go test` + `govulncheck` + secret scan) |
| S7-04 | Info | `.env` gitignored + tak pernah ter-commit; `.env.example` placeholder | `check-ignore` + `git log -- .env` kosong | ✅ Lolos |

Tidak ada temuan Critical/High/Medium. Tidak ada perubahan kode pada Batch S7.

## 4. Verifikasi

`go build/vet/gofmt` bersih; `go test ./...` semua OK.

---

## 5. Konsolidasi Akhir Audit S1–S7

| Batch | Dokumen | Critical/High/Medium | Perbaikan kode |
|---|---|---|---|
| S1 | `S1-autentikasi-sesi.md` | 1 Medium | `max=128` password + 2 test |
| S2 | `S2-otorisasi.md` | 0 | — (catatan URD ditunda) |
| S3 | `S3-validasi-injeksi.md` | 0 | — |
| S4 | `S4-kripto-rahasia.md` | 0 | — |
| S5 | `S5-file-ssrf-pihak-ketiga.md` | 1 Low | tolak CRLF `To:` + test |
| S6 | `S6-transport-dos-logging.md` | 0 | — |
| S7 | (dokumen ini) | 1 Low (proses) | — (rekomendasi) |

**Total: 0 Critical, 0 High, 1 Medium + 2 Low (semua sudah ditangani/dokumentasi).**
Sisa terencana: pengetatan aksi-Provinsi (track URD), CAPTCHA/notifikasi login (Fase 5/6 per komentar kode), ClamAV pra-approval (Fase 6), endpoint `/admin/users` (keputusan A/B), CI + Dockerfile prod.
