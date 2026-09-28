# Laporan PR — Fase 2: Core Membership Engine

> **Branch**: `backend-fase-2-membership-pendaftaran` → `main`
> **Status**: DRAFT — belum push, belum merge. Menunggu sisa task §8.
> **Tanggal laporan**: 28 September 2026
> **Cakupan**: seluruh siklus calon anggota → kader ber-NIA + verifikasi KTA,
> plus storage presigned S3. **Di luar cakupan**: SK/pengurus (Fase 3),
> CMS/Laravel (Fase 4), notifikasi (Fase 5), deploy (Fase 6).

---

## 1. Ringkasan isi PR

| Kelompok | Isi | Commit |
|---|---|---|
| Fondasi (dari merge Fase 1) | Hardening M-1 s/d M-5: logout publik, TTL ≤ 30m, seeder tanpa password di log, `activity_logs` append-only + writer, `VerifyKTASignature` | `18bfd61`, `8efcd7b`, `a60742c`, `c15ac0c`, `5d9c46d`, `160c314`, `b204813` |
| Batch 1 | Submit pendaftaran real: validasi kuat, cek duplikat NIK 2 tabel (409), enkripsi AES-GCM, sequence REG dari DB (migrasi 000006) | `cd0c3a5` |
| Batch 2 | Otorisasi: `ActorContext` + jurisdiction, `RequireRoles`, tracking publik minimal, UpdateStatus atomik + audit, limiter membership | `cd0c3a5` |
| Batch 3 | NIA format tunggal + `kta_qr_hash` saat terbit + endpoint verifikasi KTA asli + rotasi kunci (`KTA_SIGNING_KEY_PREV`) | `cd0c3a5` |
| H-1 | Storage presigned: `pkg/storage`, `storage_service`, endpoint presign PUT/GET, verifikasi dokumen saat submit | `91da13e` |
| Batch 4 | Revisi bertoken, ListQueue real + scope, validasi master wilayah, migrasi 000007 (CHECK + RESTRICT) | `1ff2ce1` |
| Batch 5 | Test AES/race/integration, pentest suite v2.4, fix limiter prefix | `51be3e7` |

Stat vs `main`: **57 file, +5581 / −406** (termasuk warisan Fase 1 dan hapus artefak `results.json/sarif`).

---

## 2. Endpoint baru (kontrak `/api/v1`)

### Publik (rate-limit 30/mnt, tanpa auth)

| Method & Path | Fungsi | Status |
|---|---|---|
| `POST /pendaftaran` | Submit pendaftaran → `201 {id, nomor REG-YYYYMM-XXXXX, DIAJUKAN}`; duplikat NIK → `409` | ✅ live OK |
| `GET /pendaftaran/track/:nomor` | DTO minimal (nomor, status, timestamp) — tanpa PII/key | ✅ live OK |
| `POST /pendaftaran/revisi/request-token` | Terbitkan token revisi (status harus PERBAIKAN, 24 jam, single-use) | ✅ live OK |
| `PUT /pendaftaran/revisi/:nomor` | Submit revisi bertoken → kembali DIAJUKAN; token salah → 422 generik | ✅ live OK |
| `GET /pendaftaran/kta/:nia?sig=` | Verdict HMAC + lookup DB; selalu 200 (`valid:true/false`, tanpa oracle) | ✅ live OK |
| `POST /storage/presign-upload` | Tiket PUT langsung (key server-generated, MIME di-sign) | ✅ live OK |

### Admin (auth + `RequireRoles` 4 role + limiter 20/mnt)

| Method & Path | Fungsi | Status |
|---|---|---|
| `GET /admin/pendaftaran?page&limit&status` | Antrean terfilter jurisdiction + proyeksi non-PII + meta | ✅ live OK |
| `GET /admin/pendaftaran/:id` | Detail; lintas wilayah → 403 | ✅ live OK |
| `POST /admin/pendaftaran/:id/verifikasi` | DIAJUKAN → DIVERIFIKASI | ✅ live OK |
| `POST /admin/pendaftaran/:id/perbaikan` | → PERBAIKAN + catatan | ✅ live OK |
| `POST /admin/pendaftaran/:id/tolak` | → DITOLAK + alasan | ✅ live OK |
| `POST /admin/pendaftaran/:id/setujui` | → DISETUJUI + terbit anggota/NIA/QR dalam 1 tx | ✅ live OK |
| `GET /storage/presign-view?key=` | Tiket baca 5 mnt + audit VIEW | ✅ live OK |

