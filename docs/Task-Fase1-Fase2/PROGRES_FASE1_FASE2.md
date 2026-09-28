# Progres Pengerjaan Fase 1 & Fase 2 — SIM-KIPAN Core (Backend)

> **Aturan skop**: dokumen ini hanya mencakup pekerjaan **backend**.
> Item yang butuh frontend/infra dipindah ke PR tersendiri (§4).
> **Update terakhir**: 28 September 2026 — branch `fix/fase-1-fase-2-hardening`
> (HEAD `ef9671e`), tree bersih, sudah push.
> Bukti detail per item ada di `docs/pr-fase1/`, `docs/pr-fase2/`,
> dan `docs/LAPORAN_AUDIT_KEAMANAN_BACKEND.md` v1.1.

---

## 1. Fase 1 — Fondasi, Kriptografi & Autentikasi Core ✅ SELESAI

| # | Pekerjaan | Status | Bukti |
|---|---|---|---|
| 1.1 | Hapus backdoor kredensial seed (migrasi tanpa akun, nonaktifkan akun lama, FK wilayah RESTRICT, seeder env-only) | ✅ | `18bfd61`, migrasi 000004, pentest SEED-01 |
| 1.2 | Session cloning via CAS pada rotasi refresh token | ✅ | `8efcd7b`, `user_repository.go` |
| 1.3 | Flag Secure cookie dari `APP_ENV` | ✅ | `a60742c` |
| 1.4 | Entrypoint tunggal + artefak scan keluar dari Git | ✅ | `c15ac0c` |
| 1.5 | Redis fail-closed production | ✅ | `5d9c46d`, `redis.go` |
| 1.6 | M-1 logout publik idempoten | ✅ | `160c314`, LOGOUT-01..03 |
| 1.7 | M-2 TTL access maks 30 menit fail-fast | ✅ | `160c314`, TTL-01 |
| 1.8 | M-3 seeder tanpa password di log | ✅ | `160c314`, `cmd/seed` |
| 1.9 | M-4 `activity_logs` append-only (trigger) + writer auth flow | ✅ | migrasi 000005, verified live |
| 1.10 | M-5 `VerifyKTASignature` + rotasi kunci (`_PREV`) | ✅ | 7+3 unit test |
| 1.11 | L-1 s/d L-8 LOW hardening (fail-closed trio, RBAC 401, JWTClaims→domain, blacklist ganti password, validasi prod, cookie configurable, proxy eksplisit, komentar AES) | ✅ | `0f7f67e`, `487fc8b`, test regresi |
| 1.12 | Pentest suite v2.3 → v2.5 (30/30 → 41/41 PASS) | ✅ | `b204813`, `487fc8b` |

---

## 2. Fase 2 — Core Membership Engine (backend saja)

### Batch 1 — Submit real ✅ (`cd0c3a5`)

| # | Pekerjaan | Bukti |
|---|---|---|
| 2.1.1 | Validasi kuat (panjang, email, 1 regex WA, tanggal NIK, umur 17–100, pola key, anti `<>`) | 20+ kasus tabel test |
| 2.1.2 | Cek duplikat NIK 2 tabel → 409 | live 409 + integration |
| 2.1.3 | Enkripsi AES-GCM + insert atomik + riwayat SUBMIT + audit | live DB verified |
| 2.1.4 | Sequence REG-YYYYMM-XXXXX dari DB (migrasi 000006, UPSERT anti-race) | live 00001→00002 |

### Batch 2 — Otorisasi ✅ (`cd0c3a5`)

| # | Pekerjaan | Bukti |
|---|---|---|
| 2.2.1 | `ActorContext` + jurisdiction di service, `RequireRoles`, hapus repo dari handler | live 403 lintas wilayah |
| 2.2.2 | Tracking publik DTO minimal (tanpa PII/key) | MEM-03 |
| 2.2.3 | `UpdateStatusWithHistory` 1 tx + audit; limiter publik 30/mnt + mutasi 20/mnt | gates + suite |

### Batch 3 — NIA & KTA ✅ (`cd0c3a5`)

| # | Pekerjaan | Bukti |
|---|---|---|
| 2.3.1 | Format NIA tunggal `KIPAN-PROV-KAB-TAHUN-SEQ` (kode BPS) | live `KIPAN-32-3273-2026-00001` |
| 2.3.2 | `kta_qr_hash` dalam tx penerbitan; endpoint verifikasi asli (HMAC + anti oracle) | KTA valid/tamper/asing |

### H-1 — Storage presigned ✅ (`91da13e`)

| # | Pekerjaan | Bukti |
|---|---|---|
| 2.4.1 | `pkg/storage` (presign PUT/GET, HeadObject, sniff magic, EnsureBucket) | E2E MinIO AGPL lokal |
| 2.4.2 | 6 kategori + batas + key server-generated + fail-closed 503 | 422/503 live |
| 2.4.3 | Verifikasi dokumen saat submit; endpoint presign + view teraudit | submit 201 fiktif 422 |

### Batch 4 — Revisi & antrean ✅ (`1ff2ce1`)

