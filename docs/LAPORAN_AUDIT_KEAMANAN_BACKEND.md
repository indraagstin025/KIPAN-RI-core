# Laporan Audit Keamanan & Kualitas — Backend SIM-KIPAN Core (Go)

| Item | Keterangan |
|---|---|
| Tanggal audit | 28 September 2026 |
| Metode | Analisis statis kode + verifikasi tool keamanan live + verifikasi klaim laporan PR Fase 1 & Fase 2 |
| Target | `backend/` (Go 1.27.1, Fiber v2, PostgreSQL 16, Redis 7) |
| Branch | `backend-fase-2-membership-pendaftaran` |
| Commit HEAD | `51be3e7` — test(security): batch 5 AES/race/integration + pentest v2.4 + limiter prefix |
| Status branch | ahead 13 commit dari origin, belum di-push |
| Cakupan | `config/`, `cmd/`, `internal/`, `pkg/`, `migrations/`, `scripts/`, `Makefile` |
| Di luar cakupan | `KIPAN_INDONESIA/` (aplikasi lama Next.js — diputuskan TIDAK dipakai lagi, di-ignore Git), frontend baru (belum ada), infrastruktur produksi (Fase 6) |

---

## 1. Ringkasan Eksekutif

Backend Go ini berada **jauh di atas rata-rata** untuk proyek tahap Fase 2. Kontrol kriptografi benar (AES-256-GCM nonce acak + blind index HMAC-SHA256 + Argon2id 64 MB), manajemen sesi ketat (JWT HS256 ber-`jti`, rotasi refresh token dengan compare-and-swap, deteksi reuse yang mencabut satu family), SQL 100% terparameter, fail-closed untuk Redis & storage di production, audit trail append-only yang dipaksa trigger database, dan seeder tanpa kredensial hardcode.

Namun masih ada **satu temuan KRITIS yang belum tercatat** di laporan PR Fase 1 maupun Fase 2: alur revisi pendaftaran dapat dibajak karena token revisi diterbitkan **hanya bermodal nomor pendaftaran**, sementara nomor pendaftaran bersifat sekuensial dan statusnya dapat ditelusuri publik. Temuan ini wajib ditutup sebelum produksi.

### 1.1 Matriks temuan