State machine ditegakkan di service (aksi enum, bukan `PATCH status`).

---

## 3. Migrasi database (000001 → 000007, berurutan, teruji up)

| # | File | Isi |
|---|---|---|
| 5 | `000005_activity_logs` | Tabel audit + trigger tolak UPDATE/DELETE (append-only) |
| 6 | `000006_registration_sequence` | Tabel sequence nomor REG per-bulan (UPSERT atomik) |
| 7 | `000007_membership_constraints` | CHECK status/enum/format NIA; riwayat + link anggota RESTRICT; index token revisi |

000001–000004 warisan Fase 1 (skema auth, seed tanpa kredensial, tabel membership, hardening). Down migration tersedia untuk 000005–000007.

---

## 4. Bukti keamanan (hasil verifikasi, bukan klaim)

- **Gates**: `go build`, `go vet`, `go test -race`, `gosec` (0 issues, 32 file/6688 baris), `govulncheck` (0 vuln di kode) — semua hijau.
- **Unit**: 39 test fungsi lolos; integration repo 5/5 lolos dengan `-race` (duplikat 409, sequence naik, 10 goroutine unik, filter wilayah, alur token). Integration SKIP otomatis tanpa `TEST_DATABASE_URL`.
- **Pentest suite v2.4**: **36/36 PASS, 0 FAIL** (1 SKIP: SEED-01 butuh password lama yang tak disimpan di repo). Termasuk MEM-01..06 dan regresi M-1/M-2.
- **Live E2E vs MinIO AGPL lokal**: presign → PUT biner → submit 201 (HeadObject + magic bytes lolos) → siklus verifikasi → revisi bertoken → setujui → NIA strip + QR 64-hex → KTA valid/tamper/asing. Key fiktif → 422. Cross-wilayah → 403.
- **Bug real yang tertangkap & diperbaiki dalam pengerjaan**: sequence selalu-1, filter wilayah dibuang, KTA `"valid"` palsu, NIK plaintext path, limiter berbagi kuota Redis (429 prematur), kolom NULL gagal scan, dokumen opsional jadi wajib saat revisi.

Alur otorisasi: JWT → `RequireRoles` → `ScopeWilayah` → `ActorContext.CanAccessWilayah` di service → query terfilter. Scope yang dipasang selalu dikonsumsi (tak ada middleware mati).

---

## 5. Cara verifikasi untuk reviewer

```powershell
# 1. Infra (Postgres 5432 + Redis 6379 wajib; MinIO :9000 untuk E2E penuh)
# 2. Backend
cd backend
Copy-Item .env.example .env   # isi 4 kunci (openssl rand -hex 32) + STORAGE_* MinIO
go run -tags 'postgres file' github.com/golang-migrate/migrate/v4/cmd/migrate `
  -path migrations -database $env:DATABASE_URL up     # → versi 7
$env:SEED_ADMIN_PASSWORD='<kuat>'; go run ./cmd/seed
go run ./cmd/api                                       # → :8080/health 200

# 3. Gates + test
go build ./...; go vet ./...
go test ./... -race -count=1
$env:TEST_DATABASE_URL=$env:DATABASE_URL; go test ./internal/repository/ -race -count=1 -v
gosec ./...; govulncheck ./...

