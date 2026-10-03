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
| 0.3 | Capability matrix (`domain/capabilities.go`) + `RequireCapability` + test | ✅ Selesai |
| 0.4 | Tooling migrasi (preflight + snapshot + klon) + state-machine SK tersurat | ✅ Selesai |

### Fase A — Kepengurusan
- A1 Master Jabatan TDD — migrasi `000026` ✅ **Selesai**.
- A2 Jabatan ganda lintas tingkat — migrasi `000027` (partial unique) ✅ **Selesai**.
- A3 PAW + Mutasi ✅ **Selesai**.
- A4 Kedaluwarsa dinamis (pakai `pengurus_status_efektif` + job harian) ✅ **Selesai**.

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

## 5. Catatan Batch 0.3

- **`internal/domain/capabilities.go`**: satu sumber kebenaran wewenang
  (`Capability` × `Role`) + metadata (label/deskripsi) untuk katalog UI.
  Aksesor: `AllCapabilities`, `HasCapability`, `RolesForCapability`,
  `CapabilitiesForRole`, `KnownCapability` (semua mengembalikan salinan).
- **`middleware.RequireCapability(caps...)`**: guard berbasis capability
  (401 tanpa claims, 403 tanpa wewenang) — menggantikan `RequireRoles` yang
  hard-code role di titik yang lebih terbaca. Sudah diterapkan di
  `/admin/wilayah` (`manage_wilayah`) & `/admin/users` (`manage_users`).
- Invariant diuji: Super punya semua capability; `USER` tidak punya satupun;
  sel matriks kunci (manage_users Super-only; verify Kab bukan Prov; wilayah
  Nasional bukan Prov).
- Capability untuk fitur mendatang (`view_audit`, `manage_organisasi`,
  `manage_backup`) sudah dideklarasikan agar **C2/C4/C5/C6** langsung
  memakainya.

## 6. Catatan Batch 0.4

- **State machine SK tersurat** (`internal/domain/sk_transition.go`):
  `SKApprovalTransitionRules` + `ResolveSKApprovalTransition(action, level)`.
  `ApproveSK` kini **memakai resolver** ini (bukan switch tersebar); perilaku
  identik. Golden test mengunci transisi per tingkat + rantai lanjutan.
- State machine **pendaftaran** sudah tersurat sejak sebelumnya
  (`PendaftaranStatusTransitionRules`).
- **Tooling migrasi** `scripts/migrate-preflight.ps1`:
  - Cek **HARD** (harus 0) sebelum migrasi partial unique: pengurus `Aktif`
    ganda lintas SK; `nik_hash` non-`DITOLAK` ganda. Gagal → exit 1.
  - Cek **INFO**: jabatan nama ganda (akan didedupe A1) & SK `DISETUJUI` aktif
    ganda per wilayah.
  - `-Snapshot` → `pg_dump -Fc`; `-CloneTo <db>` → `createdb` + `pg_restore`.
  - Jalankan sebelum migrasi destruktif: `pwsh -File scripts/migrate-preflight.ps1 -Snapshot`
    lalu uji migrasi di klon.

> **Fondasi (Batch 0) SELESAI.** Lanjut ke **Fase A** (mulai A1 Master Jabatan,
> migrasi `000026`) — ingat: jalankan preflight + snapshot dulu.

## 7. Catatan Fase A — A1 Master Jabatan (TDD D14)

- **Migrasi `000026`**: `ADD is_ketua_umum`; dedupe 27→9 (pilih survivor per
  nama: prioritas NASIONAL lalu id terkecil, **remap `pengurus.jabatan_id`**
  ke survivor, hapus duplikat); `DROP level`; `UNIQUE(nama)`. `Ketua` ditandai
  `is_ketua_umum = TRUE`.
- **Backend**: `domain.Jabatan`/`JabatanRequest` tanpa `Level` + `IsKetuaUmum`;
  `JabatanRepository.List(ctx, includeInactive)` tanpa filter level; validasi
  `jabatan.level == sk.level` dihapus dari `pengurus_service.go`.
- **Otorisasi** (capability): `create_jabatan` = **semua admin**;
  `manage_jabatan` = Super/Nasional (ubah/nonaktif). Handler/routes disesuaikan.
- **Frontend**: `JabatanPage` (hapus kolom/filter/form Level; tambah penanda
  Ketua Umum; form tampil untuk semua admin, tombol Ubah hanya Nasional/Super);
  route `/admin/jabatan` dibuka untuk semua admin; nav "Master Jabatan" tampil
  untuk semua admin; dropdown jabatan di SK/Pengurus/Wizard tanpa filter level.
- **Verifikasi**: preflight + snapshot diambil; `build`/`vet`/`test` hijau;
  `tsc`/`lint`/`build` hijau; E2E: Kab `POST` 201, Kab `PUT` 403, Nas `PUT` 200.