| # | Pekerjaan | Bukti |
|---|---|---|
| 2.5.1 | Token revisi (hash + 24 jam + sekali pakai) + bukti email+WA + limiter/nomor (BE-001) | REV-01..04 |
| 2.5.2 | ListQueue real (scope + status + pagination + non-PII); fix `ListByWilayah` | live + meta |
| 2.5.3 | Validasi master wilayah (`WilayahRepository`); migrasi 000007 (CHECK + RESTRICT) | CHECK tolak BOGUS |

### Batch 5 — Test & pentest ✅ (`51be3e7`)

| # | Pekerjaan | Bukti |
|---|---|---|
| 2.6.1 | AES roundtrip/nonce/tamper + blind index determinism | `crypto_test.go` |
| 2.6.2 | Integration repo 5/5 `-race` (409, sequence, 10 goroutine unik, filter, token) | `TEST_DATABASE_URL` |
| 2.6.3 | Suite v2.4 MEM-01..06; fix limiter prefix unik (temuan 429 prematur) | 36/36 |

### Remediasi audit ✅ (branch hardening)

| # | Pekerjaan | Bukti |
|---|---|---|
| 2.7.1 | P0 BE-001 (bukti ganda + audit gagal) | `aaa9f70` |
| 2.7.2 | P1 fail-closed + limiter email+IP + observabilitas presign | `0f7f67e` |
| 2.7.3 | P2 + test handler/config/validator + suite v2.5 41/41 | `487fc8b` |
| 2.7.4 | Laporan audit v1.1 (seluruh temuan DITUTUP + bukti) | `ef9671e` |

---

## 3. Sisa backend SEBELUM Fase 3 (Batch F2-final)

| # | Pekerjaan | Backend-only? | Status |
|---|---|---|---|
| 3.1 | Merge fast-forward hardening → fase-2 (0 konflik) | Ya | ⏳ Antre |
| 3.2 | Endpoint wilayah publik (dropdown + cross-test suite) | Ya | ⏳ Antre |
| 3.3 | 1.4.2 reveal NIK teraudit (role + scope + audit) | Ya | ⏳ Antre |
| 3.4 | PDF KTA server-side (dep baru + render saat approve + download teraudit) | Ya | ⏳ Antre |
| 3.5 | Suite: cross-wilayah 403 + wilayah + NIK + PDF; sync checklist TAHAPAN | Ya | ⏳ Antre |
| 3.6 | **Pentest mendalam pra-Fase 3** (lihat §5) | Ya | ⏳ Antre |
| 3.7 | Push + PR Fase 2 siap merge | Ya | ⏳ Antre |

---

## 4. Dipindah ke PR tersendiri (butuh frontend/infra)

| # | Item | Kebutuhan | Pengganti sementara |
|---|---|---|---|
| 4.1 | Turnstile/CAPTCHA endpoint publik | Widget + domain frontend | Limiter + error generik + DTO minimal |
| 4.2 | Kirim token revisi via WA/email | Infra notifikasi Fase 5 | Bukti ganda + kuota/nomor (risiko diterima, tercatat) |
| 4.3 | Verifikasi produksi (HSTS/Caddy/Vault/TLS) | Infra Fase 6 | `validateProduction` + docs |

---

## 5. Rencana pentest mendalam pra-Fase 3 (backend)

> Dijalankan SETELAH §3 selesai, SEBELUM kerja Fase 3 dimulai.

| # | Area | Metode | Kriteria lolos |
|---|---|---|---|
| 5.1 | Auth ulang penuh | Suite v2.5+ (41+ test) + fuzz JWT (alg none, kid, exp) | 100% PASS, gate exit 0 |
| 5.2 | Otorisasi matriks | 4 role × endpoint admin × dalam/luar wilayah (termasuk 2 wilayah via §3.2) | Tepat 403/404, nol bocor |
| 5.3 | IDOR/BOLA | ID acak + milik wilayah lain di semua `:id`/`:nomor` | 403/404 seragam, tanpa oracle |
| 5.4 | State machine | Transisi ilegal (approve langsung, double approve, revisi tanpa token, token reuse/expired) | Semua ditolak + audit tercatat |
| 5.5 | Race | Refresh paralel, approve ganda konkuren, sequence (`-race` + burst HTTP) | Satu pemenang, tanpa duplikat NIA/nomor |
| 5.6 | Abuse | Burst submit/track/presign/revisi, payload raksasa, MIME spoof, polyglot, key traversal | 429/422 seragam, magic bytes menolak |
| 5.7 | Kripto | NIK roundtrip massal, nonce unik, QR tamper, rotasi kunci (prev valid, asing tolak) | Semua sesuai test + live |
| 5.8 | Kebocoran | Grep respons/log/audit untuk NIK/plain, token, secret, stack trace, SQL | Nol temuan |
| 5.9 | Gates | build/vet/test `-race`/gosec/govulncheck + migrasi naik di DB bersih | Hijau semua |

Hasil pentest dituangkan di `docs/LAPORAN_PENTEST_PRAFASE3.md` (dibuat saat eksekusi).

---

## 6. Metrik saat ini

- Branch: `fix/fase-1-fase-2-hardening` (13 commit di atas origin fase-2), tree bersih, sudah push
- Gates: build/vet/test `-race`/gosec/govulncheck hijau
- Suite: v2.5, 41/41 PASS (1 SKIP by design)
- Migrasi: 000001 → 000007 (up teruji)
- Endpoint: 8 auth/admin-dummy + 13 membership/storage