# 4. Pentest (server harus jalan)
$env:PENTEST_ADMIN_PASSWORD='<sama>'; ./scripts/pentest_suite.ps1  # → 36/36, gate PASSED
```

Catatan: `make migrate-up` rusak (CLI tanpa build tag postgres → `unknown driver`) — pakai perintah `go run -tags` di atas sampai Makefile diperbaiki.

---

## 6. Yang SENGAJA belum dikerjakan (keputusan sadar)

- **Pengiriman token revisi via WA/email** — endpoint mengembalikan token di respons (rate-limited). Notifikasi Fase 5.
- **Render PDF KTA** (`kta_pdf_key` belum diisi) — QR verify jalan; artefak kartu Fase 2 akhir / Fase 3 awal.
- **Turnstile/CAPTCHA** di 4 endpoint publik — butuh frontend + domain; kriteria sudah ditetapkan.
- **Dekripsi NIK untuk verifikator** (Task 1.4.2) — struktur siap (`nik_encrypted` + `MaskNIK`), endpoint detail khusus + re-autentikasi belum dibangun.
- **Cross-wilayah 403 di suite** — verified live manual, belum di suite (butuh seed 2 wilayah).

---

## 7. Risiko & catatan

- **MinIO**: binary Sep 2026 berlisensi AIStor menonaktifkan S3 tanpa lisensi — dev memakai AGPL Des 2024 (checksum cocok). Jangan upgrade tanpa cek lisensi. R2 sempat dicoba, terkendala; MinIO final untuk dev.
- **`.env` lokal** berisi kredensial dev + kunci — gitignored, tidak ikut PR. Jangan pernah commit.
- **`session-ses_f200.md`** (168KB, dump transkrip) masih untracked di root — JANGAN ikut commit/push.
- **Data residency**: dev boleh di MinIO lokal/R2; PII produksi wajib IS3 Indonesia (UU PDP) — Fase 6.
- **DB dev lokal** saat ini di versi 7, bersih dari baris uji (`@example.com` dihapus tiap E2E).

---

## 8. Sisa yang perlu dilakukan SEBELUM PR siap merge

### A. LOW hardening warisan Fase 0/1A (kecil-kecil)

- [ ] L-1 Postgres fail-open → fail-closed di production (`postgres.go:43-49`)
- [ ] L-2 `ScopeWilayah` tanpa claims → 401, bukan `Next()` (`rbac.go:64-69`)
- [ ] L-3 Layering: health tanpa DB langsung; `JWTClaims` pindah ke domain; dedup parse Bearer
- [ ] L-4 Ganti password blacklist access token aktif
- [ ] L-5 Validasi config prod (`DB_SSLMODE=require`, `APP_DEBUG=false`, `REDIS_PASSWORD`, tolak `ALLOW_ORIGIN="*"`)
- [ ] L-6 Cookie Secure/SameSite configurable + dokumentasi
- [ ] L-7 Logger request-id + sempitkan trusted proxy
- [ ] L-8 Betulkan komentar format AES (`crypto.go:29`)

### B. Sisa temuan Fase 2

- [ ] `mapDBError` di `UpdateStatus*`/`SetRevisiToken` (409/422, bukan 500)
- [ ] Pesan "Repository … belum tersedia" → generik + log server (13 titik)
- [ ] Proyeksi kolom eksplisit di `GetByID`/`GetByNomor` (NIK encrypted tak transit jalur publik)
- [ ] Task 1.4.2: endpoint detail + dekripsi NIK teraudit untuk verifikator
- [ ] Render PDF KTA server-side (`kta_pdf_key`)

### C. Non-kode (di fasenya masing-masing)

- [ ] Turnstile + pengiriman token via WA/email (butuh frontend/domain/notifikasi)
- [ ] Cross-wilayah 403 masuk suite (butuh seed 2 wilayah)
- [ ] Sinkronkan checklist `docs/TAHAPAN_PENGERJAAN_DAN_CHECKLIST_FITUR.md` (masih centang kosong + penomoran lama)
- [ ] Perbaiki `make migrate-up` (tambah `-tags 'postgres file'`)
- [ ] Hapus branch `fix/fase-1-security-hardening` setelah tak dipakai

### Definisi siap-merge

- [ ] Seluruh A + B selesai, gates hijau, suite tetap 36/36 (atau lebih bila tambah test)
- [ ] Reviewer menjalankan §5 dan mereproduksi gate PASSED
- [ ] Tidak ada file stray (`session-*.md`, binary `tmp_*`, `.env`) di diff