- **Catatan**: perubahan ini menggantikan fitur "jabatan berlevel" (batch B2 lama).

## 8. Catatan Fase A — A2 Larangan Jabatan Ganda Lintas Tingkat (TDD D10)

- **Migrasi `000027`**: cleanup defensif (jika ada >1 pengurus `Aktif` per
  anggota, pertahankan yang terbaru & demosikan sisanya) + **unique index
  parsial** `uq_pengurus_single_active ON pengurus (anggota_id) WHERE status='Aktif'`.
- **Kenapa tidak menolak di AddPengurus**: alur pengangkatan (`AddWithPromotion`)
  sudah **menutup record lama** (demote) sebelum membuka yang baru — itulah
  mekanisme "mutasi". Sehingga invariant "satu jabatan aktif per anggota"
  sudah terjaga; indeks parsial menjadi **jaring pengaman DB** (kondisi
  balapan / jalur tulis lain).
- **Error mapping**: pelanggaran indeks pada `AddWithPromotion` (insert) &
  `UpdateStatus` (reaktivasi) dipetakan ke **409** yang jelas via `mapDBError`.
- **Test integrasi** `TestPengurusSingleActiveIndex`: memverifikasi keberadaan
  + predikat indeks dan perilaku (baris `Aktif` kedua untuk anggota sama →
  `unique_violation` 23505).
- Semua alur lain (termasuk `FinalizeSK` yang menonaktifkan SK lama + demosi)
  tetap memenuhi invariant.

## 9. Catatan Fase A — A3 PAW + Mutasi (TDD §5.6)

- **PAW** `PUT /admin/pengurus/:id/paw` (body `{aksi, keterangan}`), aksi:
  `DEMISIONER` / `DIBERHENTIKAN` / `MENGUNDURKAN_DIRI` / `MENINGGAL`.
  - Keterangan wajib; hanya pengurus berstatus `Aktif`.
  - Otorisasi (`canPaws`): Super semua; **DIBERHENTIKAN** khusus Admin
    Provinsi seprov atau Nasional (TDD Tabel 20); lainnya `canManageSK`.
  - Aksi `MENINGGAL` sekaligus menetapkan `anggota.status = MENINGGAL`
    (`AnggotaRepository.SetStatus` baru).
- **Mutasi** `PUT /admin/pengurus/:id/mutasi` (body `{sk_id, jabatan_id,
  tanggal_mulai?, keterangan?}`): menutup record lama (Demisioner, alasan
  "Mutasi ke …") lalu membuka record baru pada SK/jabatan/wilayah tujuan —
  **satu transaksi** (`PengurusRepository.Mutate`). Wewenang: pengelola SK
  tujuan (`canManageSK`); SK tujuan wajib aktif, belum final, ber-file;
  anggota belum tercantum; jabatan aktif & inti tunggal.
- **Frontend** `PengurusListPage`: aksi **PAW** & **Mutasi** (inline) untuk
  pengurus `Aktif`; API `adminPaws`/`adminMutasi`.
- **Verifikasi**: `build`/`vet`/`test` hijau (test unit PAW/Mutasi +
  `tsc`/`lint`/`build` hijau); E2E: PAW Demisioner 200; Mutasi → record lama
  Demisioner & baru Aktif di SK tujuan.

## 10. Catatan Fase A — A4 Deteksi Kedaluwarsa Dinamis (TDD §5.4)

- **Lapis tampilan**: query pengurus kini memakai `pengurus_status_efektif(...)`
  (konstanta `pengurusStatusEfektifExpr`) untuk kolom `status`, filter status,
  `Stats`, dan `ListPromosi` — satu definisi bersama view (`000025`). Masa bakti
  lewat langsung tampil `Demisioner` tanpa menunggu job.
- **Lapis materialisasi**: `PengurusRepository.CloseExpiredAppointments` (satu
  statement `UPDATE ... FROM surat_keputusan ... RETURNING`, idempoten) +
  `service.PengurusExpiryService.RunOnce` menulis audit `DEMISIONER_OTOMATIS`
  per personalia (aktor "Sistem").
- **Worker**: job `pengurus-expired` di `cmd/worker` (interval
  `WORKER_EXPIRY_INTERVAL`, default 24h, `Immediate`) — aman multi-instance
  (advisory lock).
- **Verifikasi**: `build`/`vet`/`test` hijau; test integrasi
  `TestCloseExpiredAppointments` (status efektif Demisioner sebelum job →
  ditutup → tersimpan Demisioner → idempoten); worker mendaftarkan job.

> **Fase A (Kepengurusan) SELESAI.** Lanjut **Fase B** (B1 retensi DITOLAK +
> unique NIK parsial, migrasi `000028`).
