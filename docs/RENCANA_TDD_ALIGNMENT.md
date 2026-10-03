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
- B1 Retensi DITOLAK + unique NIK parsial — migrasi `000028` ✅ **Selesai**.
- B2 Kolom RIWAYAT ✅ **Selesai**.
- B3 Tambah/Edit Anggota ✅ **Selesai**.

### Fase C — Fitur Admin
- C1 Laporan & Statistik (+ demografi + anomali NIA) ✅ **Selesai**.
- C2 Penelusur Audit + CSV ✅ **Selesai**.
- C3 Manajemen User scope Nasional ✅ **Selesai**.
- C4 Role & Wewenang (katalog read-only) ✅ **Selesai**.
- C5 Profil Organisasi — migrasi `000029` ✅ **Selesai**.
- C6 Database Backup (`pg_dump`) — migrasi `000030` ✅ **Selesai**.

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

## 11. Catatan Fase B — B1 Retensi DITOLAK + Unique NIK Parsial (TDD D16)

- **Migrasi `000028`**: `DROP INDEX uq_pendaftaran_nik_hash` (global) →
  `CREATE UNIQUE INDEX uq_pendaftaran_nik_hash ON pendaftaran (nik_hash)
  WHERE status <> 'DITOLAK'`. Baris DITOLAK disimpan permanen & tidak memblokir
  pendaftaran ulang.
- **Repository**: `GetByNikHash` hanya mengembalikan baris **non-DITOLAK**
  (`ORDER BY id DESC LIMIT 1`); arsip DITOLAK diabaikan.
- **Verifikasi**: `build`/`vet`/`test` hijau; test integrasi
  `TestRejectedNIKCanReregister` (DITOLAK → GetByNikHash 404 → daftar ulang
  sukses → GetByNikHash mengembalikan yang baru → non-DITOLAK kedua ditolak);
  index definisi terverifikasi memiliki predikat `<> 'DITOLAK'`.

## 12. Catatan Fase B — B2 Kolom RIWAYAT (TDD §5.5)

- **Domain** `riwayat.go`: `PengurusRiwayatRow` + `FormatRiwayat` — jabatan
  **aktif terbaru** menang; bila tak ada, **riwayat terakhir**; bila kosong `-`.
  Periode `YYYY-YYYY` dari SK; label wilayah by level (Nasional/nama prov/nama kab).
- **Repository** `anggota_repository.go`:
  - `anggotaListColumns` + `a.pekerjaan` (kolom PEKERJAAN).
  - `RiwayatByAnggotaIDs(ids)` — query batch `IN (...)`, status efektif dari
    `pengurus_status_efektif(...)`, dikelompokkan lalu `FormatRiwayat`.
- **Service** `anggota_service.go`: `attachRiwayat` dipakai di `ListAnggota`,
  `ListAnggotaCursor`, dan `GetAnggotaDetail` (best-effort; gagal → kosongkan).
- **DTO**: `AnggotaListItem` + `pekerjaan`/`riwayat`; `domain.Anggota` + `riwayat`.
- **Frontend**: Data Anggota — kolom **Nama/NIA · Pekerjaan · Riwayat** · Wilayah ·
  Status; detail menampilkan "Riwayat Kepengurusan".
- **Verifikasi**: `build`/`vet`/`test` + `tsc`/`lint`/`build` hijau; golden test
  `TestFormatRiwayat` (3 skenario Mirwan); integrasi `TestRiwayatByAnggotaIDs`;
  E2E list & detail menampilkan riwayat benar.

> **Fase B tersisa: B3 Tambah/Edit Anggota.**

## 13. Catatan Fase B — B3 Tambah/Edit Anggota (TDD §6.5)

- **Endpoints** (semua admin, `ScopeWilayah`; mutasi `MutatingRateLimit("agt_mut")`):
  - `POST /admin/anggota` — tambah langsung (NIK enkripsi+blind index, NIA
    auto, cek duplikat anggota & pendaftaran aktif, validasi wilayah).
  - `PUT /admin/anggota/:id` — sunting (field opsional; NIK/NIA tetap).
  - `PATCH /admin/anggota/:id/status` — ubah status (soft).
  - `DELETE /admin/anggota/:id` — soft delete (status NONAKTIF).
  - `GET /admin/anggota/export.csv` — ekspor CSV ter-scope (NIA, Nama, Pekerjaan,
    Riwayat, Provinsi, Kabupaten, Status; maks 5000 baris).
