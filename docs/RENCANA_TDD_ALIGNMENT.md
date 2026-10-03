# Rencana Alignment TDD → SIM-KIPAN Core (Go + React)

> Status: **BERJALAN** — eksekusi per batch.
> Ruang lingkup: backend Go (`backend/`) + frontend React (`frontend/`).
> Sumber aturan: `docs/Document/Dokumen_Desain_Teknis_SIM_KIPAN_preview (1).md` (TDD v1.1).
> Prinsip: pertahankan stack **Go + React** (rewrite dari Next.js karena alasan keamanan & performa). TDD dipakai sebagai **sumber aturan bisnis**, bukan teknologi.

---

## 1. Keputusan terkunci

**Tetap Go (tidak diubah):** NIA `KIPAN-IND-{kab4}-{tahun}-{seq6}` (counter per kab/tahun), auth admin (email) & anggota (email + tautan set-password), master wilayah, lingkup publik (track via nomor registrasi), verifikasi satu tahap, rantai SK, nomor SK manual + lampiran, notifikasi WA, status pendaftaran (termasuk `KEDALUWARSA`), usia 16–30, Single Active SK, **isolasi wilayah (tolak lintas wilayah)**.

**Adopsi TDD:** master jabatan (tanpa level, `is_ketua_umum`, semua admin boleh create; PATCH Super/Nasional), larangan jabatan ganda lintas tingkat, PAW + mutasi jabatan aktif, deteksi kedaluwarsa dinamis (tampilan efektif + materialisasi harian), retensi pendaftar DITOLAK (unique NIK parsial non-DITOLAK), tiga kolom riwayat (NAMA+NIA / PEKERJAAN / RIWAYAT), demografi + laporan anomali NIA, penelusur audit + CSV, scope Manajemen User (Admin Nasional kelola Prov/Kab, maker-checker dianjurkan).

**Ditunda:** seluruh KTA — kartu/e-KTA PNG+PDF, tanda tangan statis Ketua Umum, **tracking & validasi KTA** (dibahas bersama fase KTA).

---

## 2. Batch eksekusi

Alur tiap batch: (migrasi → preflight+snapshot+klon bila perlu) → backend (domain→repo→service→handler→router + test) → frontend (service→UI) → `gofmt`/`build`/`vet`/`test` + `tsc`/`lint`/`build` → E2E → dokumen → commit + push.

### Batch 0 — Fondasi
| Batch | Isi | Status |
|---|---|---|
| 0.1 | Worker terpisah (`cmd/worker`) + scheduler + advisory lock | ✅ Selesai |
| 0.2 | Status efektif tunggal (fungsi + view) — migrasi `000025` | ✅ Selesai |
| 0.3 | Capability matrix + test RBAC | Belum |
| 0.4 | Tooling migrasi (preflight + snapshot + klon) + state-machine tersurat | Belum |

### Fase A — Kepengurusan
- A1 Master Jabatan TDD — migrasi `000026` (expand→dedupe→contract).
- A2 Jabatan ganda lintas tingkat — migrasi `000027` (partial unique).
- A3 PAW + Mutasi.
- A4 Kedaluwarsa dinamis (pakai `pengurus_status_efektif` + job harian).

### Fase B — Pendaftaran & Keanggotaan
- B1 Retensi DITOLAK + unique NIK parsial — migrasi `000028`.
- B2 Kolom RIWAYAT.
- B3 Tambah/Edit Anggota.

### Fase C — Fitur Admin
- C1 Laporan & Statistik (+ demografi + anomali NIA).
- C2 Penelusur Audit + CSV.
- C3 Manajemen User scope Nasional.
- C4 Role & Wewenang (katalog read-only).
- C5 Profil Organisasi — migrasi `000029`.
- C6 Database Backup (`pg_dump`, terenkripsi) — migrasi `000030`.

---

## 3. Catatan Batch 0.1

- **Worker terpisah** `cmd/worker`: menjalankan scheduler + `EmailWorker.ProcessOnce`.
- **Scheduler** (`internal/worker/scheduler.go`): tugas berkala, tiap tugas di goroutine sendiri; `Immediate` untuk eksekusi awal.
- **Kunci singleton** (`internal/worker/lock.go`): `pg_try_advisory_lock` pada koneksi tetap; aman multi-instance.
- **API tidak lagi menjalankan worker** (`internal/router/deps.go`); jalankan `go run ./cmd/worker`.
- **Infra bersama** (`internal/infra`): pabrik `MailSender`/`WAGateway` dipakai API & worker.
- **Logging bersama** (`internal/logging`): `Setup()` dipakai `cmd/api` & `cmd/worker`.
- Config baru: `WORKER_TIMEZONE` (default `Asia/Jakarta`).

## 4. Catatan Batch 0.2

- **Fungsi** `pengurus_status_efektif(pengurus_status, sk_status, sk_tanggal_berakhir, ref_date)`
  (migrasi `000025`): `Aktif` hanya bila record `Aktif` **dan** SK `Aktif`
  **dan** masa bakti belum lewat; selain itu `Demisioner`. Non-`Aktif`
  dikembalikan apa adanya. `IMMUTABLE` + parameter `ref_date` → deterministik.
- **View** `v_pengurus_efektif`: proyeksi `pengurus` + kolom `status_efektif`
  (menyuntik `CURRENT_DATE`). Satu sumber kebenaran untuk daftar/riwayat (B2).
- Test integrasi (gated `TEST_DATABASE_URL`): 7 kasus fungsi + view queryable.
- Konsumen menyusul: **A4** (materialisasi) & **B2** (kolom RIWAYAT) memakai
  fungsi/view ini, bukan logika ad-hoc.