| ID | Temuan | Severity | Status (v1.1) | Lokasi utama |
|---|---|---|---|---|
| KIPAN-BE-001 | Pembajakan pendaftaran lewat `POST /pendaftaran/revisi/request-token` tanpa verifikasi pemilik | KRITIS | ✅ DITUTUP (`aaa9f70`; REV-01..04) | `internal/handler/routes.go:165`, `internal/service/pendaftaran_service.go:689-716` |
| KIPAN-BE-002 | Presign upload publik tanpa penegakan ukuran/konten (abuse storage) | SEDANG | ✅ DITUTUP realistis (`0f7f67e`; fix #1 mustahil, lihat catatan) | `internal/service/storage_service.go:110-152`, `pkg/storage/s3.go:80-93` |
| KIPAN-BE-003 | Limiter login per-email dapat dipakai melakukan lockout akun (DoS) | SEDANG | ✅ DITUTUP (`0f7f67e`; key email+IP 20/15 mnt) | `internal/middleware/security.go:94-118`, `internal/handler/routes.go:121` |
| KIPAN-BE-004 | Endpoint `/health` publik membocorkan env dan status dependensi | RENDAH | ✅ DITUTUP (`487fc8b`; HEALTH-01) | `internal/handler/health_handler.go:31-41`, `internal/handler/routes.go:36` |
| KIPAN-BE-005 | `SELECT *` pada `GetByID`/`GetByNomor`/`IssueMember` (memuat PII terenkripsi ke memori) | RENDAH | ✅ DITUTUP (`487fc8b`; kolom eksplisit) | `internal/repository/pendaftaran_repository.go:275, 287, 527` |
| KIPAN-BE-006 | Pesan internal `Repository ... belum tersedia` tampil di 9 titik kode produksi | RENDAH | ✅ DITUTUP (`487fc8b`; 503 generik, 13 titik) | `internal/service/pendaftaran_service.go:251, 287, 401, 421, 441, 497, 625, 695, 732` |
| KIPAN-BE-007 | `mapDBError` belum dipakai di `SetRevisiToken`/`SubmitRevisionTx`/`UpdateStatus*` (409/422 bisa jatuh jadi 500) | RENDAH | ✅ DITUTUP (`487fc8b`) | `pendaftaran_repository.go:380, 398, 447, 482` (dipakai di `:142, 234, 588`) |
| KIPAN-BE-008 | Sampah repo: `tmp_srv_*.exe` (5 file), `session-ses_f200.md` tidak di-ignore, `docs/pr-*` untracked | INFO | ✅ DITUTUP (`487fc8b` + `f38ecc8`) | `backend/`, `.gitignore` root |
| KIPAN-BE-009 | `make migrate-up` rusak (CLI migrate tanpa build tag `postgres`) | INFO | ✅ DITUTUP (`487fc8b`; tags + target version) | `Makefile:24-29` |
| KIPAN-BE-010 | Paket tanpa test: `cmd/*`, `config`, `internal/database`, `internal/handler`, `pkg/response`, `pkg/validator` | SEDANG (proses) | ⚠️ DITUTUP parsial (`487fc8b`; handler/config/validator ada; `cmd/*`, `database`, `response` belum) | hasil `go test ./...` |
| L-1 | PostgreSQL fail-open (ping gagal, server tetap jalan) | RENDAH | ✅ DITUTUP (`0f7f67e`; fail-closed production) | `internal/database/postgres.go:41-48` |
| L-2 | `ScopeWilayah` tanpa claims langsung `Next()` (fail-open bila lupa dipasang) | RENDAH | ✅ DITUTUP (`0f7f67e`; 401 + test) | `internal/middleware/rbac.go:62-69`, `rbac.go:27-33` |
| L-3 | Layering: health memegang `*sqlx.DB`, `JWTClaims` di middleware, parse Bearer duplikat | RENDAH | ✅ DITUTUP (`487fc8b`; JWTClaims→domain, helper tunggal) | `health_handler.go:16`, `middleware/auth.go:21-28`, `auth_handler.go:107-112` |
| L-4 | Ganti password tidak mem-blacklist access token yang masih aktif | RENDAH | ✅ DITUTUP (`487fc8b`; blacklist best-effort) | `internal/service/auth_service.go:281-318` vs `:211` |
| L-5 | Tidak ada validasi konfigurasi khusus production (`DB_SSLMODE`, `APP_DEBUG`, `REDIS_PASSWORD`, `ALLOW_ORIGIN=*`) | RENDAH | ✅ DITUTUP (`0f7f67e`; `validateProduction` + 6 test) | `config/config.go:206-222` |
| L-6 | Cookie `SameSite`/`Path` belum configurable | RENDAH | ✅ DITUTUP (`487fc8b`; configurable + validasi) | `internal/handler/auth_handler.go:181-190` |
| L-7 | Trusted proxy terlalu lebar di production (seluruh RFC1918) | RENDAH | ✅ DITUTUP (`487fc8b`; `APP_TRUSTED_PROXIES` eksplisit) | `cmd/api/main.go:153-166` |
| L-8 | Komentar format ciphertext AES tidak sesuai implementasi | INFO | ✅ DITUTUP (`487fc8b`) | `pkg/crypto/crypto.go:29` vs `:57` |

### 1.2 Kesimpulan kesiapan

| Pertanyaan | Jawaban |
|---|---|
| Aman dilanjutkan ke Fase 3 (SK/Pengurus)? | Ya — syarat KIPAN-BE-001 terpenuhi (`aaa9f70`, REV-01..04 hijau) |
| Aman untuk produksi sekarang? | Belum. Tersisa: render PDF KTA, kirim token via kanal terverifikasi (Fase 5), Turnstile (butuh frontend), & verifikasi produksi Fase 6 |
| Ada CRITICAL/HIGH lain selain KIPAN-BE-001? | Tidak ada |
| Tool keamanan hijau? | Ya: `gosec` 0 issue, `govulncheck` 0 vuln terpanggil, `go vet` bersih, `go test` semua paket PASS |

---

## 2. Metode & Bukti Verifikasi Live

Seluruh hasil berikut dijalankan pada mesin audit (Go 1.27.1, Windows, PowerShell 7, gosec dev, govulncheck v1.8.0, DB vuln per 24 September 2026).

| Perintah | Hasil aktual |
|---|---|
| `go vet ./...` | Bersih, tanpa diagnosa |
| `gosec -exclude-dir=bin ./...` | `Files: 32, Lines: 6688, Nosec: 0, Issues: 0` |
| `govulncheck -show verbose ./...` | `Your code is affected by 0 vulnerabilities` |
| `go test ./... -count=1` | Semua paket `ok`: domain, middleware, repository, service, crypto, generator, nia, storage |
| `git status --short` | Untracked: `docs/pr-fase1/`, `docs/pr-fase2/`, `session-ses_f200.md` |
| `git ls-files backend` (pola `.env`, `tmp_`, `session`, `results.`) | Hanya `backend/.env.example` — tidak ada `.env`, tidak ada biner ter-track |
| `git check-ignore -v KIPAN_INDONESIA session-ses_f200.md backend/.env` | `KIPAN_INDONESIA/` ignored, `backend/.env` ignored, `session-ses_f200.md` **TIDAK di-ignore** |

### 2.1 Yang TIDAK diverifikasi (keterbatasan laporan ini)

1. ~~**`scripts/pentest_suite.ps1` tidak dijalankan**~~ — **diperbarui v1.1**: suite **v2.5 dijalankan 41/41 PASS, 0 FAIL** (1 SKIP SEED-01, by design) pada branch hardening setelah remediasi, termasuk REV-01..04 dan HEALTH-01. Klaim 36/36 v2.4 tetap sah sebagai riwayat.
2. **Test integrasi repository** memerlukan `TEST_DATABASE_URL`; pada eksekusi audit, paket repository lolos namun jalur DB tidak diverifikasi ulang.
3. **Perilaku runtime di belakang Caddy/TLS produksi** (HSTS, trusted proxy, Vault) tidak dapat diuji dari dev — ranah Fase 6.
---

## 3. Kontrol Keamanan yang TERBUKTI BAIK (dengan bukti)

Bagian ini penting agar perbaikan berikutnya tidak merusak kontrol yang sudah benar.

| # | Kontrol | Bukti kode |
|---|---|---|
| 1 | AES-256-GCM dengan nonce acak per enkripsi | `pkg/crypto/crypto.go:46-57`; test `pkg/crypto/crypto_test.go:92-149` (roundtrip, nonce unik, tolak tamper & key salah) |
| 2 | Blind index HMAC-SHA256 untuk pencarian NIK tanpa dekripsi | `pkg/crypto/crypto.go:18-26`; kolom `nik_hash` bertipe UNIQUE (`internal/domain/schema.go:243`) |
| 3 | Argon2id 64 MB / 3 iterasi / 2 paralel (selaras OWASP) | `internal/service/auth_service.go:86-92`; idem di seeder `cmd/seed/main.go:81-87` |
| 4 | JWT HS256, `WithValidMethods`, wajib punya `jti` | `internal/middleware/auth.go:110-123` |
| 5 | Blacklist token per-`jti` (bukan raw token) di Redis | `internal/middleware/auth.go:95-106`; penulisan `auth_service.go:233-262` |
| 6 | Rotasi refresh token atomik + compare-and-swap (anti session cloning) | `internal/repository/user_repository.go:163-205` |
| 7 | Deteksi reuse token → cabut seluruh family + audit `TOKEN_REUSE` | `internal/service/auth_service.go:399-447` |
| 8 | Segel rotasi paralel memakai `ErrTokenAlreadyRotated` | `internal/domain/errors.go:25-31`; `auth_service.go:426-437` |
| 9 | Refresh token tidak pernah masuk JSON body | `internal/service/auth_service.go:36-43`; `internal/handler/auth_handler.go:75-76` |
| 10 | Cookie `HttpOnly` + `Secure` + `SameSite=Strict` + `Path` sempit | `internal/handler/auth_handler.go:176-191` |
| 11 | Flag `Secure` diturunkan dari `APP_ENV`, bukan header request | `internal/handler/routes.go:54-57` |
| 12 | Anti-enumerasi login: pesan seragam + ekualisasi waktu Argon2id | `internal/service/auth_service.go:107-126` |
| 13 | Rate limit dua lapis: 20/menit per IP dan 5/15 menit per email | `internal/handler/routes.go:117-123`; `internal/middleware/security.go:64-118` |
| 14 | Header keamanan lengkap (nosniff, DENY, CSP ketat, COOP/CORP) | `internal/middleware/security.go:21-56` |
| 15 | Redis fail-closed di production, degraded eksplisit hanya di dev | `internal/database/redis.go:31-59` |
| 16 | Storage & validasi wilayah fail-closed 503 di production | `internal/handler/routes.go:79-104`; `internal/service/pendaftaran_service.go:565-577, 581-603` |
| 17 | Object key server-generated + allowlist pola + anti traversal | `internal/service/storage_service.go:134-143`; `pkg/storage/s3.go:185-198` |
| 18 | Validasi isi file via magic bytes (bukan hanya MIME klien) | `pkg/storage/s3.go:204-228`; dipakai di `storage_service.go:197-203` |
| 19 | SQL terparameter penuh; klausa dinamis hanya menyusun placeholder `$n` | `internal/repository/pendaftaran_repository.go:309-351`; seluruh repo memakai `$1..$n` |
| 20 | Proyeksi kolom non-PII untuk antrean admin | `pendaftaran_repository.go:329` (`queueColumns`), `:342-368` |
| 21 | Otorisasi berbasis claims server-side (`ActorContext`), bukan body/query | `internal/handler/pendaftaran_handler.go:30-44`; dipakai `pendaftaran_service.go:427-431, 451-453` |
| 22 | Jurisdiction wilayah murni di domain (mudah diuji) | `internal/domain/schema.go:366-383`; test `service/pendaftaran_service_test.go:173` |
| 23 | State machine pendaftaran (aksi enum, bukan PATCH status) | `internal/domain/pendaftaran.go:52-68`; `pendaftaran_service.go:455-461` |
| 24 | Audit trail append-only dipaksa trigger DB | `migrations/000005_activity_logs.up.sql:38-49`; writer `internal/repository/audit_log_repository.go` |
| 25 | Duplikat NIK dicek 2 tabel → 409 (bukan 500) | `pendaftaran_repository.go:142, 234` via `mapDBError` |
| 26 | Seeder fail-closed per `APP_ENV` + password dari env/acak, bukan hardcode | `cmd/seed/main.go:47-51, 69-77, 112-124` |
| 27 | Akun seed lama dinonaktifkan + seluruh sesinya dicabut | `migrations/000004_harden_fase1_security.up.sql:30-56` |
| 28 | Master wilayah dilindungi FK RESTRICT | `migrations/000004:64-72`; `000007:46-65` |
| 29 | PII tidak bocor lewat JSON: `nik_hash` & `nik_encrypted` bertag `json:"-"` | `internal/domain/schema.go:188-189, 243-244` |
| 30 | Tidak ada secret hardcode di kode produksi | Audit grep `password\s*[:=]\s*"` hanya menemukan konstanta test (`pkg/crypto/kta_signature_test.go:6`) |

---

## 4. Verifikasi Klaim Laporan PR Fase 1 & Fase 2

Tabel ini menjawab pertanyaan: apakah yang dilaporkan benar-benar ada di kode?

| Klaim laporan | Hasil verifikasi | Bukti |
|---|---|---|
| Kriptografi (GCM, blind index, Argon2id, KTA HMAC) | BENAR | `pkg/crypto/crypto.go`; `auth_service.go:86-92`; `crypto.go:100-135` |
| JWT 15 menit, maksimum 30 menit (fail-fast bila lebih) | BENAR | `config/config.go:318-326` (menolak `> 30m`) |
| Rotasi + reuse detection + CAS anti-clone | BENAR | `user_repository.go:174-193`; `auth_service.go:426-437` |
| RBAC 4 role + scope wilayah | BENAR | `internal/middleware/rbac.go:37-100`; `domain/schema.go:13-18` |
| Rate limit ganda IP + email | BENAR | `routes.go:118-123` |
| Audit append-only + writer | BENAR | migrasi 000005 + `audit_log_repository.go` |
| Seeder dev-only, tanpa password di log | BENAR | `cmd/seed/main.go:112-124`; password acak ditampilkan sekali di stdout |
| Encrypted NIK tidak transit jalur publik | BENAR (dan lebih kuat dari klaim) | `json:"-"` di `schema.go:189, 244` — aman walau repo memakai `SELECT *` |
| Redis fail-closed production | BENAR | `internal/database/redis.go:31-59` |
| Postgres fail-open masih ada (L-1) | BENAR, masih terbuka | `internal/database/postgres.go:41-48` |
| `ScopeWilayah` tanpa claims langsung `Next()` (L-2) | BENAR, masih terbuka | `internal/middleware/rbac.go:64-69` |
| `mapDBError` belum di `UpdateStatus*`/`SetRevisiToken` | BENAR, masih terbuka | dipakai hanya di `:142, 234, 588` |
| Pesan `Repository ... belum tersedia` belum diganti | BENAR, masih ada 9 titik | `pendaftaran_service.go:251, 287, 401, 421, 441, 497, 625, 695, 732` |
| Proyeksi kolom eksplisit di `GetByID`/`GetByNomor` | BENAR, masih `SELECT *` | `pendaftaran_repository.go:275, 287, 527` |
| `gosec` 0 issue & `govulncheck` 0 vuln | BENAR (direproduksi) | lihat Bagian 2 |
| Pentest 30/30 dan 36/36 PASS | BELUM diverifikasi ulang | suite tersedia, butuh environment lengkap |
| Render PDF KTA & endpoint detail NIK teraudit | BELUM dikerjakan (sesuai pengakuan laporan) | kolom `kta_pdf_key` ada (`migrations/000003_membership_registration.up.sql:90`) namun belum diisi |

Catatan penting: laporan Fase 1 & 2 **jujur dan akurat** — seluruh sisa pekerjaan yang dicentang kosong di bagian 8 memang masih kosong. Temuan KRITIS di Bagian 5 adalah **hal yang belum terdeteksi** oleh audit sebelumnya, bukan klaim palsu.
---

## 5. Temuan BARU — Detail, Dampak, Bukti, dan Perbaikan

### 5.1 KIPAN-BE-001 (KRITIS) — Pembajakan pendaftaran via token revisi tanpa verifikasi pemilik

**Ringkas**: endpoint penerbitan token revisi hanya membutuhkan nomor pendaftaran. Nomor pendaftaran bersifat **sekuensial**, dan status pendaftaran dapat dicek siapa pun tanpa login. Penyerang dapat meminta token revisi milik pendaftar lain, lalu mengganti dokumen identitas pendaftar tersebut.

**Rantai bukti (semua di kode):**

1. Endpoint publik, hanya di-rate-limit (`internal/handler/routes.go:161-167`):

```go
public := v1.Group("/pendaftaran")
public.Use(publicLimiter)                     // 30 request/menit/IP
public.Get("/track/:nomor", handler.TrackStatus)
public.Post("/revisi/request-token", handler.RequestRevisionToken)
public.Put("/revisi/:nomor", handler.SubmitRevision)
```

2. Handler hanya memerlukan field `nomor` (`internal/handler/pendaftaran_handler.go:126-136`):

```go
var payload struct {
    Nomor string `json:"nomor"`
}
_ = c.BodyParser(&payload)
res, err := h.service.RequestRevisionToken(c.Context(), payload.Nomor, auditContextOf(c))
```

3. Service tidak memverifikasi identitas pemohon, hanya status (`internal/service/pendaftaran_service.go:689-716`):

```go
item, err := s.repo.GetByNomorPendaftaran(ctx, nr)
if item.Status != domain.PendaftaranStatusPerbaikan {
    return nil, domain.NewValidationError("Pendaftaran tidak dalam status revisi (PERBAIKAN)")
}
raw, err := crypto.GenerateSecureToken(32)
...
return &domain.RevisionTokenResponse{Token: raw, ExpiresAt: expiresAt}, nil   // token MENTAH di respons
```

4. Nomor pendaftaran sekuensial dan mudah ditebak (`pkg/generator/registration.go:13-24`):

```go
return fmt.Sprintf("REG-%d%02d-%05d", year, month, seq), nil   // REG-202609-00001, REG-202609-00002, ...
```

5. Status pendaftaran siapa pun dapat dibaca tanpa login (`pendaftaran_service.go:395-413`, DTO hanya nomor + status + waktu).

6. Dengan token tersebut, dokumen pendaftar dapat diganti (`pendaftaran_service.go:722-784` menuju `SubmitRevisionTx`, hanya memvalidasi token + pola key + verifikasi storage).

**Proof of Concept (2 permintaan, tanpa autentikasi):**

```bash
# 1) cari kandidat berstatus PERBAIKAN (iterasi nomor, 30/menit)
curl http://localhost:8080/api/v1/pendaftaran/track/REG-202609-00001

# 2) ambil token revisi milik pendaftar itu
curl -X POST http://localhost:8080/api/v1/pendaftaran/revisi/request-token \
  -H 'Content-Type: application/json' -d '{"nomor":"REG-202609-00001"}'
# -> 200 { data: { token: <64 hex>, expires_at: ... } }

# 3) kirim revisi dengan dokumen milik penyerang (token sekali pakai, 24 jam)
curl -X PUT http://localhost:8080/api/v1/pendaftaran/revisi/REG-202609-00001 \
  -H "Content-Type: application/json" \
  -d '{"token":"<token>","ktp_key":"uploads/pendaftaran/.../xxx.jpg"}'
```

**Dampak**: (a) pengambilalihan berkas pendaftaran orang lain (integritas identitas anggota ber-NIA), (b) manipulasi PII pendaftar, (c) pelanggaran prinsip persetujuan/akurasi data UU PDP, (d) token bocor ke pihak ketiga yang tidak sah.

**Bukti belum tercakup pengujian**: test yang ada hanya menguji repositori token (`internal/repository/pendaftaran_repository_test.go:207-246`); paket `internal/handler` tidak punya test sama sekali, dan tidak ada test bahwa penerbitan token butuh bukti identitas.

**Rencana perbaikan (P0):**

1. Wajibkan faktor pemilik saat meminta token: cocokkan `email` **atau** `whatsapp` **atau** 4 digit terakhir NIK terhadap baris pendaftaran; gagal cocok → 403 dengan pesan generik; catat percobaan gagal ke audit log.
2. Jangan pernah mengembalikan token di respons publik. Kirim via kanal terverifikasi (WA/email) — Fase 5. Sebagai langkah transisi, kembalikan hanya `expires_at` + pesan `token dikirim ke kanal terdaftar`.
3. Batasi frekuensi per nomor pendaftaran (mis. 3 permintaan / 24 jam) dengan key limiter `rev_tok:<nomor>`, bukan hanya per IP.
4. Jadikan nomor pendaftaran tidak sekuensial (mis. `REG-202609-7F3K9Q`) atau tambahkan **kode lacak rahasia** yang hanya diketahui pendaftar; simpan hash-nya.
5. Perkuat `GET /pendaftaran/track/:nomor` agar tidak menjadi alat enumerasi (butuh kode lacak, atau balas 404 seragam).
6. Test regresi: (i) request-token tanpa bukti identitas → 403/422; (ii) nomor milik orang lain → gagal; (iii) nomor benar + bukti benar → sukses satu kali; (iv) revisi dengan token kedaluwarsa/terpakai → gagal; (v) rate limit per nomor bekerja.

---

### 5.2 KIPAN-BE-002 (SEDANG) — Presign upload publik tanpa penegakan ukuran/konten

**Bukti**: `internal/service/storage_service.go:126-129` hanya memvalidasi **deklarasi** ukuran dari klien, dan `pkg/storage/s3.go:80-93` menandatangani PUT tanpa `ContentLength`:

```go
out, err := c.presign.PresignPutObject(ctx, &s3.PutObjectInput{
    Bucket:      aws.String(bucket),
    Key:         aws.String(key),
    ContentType: aws.String(contentType),   // tidak ada ContentLength
}, s3.WithPresignExpires(expiry))
```

Verifikasi ukuran sebenarnya baru terjadi saat submit (`storage_service.go:178-180`), sementara penyerang dapat mengunggah objek besar lalu **tidak pernah submit**. Route bersifat publik (`routes.go:200`, 30/menit/IP).

**Dampak**: pembanjiran bucket (biaya penyimpanan/egress + kehabisan kuota), objek sampah tanpa referensi DB, potensi penyalahgunaan bucket sebagai hosting file (MIME di-sign sehingga hanya jpg/png/pdf tertentu, namun tetap bisa dipakai menyimpan banyak objek).

**Rencana perbaikan (P1):**
1. ~~Tambahkan `ContentLength` pada `PresignPutObject`~~ — **dikoreksi v1.1: TIDAK FEASIBLE.** SigV4 presigned PUT tidak memiliki kondisi content-length (hanya POST policy yang punya). Pengganti yang diterapkan (`0f7f67e`): log penerbitan presign untuk deteksi abuse + kuota limiter 30/mnt/IP yang sudah ada + penegakan ukuran keras via HeadObject saat submit.
2. Pasang lifecycle rule: hapus objek di `uploads/` yang lebih tua dari 7 hari dan tidak direferensikan tabel. *(Infra, didokumentasikan di kode `storage_service.go`; dieksekusi saat setup bucket produksi.)*
3. Pertimbangkan bucket policy ukuran maksimum + kuota per IP/hari. *(Infra produksi.)*
4. Tambah worker pemindaian antivirus (ClamAV) untuk berkas yang direferensikan sebelum status DISETUJUI. *(Fase 6.)*

---

### 5.3 KIPAN-BE-003 (SEDANG) — Limiter per-email dapat dipakai melakukan lockout akun (DoS)

**Bukti**: `internal/middleware/security.go:94-118` membuat key dari email di body, lalu `internal/handler/routes.go:121` menetapkan 5 percobaan / 15 menit. Pihak ketiga dapat mengirim 5 login gagal dengan email korban untuk memblokir korban.

**Dampak**: denial of service terhadap akun admin tertentu (termasuk SUPER_ADMIN) secara berulang; juga bisa dipakai menutupi serangan lain.

**Rencana perbaikan (P1):**
1. Gunakan kunci gabungan `email + IP` (bukan email saja), dengan kuota lebih longgar per akun (mis. 20/15 menit) dan lebih ketat per IP.
2. Terapkan *progressive delay*/*exponential backoff* alih-alih blokir keras.
3. Tambahkan CAPTCHA (Turnstile) setelah beberapa kegagalan dan notifikasi ke pemilik akun.
4. Pastikan `LimitReached` tidak membocorkan apakah email terdaftar (pesan sudah seragam — pertahankan).

---

### 5.4 KIPAN-BE-004 (RENDAH) — `/health` membocorkan informasi internal

**Bukti**: `internal/handler/health_handler.go:31-41` mengembalikan `env`, status DB, dan status Redis; route didaftarkan tanpa auth pada `internal/handler/routes.go:36`.

**Dampak**: penyerang memetakan lingkungan (dev/staging/production) dan mengetahui kapan dependensi mati (mis. fase degraded tanpa blacklist) sehingga bisa memilih waktu serangan.

**Rencana perbaikan (P2)**: endpoint publik hanya `{status: ok}`; detail (env, DB, Redis, versi) dipindah ke `/internal/health` yang hanya dapat diakses dari jaringan internal/kredensial monitoring.
---

## 6. Temuan Warisan — Status v1.1: SEMUA DITUTUP

Seluruh item di bawah sudah dikerjakan di branch hardening (`0f7f67e`, `487fc8b`)
dengan test regresi. Bukti kode pada tiap subbagian tetap sah sebagai temuan
awal; status keterbukaan diperbarui di matriks §1.1.

### L-1 PostgreSQL fail-open (RENDAH)
Bukti (`internal/database/postgres.go:41-48`):

```go
if err := db.PingContext(ctx); err != nil {
    log.Warn().Err(err).Msg("Database PostgreSQL belum dapat dihubungi (pastikan service DB aktif)")
} else { ... }
return db, nil       // db dikembalikan walau ping gagal
```
Dampak: di production, server tetap start walau DB tak dapat dihubungi; endpoint mengembalikan 500 alih-alih gagal cepat saat deploy (masalah deteksi dini + health checkpoint palsu).
Perbaikan: kembalikan error bila `cfg.App.Env == "production"` (fail-closed), pertahankan perilaku sekarang hanya untuk dev/test.

### L-2 ScopeWilayah fail-open tanpa claims (RENDAH)
Bukti (`internal/middleware/rbac.go:62-69`):

```go
claims := GetUser(c)
if claims == nil {
    return c.Next()          // fail-open
}
```
Ditambah `GetWilayahScope` yang mengembalikan scope kosong = nasional saat tidak di-set (`rbac.go:27-33`).
Dampak: satu kelalaian menempelkan middleware pada route admin (atau urutan yang salah) berujung pada akses tanpa filter wilayah, bukan penolakan.
Perbaikan: kembalikan 401 saat claims nil; ubah zero value `WilayahScope` menjadi scope yang menolak (mis. flag `valid bool`) supaya lupa set = tolak.

### L-3 Lapisan/duplikasi (RENDAH)
Bukti: handler health memegang langsung `*sqlx.DB` dan `*redis.Client` (`internal/handler/health_handler.go:16-27`); `JWTClaims` didefinisikan di paket middleware (`internal/middleware/auth.go:21-28`) alih-alih domain; parsing header Bearer ditulis dua kali (`middleware/auth.go:76-91` dan `handler/auth_handler.go:107-112`).
Dampak: bukan celah langsung, tetapi memperbesar peluang divergensi logika (mis. format token berbeda antar jalur) dan menyulitkan test.
Perbaikan: pindahkan `JWTClaims` ke `internal/domain`, sediakan satu helper `bearerToken(c)` yang dipakai ulang, dan beri health handler port/interface (mis. `HealthChecker`) agar mudah diuji.

### L-4 Ganti password tidak mem-blacklist access token aktif (RENDAH)
Bukti: `blacklistAccessToken` hanya dipanggil dari `Logout` (`internal/service/auth_service.go:211`), sedangkan `ChangePassword` (`:281-318`) hanya memanggil `RevokeAllUserTokens` (refresh token).
Dampak: setelah korban mengganti password karena akunnya diduga dibajak, access token penyerang tetap valid hingga 30 menit.
Perbaikan: blacklist access token milik pemanggil + simpan `token_version`/`password_changed_at` per user dan tolak JWT dengan `iat` lebih lama (lebih tahan lama untuk semua sesi, termasuk token yang tidak diketahui `jti`-nya).

### L-5 Tidak ada validasi konfigurasi khusus production (RENDAH, naik prioritas)
Bukti (`config/config.go:206-222`): validasi hanya mencakup 3 kunci kripto, secret/TTL auth, dan keberadaan DSN untuk non-development. Default yang tersisa berbahaya bila env keliru:
- `DB_SSLMODE` default `disable` (`config.go:103`) → koneksi DB tanpa TLS di produksi.
- `APP_DEBUG` dapat `true` → `EnablePrintRoutes` mencetak seluruh peta route saat start (`cmd/api/main.go:103`).
- `REDIS_PASSWORD` kosong diizinkan selama `REDIS_ADDR` ada.
- `APP_ALLOW_ORIGIN` tidak divalidasi; CORS memakai `AllowCredentials: true` (`main.go:133-139`) sehingga konfigurasi `*` menjadi kombinasi wildcard + credentials yang rawan.
Perbaikan: tambahkan `validateProduction(cfg)` yang menolak: `DB_SSLMODE != require`, `APP_DEBUG == true`, `REDIS_PASSWORD == ""`, `ALLOW_ORIGIN` mengandung `*`, dan mewajibkan `STORAGE_ENDPOINT` bila dokumen wajib diverifikasi. Jalankan sebelum server listen.

### L-6 Cookie belum configurable (RENDAH)
Bukti (`internal/handler/auth_handler.go:181-190`): `SameSite: "Strict"` dan `Path: "/api/v1/auth"` hardcode. Secara keamanan ini aman (justru paling ketat), tetapi menyulitkan skenario frontend berbeda domain (SPA di subdomain lain) dan cookie admin path lain.
Perbaikan: jadikan `SameSite`, `Path`, dan `Domain` konfigurasi dengan default aman (`Strict`, `/api/v1/auth`), plus dokumentasi konsekuensi bila dilonggarkan.

### L-7 Trusted proxy terlalu lebar (RENDAH)
Bukti (`cmd/api/main.go:153-166`): di production ditambahkan `10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16` sebagai proxy terpercaya. Bila Go diekspos langsung (mis. port diteruskan) atau ada host lain di VPC, header `X-Forwarded-For` dari host tersebut dianggap sah.
Dampak: pemalsuan IP pada rate limiter dan log audit (bypass kuota per IP, mengaburkan forensik).
Perbaikan: hanya percayai proxy yang benar-benar dipakai (default `127.0.0.1`, `::1`; tambahkan IP Caddy secara eksplisit lewat env `TRUSTED_PROXIES`).

### L-8 Komentar format AES keliru (INFO)
Bukti: komentar `pkg/crypto/crypto.go:29` menulis `base64(nonce):base64(ciphertext):base64(tag)` padahal implementasi mengembalikan `base64(nonce):base64(sealed)` dengan tag tergabung di `sealed` (`:52-57`). Tidak ada dampak keamanan, tapi menyesatkan pembaca/perubahan berikutnya.

---

## 7. Sisa Temuan Fase 2 (bagian 8B) — Status Terkini

| Item 8B | Status | Bukti |
|---|---|---|
| `mapDBError` di `UpdateStatus*`/`SetRevisiToken` | Belum | `pendaftaran_repository.go:380, 398, 447, 482` (hanya `:142, 234, 588` memakai) |
| Pesan `Repository ... belum tersedia` → generik + log | Belum | 9 titik pada `pendaftaran_service.go` |
| Proyeksi kolom eksplisit di `GetByID`/`GetByNomor` | Belum (risiko sudah dimitigasi `json:"-"`) | `pendaftaran_repository.go:275, 287, 527` |
| Endpoint detail + dekripsi NIK teraudit untuk verifikator | Belum | struktur siap (`nik_encrypted`, `MaskNIK`), endpoint belum ada |
| Render PDF KTA server-side | Belum | kolom `kta_pdf_key` ada tapi kosong (`migrations/000003:90`) |
| Turnstile/CAPTCHA di endpoint publik | Belum | butuh frontend + domain |

---

## 8. Higiene Repo, Proses, dan Rantai Pasok

### 8.1 Sampah & risiko kebocoran (KIPAN-BE-008)
- `session-ses_f200.md` (±168 KB, dump transkrip) ada di root dan **tidak** tercakup `.gitignore`; `git status` menandainya untracked sehingga berisiko ikut ter-commit.
- `backend/tmp_srv_b4.exe`, `tmp_srv_b4.exe~`, `tmp_srv_b5.exe`, `tmp_srv_b5.exe~`, `tmp_srv_h1.exe` masih ada (ter-ignore `*.exe`, tetapi tetap harus dihapus).
- `docs/pr-fase1/` dan `docs/pr-fase2/` masih untracked (belum di-commit).
- Branch lokal `backend-fase-2-membership-pendaftaran` **ahead 13 commit** dari origin → risiko kehilangan pekerjaan dan laporan PR tidak dapat direview.

### 8.2 Rahasia & konfigurasi
- `backend/.env` benar-benar tidak ter-track (bukti: `git ls-files` + `git check-ignore`). Isi lokalnya memuat 3 kunci kripto 64-hex, `AUTH_ACCESS_TOKEN_SECRET` 64 char, serta kredensial storage dev. Konsekuensi: sekali file ini bocor/terkirim, seluruh asumsi kriptografi runtuh → **rotasi keempat kunci bila file pernah dibagikan**.
- Tidak ditemukan secret hardcode di kode Go (hanya konstanta pada file test).

### 8.3 Alur kerja & tooling
- `make migrate-up` dan `make migrate-down` masih gagal (`unknown driver`) karena CLI migrate dibuild tanpa tag (`Makefile:24-29`). Perbaikan: tambahkan `-tags 'postgres file'` (seperti pada dokumentasi laporan PR).
- Cakupan test belum merata (KIPAN-BE-010). Paket tanpa test: `cmd/api`, `cmd/flushcache`, `cmd/seed`, `config`, `internal/database`, `internal/handler`, `pkg/response`, `pkg/validator`. Yang paling penting adalah `internal/handler` karena ia permukaan serangan HTTP (parsing, otorisasi, cookie).

### 8.4 Rantai pasok (supply chain)
`govulncheck -show verbose` melaporkan 0 kerentanan yang **dipanggil** kode, dengan dua catatan tidak berdampak langsung:
- `github.com/valyala/fasthttp@v1.51.0` (dependensi tidak langsung Fiber) tercatat pada GO-2026-4950 — tidak terpanggil oleh kode kita, tetapi penjadwalan upgrade Fiber (yang akan menaikkan fasthttp) disarankan.
- `golang.org/x/crypto` tercatat GO-2026-5932 (modul-level, terkait paket `openpgp` yang tidak dipakai).
---

## 9. Rencana Remediasi Berprioritas — Status v1.1: P0–P2 SELESAI

Seluruh langkah P0–P2 di bawah selesai di branch `fix/fase-1-fase-2-hardening`
(P0: `aaa9f70`; P1: `0f7f67e`; P2: `487fc8b`), suite v2.5 41/41 PASS.
Tersisa di luar remediasi: render PDF KTA, endpoint 1.4.2, Turnstile
(DITUNDA — belum ada frontend, pengganti sementara: limiter + DTO minimal +
kuota/nomor; syarat gerbang frontend/produksi), kirim token via WA/email
(Fase 5), sinkron checklist TAHAPAN, hapus branch `fix/` lama.

### P0 — wajib sebelum merge/deploy (blocker) — ✅ selesai

| Langkah | Berkas yang disentuh | Kriteria selesai |
|---|---|---|
| Tutup KIPAN-BE-001: verifikasi pemilik + jangan kirim token di respons + limiter per nomor | `internal/service/pendaftaran_service.go`, `internal/handler/pendaftaran_handler.go`, `internal/handler/routes.go`, `internal/domain/pendaftaran.go` | request-token tanpa bukti identitas gagal; test baru hijau; audit log mencatat percobaan gagal |
| Nomor pendaftaran tidak sekuensial (atau tambah kode lacak rahasia) | `pkg/generator/registration.go`, `internal/repository/pendaftaran_repository.go`, migrasi baru | nomor tidak dapat ditebak berurutan; data lama tetap terbaca |
| Perkuat `GET /pendaftaran/track/:nomor` agar bukan alat enumerasi | `internal/service/pendaftaran_service.go`, `internal/handler/pendaftaran_handler.go` | butuh kode lacak; balasan seragam |
| Test handler untuk 4 endpoint publik membership | `internal/handler/*_test.go` (baru) | ada test otorisasi/validasi/rate limit |

### P1 — cepat, dampak jelas — ✅ selesai

| Langkah | Berkas | Kriteria selesai |
|---|---|---|
| KIPAN-BE-002: `ContentLength` pada presign + lifecycle objek yatim | `pkg/storage/s3.go`, `internal/service/storage_service.go`, dokumen infra | upload melebihi batas ditolak S3; objek yatim terhapus otomatis |
| KIPAN-BE-003: limiter login berbasis `email+IP` dengan backoff | `internal/middleware/security.go`, `internal/handler/routes.go` | lockout akun oleh pihak ketiga tidak mungkin lagi |
| L-5: validasi konfigurasi production (fail-fast) | `config/config.go`, `cmd/api/main.go` | server menolak start dengan `sslmode=disable` / `APP_DEBUG=true` / `REDIS_PASSWORD` kosong / origin `*` |
| L-2: `ScopeWilayah` dan `GetWilayahScope` fail-closed | `internal/middleware/rbac.go`, `rbac_test.go` | test baru: tanpa claims → 401; scope belum di-set → tolak |
| L-1: PostgreSQL fail-closed di production | `internal/database/postgres.go` | ping gagal → server tidak start saat production |
| L-4: blacklist/tolak access token setelah ganti password | `internal/service/auth_service.go`, `internal/repository/user_repository.go`, `internal/middleware/auth.go` | token terbit sebelum ganti password langsung ditolak |
| L-7: trusted proxy eksplisit lewat env | `cmd/api/main.go`, `.env.example` | hanya proxy yang didaftarkan yang dipercaya |

### P2 — kerapian dan pertahanan berlapis — ✅ selesai

| Langkah | Berkas |
|---|---|
| KIPAN-BE-004: `/health` publik minimal, detail ke endpoint internal | `internal/handler/health_handler.go`, `internal/handler/routes.go` |
| KIPAN-BE-005: proyeksi kolom eksplisit (hindari `SELECT *`) | `internal/repository/pendaftaran_repository.go:275, 287, 527` |
| KIPAN-BE-006: pesan generik + log server untuk 9 titik | `internal/service/pendaftaran_service.go` |
| KIPAN-BE-007: `mapDBError` pada `SetRevisiToken`/`SubmitRevisionTx`/`UpdateStatus*` | `internal/repository/pendaftaran_repository.go` |
| L-3: `JWTClaims` ke domain, helper Bearer tunggal, health via interface | `internal/domain`, `internal/middleware`, `internal/handler` |
| L-6: cookie `SameSite`/`Path`/`Domain` configurable dengan default aman | `internal/handler/auth_handler.go`, `config/config.go` |
| L-8: betulkan komentar format ciphertext | `pkg/crypto/crypto.go:29` |
| KIPAN-BE-008: `.gitignore` untuk `session-*.md`, hapus `tmp_srv_*.exe`, commit `docs/pr-*` | `.gitignore`, `backend/`, `docs/` |
| KIPAN-BE-009: perbaiki target migrate pada Makefile | `Makefile:24-29` |
| KIPAN-BE-010: tambah test handler, config, validator | paket terkait |

### Usulan urutan kerja (1 sprint kecil)
1. Hari 1–2: KIPAN-BE-001 (kode + test + migrasi bila memilih nomor non-sekuensial).
2. Hari 3: L-1, L-2, L-5 (fail-closed trio) + test.
3. Hari 4: KIPAN-BE-002, KIPAN-BE-003.
4. Hari 5: sisa P2 + higiene repo + push branch + jalankan pentest suite v2.5 (tambahan gate untuk temuan baru).

---

## 10. Lampiran A — Perintah Reproduksi Bukti

```powershell
# 1) Tool keamanan & test
cd backend
go vet ./...
gosec -exclude-dir=bin ./...
govulncheck -show verbose ./...
go test ./... -count=1
go test ./... -race -count=1
$env:TEST_DATABASE_URL=$env:DATABASE_URL; go test ./internal/repository/ -race -count=1 -v

# 2) Bukti KIPAN-BE-001 (server harus jalan; tanpa autentikasi sama sekali)
curl http://localhost:8080/api/v1/pendaftaran/track/REG-202609-00001
curl -X POST http://localhost:8080/api/v1/pendaftaran/revisi/request-token -H 'Content-Type: application/json' -d '{"nomor":"REG-202609-00001"}'

# 3) Higiene repo
git status --short
git check-ignore -v KIPAN_INDONESIA session-ses_f200.md backend/.env
Get-ChildItem backend -Filter tmp_*

# 4) Bukti L-1 (server tetap start walau DB mati) — hentikan Postgres, lalu:
go run ./cmd/api    # perhatikan hanya muncul warning
curl http://localhost:8080/health
```

---

## 11. Lampiran B — Peta Berkas per Temuan

| Berkas | Temuan terkait |
|---|---|
| `internal/service/pendaftaran_service.go` | KIPAN-BE-001, KIPAN-BE-006 |
| `internal/handler/pendaftaran_handler.go` | KIPAN-BE-001, KIPAN-BE-010 |
| `internal/handler/routes.go` | KIPAN-BE-001, KIPAN-BE-003, KIPAN-BE-004, KIPAN-BE-010 |
| `pkg/generator/registration.go` | KIPAN-BE-001 |
| `internal/service/storage_service.go`, `pkg/storage/s3.go` | KIPAN-BE-002 |
| `internal/middleware/security.go` | KIPAN-BE-003 |
| `internal/handler/health_handler.go` | KIPAN-BE-004, L-3 |
| `internal/repository/pendaftaran_repository.go` | KIPAN-BE-005, KIPAN-BE-007 |
| `internal/database/postgres.go` | L-1 |
| `internal/middleware/rbac.go` | L-2 |
| `internal/domain/*`, `internal/middleware/auth.go` | L-3 |
| `internal/service/auth_service.go` | L-4 |
| `config/config.go` | L-5, L-6 |
| `internal/handler/auth_handler.go` | L-6 |
| `cmd/api/main.go` | L-5, L-7 |
| `pkg/crypto/crypto.go` | L-8 |
| `Makefile` | KIPAN-BE-009 |
| `.gitignore` (root) | KIPAN-BE-008 |

---

## 12. Lampiran C — Checklist Kesiapan Produksi (Fase 6)

- [x] Semua P0 dan P1 di Bagian 9 selesai dengan test regresi.
- [x] Pentest suite diperluas (REV-01..04, HEALTH-01) dan hijau 41/41 (v2.5).
- [x] `make migrate-up` diperbaiki (tags + target version).
- [ ] Rotasi 4 kunci (`AES_MASTER_KEY`, `BLIND_INDEX_KEY`, `KTA_SIGNING_KEY`, `AUTH_ACCESS_TOKEN_SECRET`) dijalankan dari secret manager (Vault), bukan `.env`.
- [ ] CORS `ALLOW_ORIGIN` dikunci ke domain produksi (tanpa wildcard; validasi sudah ada di `validateProduction`), HSTS di-set di Caddy.
- [ ] Backup + PITR PostgreSQL teruji restore; Redis dengan password + network policy.
- [ ] Data residency PII sesuai UU PDP (IS3 Indonesia), enkripsi at-rest bucket.
- [ ] Notifikasi token revisi lewat WA/email (Fase 5) sehingga token tidak pernah kembali ke respons HTTP. **Sementara: penerimaan risiko token-di-respons** (bukti ganda + kuota/nomor + 24 jam + sekali pakai; disetujui pemilik).
- [ ] Turnstile/CAPTCHA di endpoint publik (DITUNDA — belum ada frontend; pengganti sementara: limiter + error generik + DTO minimal).
- [ ] Monitoring: alert pada lonjakan 401/403/409/429 dan pada `TOKEN_REUSE` di activity log.
- [ ] Review akses DB: user aplikasi hanya `SELECT/INSERT/UPDATE` pada tabel operasional; trigger append-only `activity_logs` diverifikasi pada koneksi aplikasi.

---

## 13. Penutup

Backend Go pada Fase 2 sudah memiliki fondasi keamanan yang kuat dan sudah jauh lebih matang daripada aplikasi lama. Prioritas tunggal yang paling mendesak adalah **KIPAN-BE-001**: selama token revisi dapat diterbitkan hanya dengan menebak nomor pendaftaran, seluruh kerangka otorisasi wilayah (yang sudah dibangun dengan benar) dapat dilewati lewat jalur publik yang tidak terlindungi. Setelah temuan itu ditutup bersama `L-1/L-2/L-5` dan `KIPAN-BE-002`, backend ini layak dianggap siap untuk Fase 3 dan selanjutnya menuju persiapan produksi.

### Riwayat dokumen

| Versi | Tanggal | Perubahan |
|---|---|---|
| 1.0 | 28 September 2026 | Audit awal: verifikasi klaim PR Fase 1 & 2, temuan baru KIPAN-BE-001 s/d KIPAN-BE-010, konfirmasi utang teknis L-1 s/d L-8 |
| 1.1 | 28 September 2026 | Remediasi selesai: seluruh BE-001 s/d BE-010 (parsial BE-010) dan L-1 s/d L-8 DITUTUP (`aaa9f70`, `0f7f67e`, `487fc8b`); suite v2.5 41/41; koreksi BE-002 fix #1 (mustahil di presigned PUT); Turnstile + token-via-WA/email eksplisit ditunda; penerimaan risiko token-di-respons dicatat |