- **Repository**: `AllocateNIA` (sequence per kab/tahun), `Create`, `Update`.
- **Service** `anggota_admin.go`: `CreateAnggota/UpdateAnggota/SetAnggotaStatus/ExportCSV`.
- **Frontend**: modal `AnggotaFormModal` (tambah/edit, pilih wilayah), tombol
  Tambah/Edit/Nonaktifkan/Ekspor CSV di Data Anggota & detail.
- **Catatan**: pembuatan **akun login** (buat_akun) belum termasuk batch ini —
  akun anggota dibuat via alur pendaftaran atau `reset-password`. Kandidat
  penyempurnaan kecil.
- **Verifikasi**: `build`/`vet`/`test` hijau (test service create/dup/scope/
  update/status/CSV); `tsc`/`lint`/`build` hijau; E2E create→edit→status→export.

> **Fase B SELESAI.** Lanjut **Fase C** (C1 Laporan & Statistik).

## 14. Catatan Fase C — C1 Laporan & Statistik (TDD §6.8)

- **Endpoints** (capability `view_laporan`, `ScopeWilayah`):
  - `GET /admin/laporan` — ringkasan (anggota/status, pengurus aktif efektif,
    SK aktif, menunggu verifikasi), demografi (usia/pendidikan/pekerjaan),
    distribusi wilayah per scope, tren 12 bulan, dan **anomali NIA**.
  - `GET /admin/laporan/export.csv`.
- **Anomali NIA**: NIK didekripsi **hanya di server** (`DecryptAESGCM`), lalu
  4 digit awal dibandingkan kode kabupaten domisili; dibatasi 3000 kandidat.
- **Repository** `laporan_repository.go` (agregat ter-scope, status efektif),
  **service** `laporan_service.go`, **handler/routes** baru + capability
  `view_laporan` (semua admin).
- **Frontend**: halaman `/admin/laporan` (kartu ringkasan, bar demografi/wilayah/
  tren, tabel anomali, Ekspor CSV, Cetak) + menu **Laporan**.
- **Verifikasi**: `build`/`vet`/`test` + `tsc`/`lint`/`build` hijau; E2E
  `GET /admin/laporan` & `export.csv` mengembalikan data benar (termasuk 1
  anomali NIA pada data dev).

## 15. Catatan Fase C — C2 Penelusur Audit + CSV (TDD §7.2)

- **Repository** `AuditLogRepository.List(ctx, domain.AuditFilter)` — filter aksi,
  entitas, aktor (ILIKE), rentang `created_at`, pagination offset, total
  opsional. Tetap **append-only** (hanya baca, tak ada update/delete).
- **Service** `audit_service.go`: `List` + `ExportCSV` (maks 5000 baris) +
  `ParseAuditFilter` (terima RFC3339 atau `YYYY-MM-DD`).
- **Endpoints** (capability `view_audit` = Super/Nasional):
  `GET /admin/audit` dan `GET /admin/audit/export.csv`.
- **Frontend**: halaman `/admin/audit` (filter aksi/entitas/aktor/tanggal,
  tabel, pagination, Ekspor CSV, Cetak) + menu **Jejak Audit** (Super/Nasional).
- **Verifikasi**: `build`/`vet`/`test` + `tsc`/`lint`/`build` hijau; E2E
  Nasional got 200/200, **Kabupaten 403**, total 199 baris.

## 16. Catatan Fase C — C3 Manajemen User scope Nasional (TDD §6.8)

- **Capability** `manage_regional_users` = **Super + Nasional**; route `/admin/users`
  memakai capability ini (sebelumnya `manage_users`/Super-only).
- **Aturan server-side** (`user_admin_service.go`):
  - Admin Nasional hanya boleh melihat **&** mengelola akun **Provinsi/Kabupaten**;
    daftar difilter (`ListAdmin(roles…)`), filter role Super/Nasional → 403.
  - Tidak boleh membuat akun Super/Nasional (403); tetap menjaga: cegah
    menonaktifkan/menghapus akun sendiri & Super Admin terakhir.
