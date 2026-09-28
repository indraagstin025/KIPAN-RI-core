# Laporan PR — Fase 1: Fondasi, Kriptografi & Autentikasi Core

> **Branch**: `backend-fase-1-authentication-authorization` → `main`
> **Status**: DRAFT — sudah push, belum merge ke `main`. Isi sudah ditarik ke
> branch Fase 2 via merge `704b417`.
> **Tanggal laporan**: 28 September 2026
> **Cakupan penomoran**: gabungan Fase 0 + Fase 1A (fondasi, kriptografi,
> skema DB, subsistem auth & RBAC). **Di luar cakupan**: membership (Fase 2),
> SK/pengurus (Fase 3), CMS (Fase 4), notifikasi (Fase 5), deploy (Fase 6).

---

## 1. Ringkasan isi PR

| # | Commit | Isi |
|---|---|---|
| 1 | `18bfd61` | Hapus backdoor kredensial seed (migrasi tanpa akun + nonaktifkan akun lama) + FK wilayah RESTRICT + seeder env-only + pentest v2.2 |
| 2 | `8efcd7b` | Cegah session cloning via CAS pada rotasi refresh token |
| 3 | `a60742c` | Flag Secure cookie dari `APP_ENV`, bukan header request |
| 4 | `c15ac0c` | Entrypoint tunggal `./cmd/api`, `.air.toml`, artefak scan keluar dari Git |
| 5 | `5d9c46d` | Redis fail-closed di production, degraded eksplisit dev |
| 6 | `160c314` | Tutup temuan audit M-1 s/d M-5 (logout publik, TTL ≤ 30m, seeder tanpa password di log, `activity_logs` + writer, `VerifyKTASignature`) |
| 7 | `b204813` | Pentest suite v2.3 (regresi LOGOUT + TTL) |
| 8 | `535dda0` | Dokumen checklist tahapan (acuan kerja Fase 2) |

Stat vs `main`: **31 file, +1496 / −346** (termasuk hapus `results.json`/`results.sarif` dari tracking).

---

## 2. Yang dibangun (fondasi + auth core)

- **Kriptografi**: AES-256-GCM (nonce acak) + Blind Index HMAC-SHA256 + Argon2id (64MB/3/2) + KTA HMAC; fail-fast bila kunci tidak 64-hex/berbeda (`config.go`).
- **Auth**: JWT 15 mnt (maks 30 mnt) + refresh rotation + token family + reuse detection (revoke se-family) + CAS anti-clone + blacklist per-jti di Redis + cookie HttpOnly/Secure/SameSite=Strict.
- **RBAC**: 4 role + `ScopeWilayah` (provinsi/kabupaten/nasional); rate limit ganda IP + email untuk login.
- **Audit**: tabel `activity_logs` append-only (trigger tolak UPDATE/DELETE) + writer di login/refresh/logout/reuse/ganti-password.
- **Seeder dev-only**: password dari env atau acak crypto (tampil sekali di stdout, bukan log), guard `APP_ENV`, tanpa kredensial di migrasi.

---

## 3. Endpoint dalam cakupan (kontrak `/api/v1`)

| Method & Path | Auth | Fungsi |
|---|---|---|
| `POST /auth/login` | publik + limiter | JWT 15 mnt + refresh cookie; anti-enumerasi |
| `POST /auth/refresh` | cookie refresh | Rotasi atomik; reuse → 403 + cabut family |
| `POST /auth/logout` | publik + limiter | Cabut via refresh cookie walau access expired; idempoten |
| `GET /auth/me` | Bearer | Profil + nama wilayah |
| `PUT /auth/password` | Bearer | Verifikasi lama + policy kuat + cabut semua sesi |
| `GET /admin/me-scope` | Bearer | Cermin scope wilayah |
| `GET /admin/super-only`, `/admin/nasional-or-super` | Bearer + role | Guard RBAC |
| `GET /health` | — | Status service/DB/Redis |

---

## 4. Migrasi dalam delta (000003 → 000005)

| # | File | Isi |
|---|---|---|
| 3 | `000003_membership_registration` | Tabel pendaftaran/riwayat/anggota/sequence NIA (skema; logika di Fase 2) |
| 4 | `000004_harden_fase1_security` | Nonaktifkan akun seed lama + cabut sesinya; FK users→wilayah RESTRICT |
| 5 | `000005_activity_logs` | Tabel audit append-only + trigger |

000001 (skema auth) dan 000002 (seed tanpa kredensial) sudah ada sebelumnya.

---

## 5. Bukti keamanan (hasil verifikasi, bukan klaim)

- **Audit RULES 1–26** (3 subagent independen): tidak ada CRITICAL/HIGH live; 5 temuan MEDIUM (M-1 s/d M-5) semuanya ditutup + terverifikasi ulang.
- **Gates**: `go build`, `go vet`, `go test -race`, `gosec` (0 issues), `govulncheck` (0 vuln di kode) — hijau saat commit.
- **Pentest suite v2.3**: **30/30 PASS, 0 FAIL** (1 SKIP: SEED-01 butuh password lama yang tak disimpan di repo).
- **Verifikasi langsung**: TTL 720 jam → server tolak start; migrasi 000005 (insert ok, update/delete ditolak trigger); 7 unit test KTA (roundtrip/tamper/wrong-key/malformed).

---

## 6. Cara verifikasi untuk reviewer

```powershell
cd backend
Copy-Item .env.example .env   # isi 4 kunci (openssl rand -hex 32)
go run -tags 'postgres file' github.com/golang-migrate/migrate/v4/cmd/migrate `
  -path migrations -database $env:DATABASE_URL up     # → versi 5 (di branch ini)
$env:SEED_ADMIN_PASSWORD='<kuat>'; go run ./cmd/seed
go run ./cmd/api
go build ./...; go vet ./...; go test ./... -race -count=1; gosec ./...; govulncheck ./...
$env:PENTEST_ADMIN_PASSWORD='<sama>'; ./scripts/pentest_suite.ps1  # → 30/30 (v2.3)
```

Catatan: `make migrate-up` rusak (CLI tanpa build tag postgres) — pakai perintah `go run -tags` di atas.

---

## 7. Yang SENGAJA belum dikerjakan

- **8 LOW hardening** (L-1 s/d L-8: Postgres fail-open, validasi config prod, cookie, trusted proxy, logger, komentar AES) — dijadwalkan batch LOW di Fase 2, tercatat di `docs/pr-fase2/LAPORAN_PR_FASE2.md` §8A.
- **Verifikasi produksi** (HSTS/Caddy, Vault, password seed real) — ranah Fase 6, tak bisa diuji dari dev.
- **Coverage service/repo** — ditutup bertahap di Fase 2 (batch 5).

---

## 8. Sisa yang perlu dilakukan SEBELUM merge ke `main`

- [ ] Selesaikan batch LOW (A) — dikerjakan di branch Fase 2, lalu backport/cherry-pick yang relevan atau merge Fase 2 sekalian
- [ ] Reviewer mereproduksi §6 dengan gate PASSED
- [ ] Hapus branch `fix/fase-1-security-hardening` (sudah tergabung, tak dipakai lagi)
- [ ] Pastikan diff bersih: tanpa `session-*.md`, binary `tmp_*`, `.env`, artefak scan

### Definisi siap-merge

- [ ] Gates + suite hijau hasil reproduksi reviewer
- [ ] Tidak ada CRITICAL/HIGH terbuka (acuan: audit RULES §5 laporan ini)
- [ ] Keputusan merge: langsung ke `main`, atau tunggu Fase 2 selesai lalu satu PR gabungan