- **Repository** `ListAdmin` menerima daftar role (`roles []string`).
- **Frontend**: menu & route Manajemen Pengguna untuk Super+Nasional; bagi
  Nasional, tab Super/Nasional disembunyikan, opsi role dibatasi Prov/Kab, dan
  aksi Ubah/Hapus hanya untuk baris Prov/Kab.
- **Verifikasi**: `build`/`vet`/`test` + `tsc`/`lint`/`build` hijau; E2E:
  GET users nas=200 / kab=403 / super=200; nas filter SUPER_ADMIN=403;
  nas POST Provinsi=201; nas POST Nasional=403.

## 17. Catatan Fase C — C4 Role & Wewenang (katalog read-only, TDD §3.3)

- **Domain**: `RoleInfo` + `RoleCatalog` + `AllRoles()` (5 role, termasuk USER).
- **Service** `role_service.go` (katalog dari `AllCapabilities()` + `AllRoles()`),
  **handler/routes** `GET /admin/roles` (Super Admin saja) — read-only, tanpa migrasi.
- **Frontend**: halaman `/admin/roles` (kartu role + tabel matriks wewenang
  capability × role, Cetak) + menu **Role & Wewenang** (Super).
- **Verifikasi**: `build`/`vet`/`test` + `tsc`/`lint`/`build` hijau; E2E
  Super 200 / Nasional 403; 5 role & 15 capability.

## 18. Catatan Fase C — C5 Profil Organisasi (TDD §2.1)

- **Migrasi `000029`**: tabel 1-baris `organisasi_profile` (+ seed default) —
  nama, singkatan, deskripsi, visi/misi, kontak, sosmed, `logo_url`, `updated_by`.
- **Endpoints**: `GET /organisasi` (publik, policy `org_pub`),
  `PUT /admin/organisasi` (capability `manage_organisasi` = Super, policy `org_mut`).
- **Service**: sanitasi konten (tolak `<`/`>` → anti stored-XSS) + audit.
- **Frontend**: halaman admin `/admin/organisasi` (Super) & halaman publik
  `/profil`; menu **Profil Organisasi** (Super).
- **Catatan**: `logo_url` berupa URL teks (admin menempelkan URL). Upload logo
  via storage = penyempurnaan lanjutan.
- **Verifikasi**: `build`/`vet`/`test` + `tsc`/`lint`/`build` hijau; E2E:
  GET publik 200; PUT Super 200 / Nasional 403; konten `<script>` → 422.

## 19. Catatan Fase C — C6 Database Backup (TDD §8)

- **Migrasi `000030`**: tabel `backups` (riwayat) — filename, object_key, size,
  status, error, created_by.
- **Storage**: `pkg/storage.Client.Delete` + `StorageService` `PutPrivateObject`
  /`PresignPrivateObject`/`DeletePrivateObject` (bucket privat).
- **Service** `backup_service.go`: `Create` (jalankan `pg_dump --format=custom`
  via `exec` dengan timeout, unggah ke `backups/`, catat riwayat, audit;
  **DSN tidak pernah di-log**, fail-closed bila pg_dump/storage tak ada),
  `List`, `DownloadURL` (presign), `Delete` (objek + riwayat).
- **Config**: `BACKUP_ENABLED` (default false), `BACKUP_PG_DUMP_PATH`,
  `BACKUP_TIMEOUT`.
- **Endpoints** (capability `manage_backup` = Super, policy `bak_mut`):
  `POST/GET /admin/backups`, `GET /admin/backups/:id/download`,
  `DELETE /admin/backups/:id`.
- **Frontend**: halaman `/admin/backup` (Buat Backup, tabel, Unduh, Hapus) +
  menu **Database Backup** (Super).
- **Verifikasi**: `build`/`vet`/`test` + `tsc`/`lint`/`build` hijau; E2E:
  CREATE (pg_dump ~170KB → bucket privat) → LIST → DOWNLOAD (presign) → DELETE.

> **Fase C SELESAI.** Seluruh rencana **TDD Alignment (Batch 0 · Fase A · B · C)
> tuntas**. Tersisa (ditunda): **Fase KTA** — kartu/e-KTA PNG+PDF + tanda tangan
> statis, serta Tracking & Validasi KTA.
