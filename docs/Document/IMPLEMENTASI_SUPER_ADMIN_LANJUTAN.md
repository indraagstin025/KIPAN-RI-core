# Implementasi Lanjutan Super Admin — SK (Draft/Ajukan), Jabatan Berlevel, Halaman Pengurus, Pendaftaran (Draft/Kedaluwarsa), Master Wilayah, Admin Nasional, Manajemen Pengguna, & Optimisasi Skala

> Status: **SELESAI — terimplementasi (B0–B9)** — diperbarui 03 Oktober 2026
> Ruang lingkup: backend Go (`backend/`) + frontend React (`frontend/`)
> Kode dokumen: `IMPLEMENTASI_SUPER_ADMIN_LANJUTAN.md`
> Dokumen terkait: `IMPLEMENTASI_PENGANGKATAN.md`, `IMPLEMENTASI_SK_MULTILEVEL.md`, `PERBANDINGAN_ADMIN_LAMA_BARU.md`, `AUTH_GUIDE.md`.

---

## 1. Ringkasan & Keputusan Terkunci

| # | Area | Keputusan |
|---|---|---|
| 1 | **SK** | Alur `DRAFT → Ajukan`; rantai KAB→PROV→NAS; SK Nasional langsung final + *Single Active SK rule*. |
| 2 | **Pembuat SK** | Super Admin & Admin Nasional bebas pilih level (Nasional/Provinsi/Kabupaten) + wilayah; Provinsi/Kabupaten terkunci ke wilayahnya. |
| 3 | **File SK** | Wajib. |
| 4 | **Masa Berlaku** | `tanggal_berakhir` **wajib** (harus > `tanggal_terbit`). |
| 5 | **Jabatan** | Berlevel (NASIONAL/PROVINSI/KABUPATEN), `UNIQUE(nama, level)`, nama sama di 3 level; validasi `jabatan.level == sk.level`. |
| 6 | **Halaman Pengurus** | Kartu statistik + tab level + filter lengkap + Tambah Pengurus global + Export/Cetak (print browser); Super Admin melihat **seluruh Indonesia**. |
| 7 | **Pendaftaran** | Status awal `DIAJUKAN` → **`DRAFT`**; status `PERBAIKAN` kirim ulang → `DRAFT`. |
| 8 | **Kedaluwarsa** | `DRAFT` > **30 hari** → status **`KEDALUWARSA`**, mekanisme **lazy-on-access**. |
| 9 | **Draf klien** | Isian disimpan di perangkat **1 hari**; lewat 1 hari → dikosongkan; refresh kembali ke langkah terakhir. |
| 10 | **Master Wilayah** | Status **Aktif/Nonaktif** (`is_active`); Tambah dari master; nama/kode read-only; Ketua dihitung dari pengurus; Activity dari `activity_logs`; akses Super/Nasional; 514 (Kemendagri 2025). |
| 11 | **Skala** | `pg_trgm` + GIN, indeks komposit, **cursor pagination** (daftar besar), total opsional, debounce, monitoring. |
| 12 | **Admin Nasional** | **= Super Admin minus grup menu "Pengaturan"** (Role & Wewenang, Manajemen User, Profil Organisasi, Database Backup); scope seluruh Indonesia. |
| 13 | **Manajemen Pengguna** | **Super Admin saja**; CRUD akun admin + kartu hitung per role + tab role; password **auto-generate** (tampil sekali); **soft delete**; **tanpa migrasi baru**. |
| 14 | **Admin Provinsi & Kab/Kota** | Provinsi = scope provinsi (buat SK Provinsi, teruskan SK Kab, **kelola pengurus Prov+Kab** — Opsi A, lihat pendaftaran saja); Kab/Kota = scope kabupaten (buat SK Kabupaten, verifikasi pendaftaran, kelola pengurus Kab). Tanpa Wilayah/Jabatan/Pengaturan. |

---

## 2. SK (Draft → Ajukan)

### 2.1 Alur status
- `CreateSK` menyimpan `approval_status = DRAFT` (semua role; tidak auto-maju).
- Aksi **"AJUKAN"** (`ApproveSK`), target menurut **level SK**:
  - `KABUPATEN` → `MENUNGGU_PROVINSI`
  - `PROVINSI` → `MENUNGGU_NASIONAL`
  - `NASIONAL` → `DISETUJUI` (final, memicu **Single Active SK rule**)
- `TERUSKAN` (Provinsi) → `MENUNGGU_NASIONAL`; `SAHKAN`/`TOLAK` (Nasional/Super) → `DISETUJUI`/`DITOLAK`.

### 2.2 Otorisasi (security by design)
- Aktor dari JWT (bukan body/query).
- **Ajukan**: `canManageSK(actor, sk)` — Kab untuk SK Kabupaten, Prov untuk SK Provinsi, Nas/Super untuk SK Nasional; Super oversight semua.
- **Level & wilayah ditentukan/divalidasi server**:
  - Super/Nasional: pilih `level`; `NASIONAL` tanpa wilayah; `PROVINSI` wajib `provinsi_id`; `KABUPATEN` wajib `provinsi_id`+`kabupaten_id` (validasi `KabupatenInProvinsi`).
  - Provinsi: dipaksa `PROVINSI` + provinsi miliknya.
  - Kabupaten: dipaksa `KABUPATEN` + wilayah miliknya.

### 2.3 Form Buat SK (frontend)
- Field: Nomor SK, Judul SK, **Level**, **Provinsi**, **Kabupaten/Kota** (kondisional), Tanggal Terbit, **Tanggal Berakhir**, **File SK (wajib)**.
- **Banner**: "Penting (Aturan SK Tunggal): mengajukan & menyetujui SK baru otomatis menonaktifkan SK lama selevel+wilayah beserta pengurusnya (Demisioner)."
- Level/wilayah **disabled** untuk Provinsi/Kabupaten.

### 2.4 Filter & tabel
- Filter: **Level** (Semua/Nasional/Provinsi/Kabupaten) · **Status SK** (Semua/Aktif/Tidak Aktif) · **Approval** (Semua/DRAFT/Menunggu Provinsi/Menunggu Nasional/Disetujui/Ditolak).
- Kolom: No · Nomor SK · Judul · Level · Wilayah · Pengurus · Approval · Status SK · Masa Berlaku · Aksi.

### 2.5 Perubahan teknis
- `domain.SKApprovalAction`: + `SKActionAjukan`.
- `domain.SKCreateRequest`: + `Level`, `ProvinsiID`, `KabupatenID`, `TanggalBerakhir`.
- `KepengurusanDeps`: + `WilayahRepo`.
- `skInsertQuery`: + kolom `tanggal_berakhir`.
- `repository.SKFilter`: + `Status` (status SK); `skWhere` filter `sk.status`; `SKListItem`/`skListColumns` + `tanggal_berakhir`.

---

## 3. Jabatan Berlevel (A1)

### 3.1 Migrasi `000019_jabatan_level`
- `ALTER TABLE jabatan ADD COLUMN level VARCHAR(20)`.
- Backfill baris lama → `NASIONAL`, lalu `SET NOT NULL` + `CHECK (level IN ('NASIONAL','PROVINSI','KABUPATEN'))`.
- Ganti `UNIQUE(nama)` → **`UNIQUE(nama, level)`**.
- Seed **27 jabatan** (9 nama × 3 level, nama sama; `is_inti`/`is_active`/`urutan` identik).

### 3.2 Backend
- `domain.Jabatan` + `JabatanRequest`: + `Level`.
- Repository: `List(includeInactive, level)`; Create/Update dengan level.
- **Validasi**: saat tambah/ganti jabatan pengurus → `jabatan.level == sk.level`.

### 3.3 Frontend
- `JabatanPage`: kolom **Level** (badge) + filter **Semua Level** + pilihan Level di form.
- Dropdown jabatan di `SkDetailPage` difilter per `sk.level`; di `PengurusListPage` difilter per `pengurus.level`.

---

## 4. Halaman Pengurus (B1)

### 4.1 Backend
- `PengurusFilter`: + `Level`, `MasaJabatan` (Aktif/AkanBerakhir/Berakhir); `ProvinsiID`/`KabupatenID`/`Status`/`Search` sudah ada.
- `PengurusDetail`: + `sk_tanggal_berakhir` (join `surat_keputusan`) untuk kolom "Masa Jabatan".
- Filter masa jabatan: `Aktif` = `tanggal_berakhir IS NULL OR >= CURRENT_DATE`; `AkanBerakhir` = ≤90 hari; `Berakhir` = `< CURRENT_DATE`.
- **Endpoint baru** `GET /api/v1/admin/pengurus/stats` (ter-scope): total, per level (Nasional/Provinsi/Kabupaten), "Masa Jabatan Akan Berakhir".
- **Super Admin**: tanpa batas wilayah → daftar **seluruh Indonesia**.

### 4.2 Frontend (`PengurusListPage`)
- 5 kartu statistik: Total · Nasional · Provinsi · Kabupaten · Masa Jabatan Akan Berakhir.
- Tab level dengan jumlah: Semua / Nasional / Provinsi / Kabupaten.
- Filter: cari, Level, Provinsi (seluruh provinsi untuk Super/Nasional), Kabupaten, Status, Masa Jabatan.
- Kolom: No · Nama/NIA · Jabatan · Level · Wilayah · SK · Status · Masa Jabatan · Aksi.
- Tombol **Tambah Pengurus (global)**: pilih SK (yang boleh dikelola & non-final) → anggota → jabatan (per level SK) → konfirmasi.
- **Export/Cetak**: cetak browser (print → Save as PDF).

---

## 5. Pendaftaran (C1)

### 5.1 Migrasi `000020_pendaftaran_draft`
- Ubah `chk_pendaftaran_status`: hapus `DIAJUKAN`, tambah **`DRAFT`** dan **`KEDALUWARSA`**.
- Migrasi baris lama `DIAJUKAN` → `DRAFT`.

### 5.2 Alur
- Submit publik (data lengkap) → `DRAFT`.
- Admin: `DRAFT` → (Verifikasi) → `DIVERIFIKASI` → `DISETUJUI`/`DITOLAK`/`PERBAIKAN`.
- `PERBAIKAN` → pendaftar kirim ulang → `DRAFT` lagi.
- `DRAFT` yang belum diverifikasi **> 30 hari** → **`KEDALUWARSA`** (lazy-on-access).
- `DIAJUKAN` **dihapus** dari enum/transisi.

### 5.3 Backend
- `schema.go`: `PendaftaranStatusDiajukan` → `PendaftaranStatusDraft = "DRAFT"`; + `PendaftaranStatusKedaluwarsa = "KEDALUWARSA"`.
- `pendaftaran_service.go` submit: `Status = DRAFT`.
- `pendaftaran_repository.go` revisi: status → `DRAFT`.
- `pendaftaran.go` transisi: `DRAFT → DIVERIFIKASI`; label tracking `DRAFT = "Pendaftaran Diterima"`, `KEDALUWARSA = "Kedaluwarsa"`.
- `dashboard_repository.go`: "menunggu" = `DRAFT` + `DIVERIFIKASI`.
- **Lazy expiry**: sebelum list/track, `UPDATE pendaftaran SET status='KEDALUWARSA' WHERE status='DRAFT' AND created_at < now()-interval '30 days'`.
- Pesan revisi: "status kembali DRAFT".

### 5.4 Frontend
- `AntreanPage`: **tab status + stepper + jumlah** (Semua/Draft/Diverifikasi/Disetujui/Ditolak/Perbaikan/Kedaluwarsa).
- `StatusBadge`: tambah gaya `DRAFT`, `KEDALUWARSA`.
- Tracking: label dari backend.
- **Draf klien**: `localStorage` disimpan **maks 1 hari** (auto-bersih); saat refresh **kembali ke langkah terakhir**; OTP diminta ulang saat Kirim.

---

## 6. Master Wilayah (Super/Nasional)

### 6.1 Model & aturan
- Pakai `wilayah_provinsi`/`wilayah_kabupaten`; **status = `is_active`** (Aktif/Nonaktif).
- **514** kabupaten/kota (Kemendagri 2025); **nama/kode read-only** (dari master wilayah.id).
- **Ketua** = dihitung dari `pengurus` (jabatan inti "Ketua", level sesuai, status `Aktif`); menetapkan ketua dilakukan via fitur **Tambah Pengurus** (halaman Pengurus).
- **Keamanan**: akses **Super Admin & Admin Nasional**; perubahan status dicatat audit.

### 6.2 Backend (endpoint admin)
- `GET /api/v1/admin/wilayah?type=provinsi|kabupaten&search=&status=&provinsi_id=` → daftar + `jml_kabupaten` (provinsi), `jml_pengurus` (level sesuai), `ketua`.
- `GET /api/v1/admin/wilayah/:type/:id/detail` → info wilayah + `pengurusList` + `statistik` (total/aktif, total kabupaten, tren 6 bulan) + `activity`.
- `GET /api/v1/admin/wilayah/:type/:id/pengurus?all=` → pengurus level sesuai (default Aktif).
- `PATCH /api/v1/admin/wilayah/:type/:id` → ubah **status** (Aktif/Nonaktif) + audit.
- `POST /api/v1/admin/wilayah` → **tambah dari master** (pilih provinsi + kabupaten; kode/nama otomatis) → pastikan ada & Aktif + audit.
- **Audit wilayah**: `activity_logs` (entity `wilayah_provinsi`/`wilayah_kabupaten`, action `CREATE`/`UPDATE`); tab Activity membaca dari sana.
- Statistik & hitungan efisien (indeks `000021`, `LEFT JOIN LATERAL`).

### 6.3 Frontend
- Menu **Wilayah** (Super/Nasional) di nav admin.
- **Halaman**: kartu (Total Provinsi 38 · Total Kabupaten 514 · Total Pengurus), tab **Provinsi/Kabupaten**, filter (cari/status/provinsi), **ringkasan hasil filter**, tabel kolom: `No · Kode · Nama · Jml Kab/Kota (provinsi) · Provinsi (kabupaten) · Jml Pengurus · Ketua · Status · Aksi`, aksi (Detail / Edit / Aktifkan–Nonaktifkan).
- **Detail modal**: tab **Informasi · Pengurus · Statistik · Activity**.
- **Edit/Tambah modal**: pilih Provinsi + Kabupaten dari master (kode/nama auto), Ketua (read-only), Status (Aktif/Nonaktif).

> Karena **semua 514 sudah di-seed**, tombol "Tambah dari master" praktis untuk **mengaktifkan/mendaftarkan ulang** wilayah berstatus Nonaktif (create bila belum ada).

---

## 7. Admin Nasional

### 7.1 Definisi
> **Admin Nasional = Super Admin, tanpa grup menu "Pengaturan"** (Role & Wewenang, Manajemen User, Profil Organisasi, Database Backup).

### 7.2 Menu
- Dasbor (analitik, seluruh Indonesia)
- **Master Data**: Wilayah · Data Anggota · Surat Keputusan · Jabatan · Pengurus
- **Pendaftaran**: Pendaftaran Baru · Verifikasi Anggota
- **Laporan**: Statistik · Cetak Laporan
- Akun Saya
- ❌ Role & Wewenang · Manajemen User · Profil Organisasi · Database Backup (**Super eksklusif**)
- (CMS Berita/Galeri/Program: di-skip dulu.)

### 7.3 Hak & scope
- Scope **seluruh Indonesia** (tanpa batas wilayah).
- Boleh **verifikasi** pendaftaran; boleh **buat + SAHKAN/TOLAK SK Nasional**; boleh kelola **Jabatan** & **Wilayah**.
- **Perbedaan kunci**: **tidak ada Manajemen User**.
- Selaras `URD.md` §4.3 ("Kelola akun admin" = Super saja).

### 7.4 Implementasi
- Backend: tanpa perubahan khusus (Nasional sudah ber-scope nasional + boleh verifikasi/approve lewat `CanAccessWilayah`/`RequireRoles`).
- Frontend: nav admin menyesuaikan role (menu grup "Pengaturan" tidak tampil untuk Nasional).

---

## 8. Admin Provinsi & Admin Kabupaten/Kota

### 8.1 Admin Provinsi (DPW)
- **Scope**: provinsi sendiri.
- **Menu**: Dasbor · Data Anggota · Surat Keputusan · Pengurus · (CMS skip) · Cetak Laporan · Akun Saya. (Tanpa Wilayah/Jabatan/Pendaftaran/Verifikasi/Statistik/Pengaturan.)
- **SK**: buat SK **Provinsi** (prov sendiri); **TERUSKAN** SK Kabupaten; boleh **nonaktifkan** SK di provinsinya; **tidak** sahkan final.
- **Pengurus (Opsi A)**: boleh kelola pengurus **Provinsi + Kabupaten** di dalam provinsinya (tambah/hapus/ganti jabatan).
- **Data Anggota**: scope provinsi; lihat/**edit**; **Tambah Anggota**; **Export PDF/Excel**.
- **Pendaftaran**: **lihat saja**, tidak verifikasi (§2A).

### 8.2 Admin Kabupaten/Kota (DPD)
- **Scope**: kabupaten/kota sendiri.
- **Menu**: Dasbor · Data Anggota · Surat Keputusan · Pengurus · Pendaftaran Baru · Verifikasi Anggota · (CMS skip) · Cetak Laporan · Akun Saya.
- **SK**: buat SK **Kabupaten** (kab sendiri); nonaktifkan SK Kabupaten; **tidak** sahkan.
- **Pengurus**: kelola pengurus **Kabupaten** saja.
- **Data Anggota**: scope kabupaten; lihat/**edit**; **Tambah Anggota**; **Export PDF/Excel**.
- **Pendaftaran/Verifikasi**: **wewenang eksklusif** Kab/Kota.

### 8.3 Aturan umum
- **Jabatan** & **Wilayah** → Super/Nasional saja; **Manajemen Pengguna** → Super saja.
- Buat SK: Super/Nasional bebas level; **Provinsi** hanya SK Provinsi; **Kabupaten** hanya SK Kabupaten.
- Nonaktifkan SK: Provinsi (provnya, termasuk SK Kabupaten), Kabupaten (kabnya).
- **Otorisasi pengelola pengurus (`canManageSK`)**: `KABUPATEN` → Admin Kabupaten sekab **atau** Admin Provinsi seprov; `PROVINSI` → Admin Provinsi seprov; `NASIONAL` → Nasional/Super; Super oversight semua.

---

## 9. Manajemen Pengguna (Super Admin)

### 8.1 Akses & data
- **Super Admin saja** (`RequireRoles(SUPER_ADMIN)`).
- Kartu "Total Pengguna" = seluruh akun admin (Super+Nasional+Provinsi+Kab/Kota), **hitung live**.
- Total kab/kota mengikuti data nyata (≈ **514**; penimpaan alias dihindari).

### 8.2 Backend (endpoint admin, Super-only)
- `GET /api/v1/admin/users` — list + filter (role, status, search nama/email/wilayah) + **hitungan per role** (5 kartu) + pagination.
- `POST /api/v1/admin/users` — buat akun (nama, email, role, wilayah sesuai role, status); **password auto-generate** (ditampilkan **sekali**).
- `PUT /api/v1/admin/users/:id` — ubah nama/email/role/wilayah/status; **password opsional** (kosong = tetap) + **Reset Password** (auto-generate, tampil sekali).
- `DELETE /api/v1/admin/users/:id` — **soft delete** (`deleted_at`); data terjaga.
- Set `tipe_user='ADMIN'`; validasi wilayah & role server-side; email unik (partial index aktif).

### 8.3 Frontend
- Halaman **Manajemen Pengguna**: 5 kartu (Total/Super/Nasional/Provinsi/Kota-Kab), tab role, search, tabel (`Nama & Email · Role · Cakupan Wilayah · Status · Terdaftar · Aksi`), pagination, tombol **Tambah Pengguna**.
- **Modal Edit/Tambah**: Nama, Email, Password (opsional), Role, **wilayah kondisional** (Provinsi untuk DPW; Provinsi + Kabupaten untuk DPD), Status Aktif/Nonaktif. Super/Nasional → "cakupan Nasional".
- Menu **Manajemen User** di grup "Pengaturan".

### 8.4 Keamanan
- Aktor dari JWT; route Super-only.
- **Cegah lockout**: tidak boleh menonaktifkan/menghapus **akun sendiri**.
- **Cabut sesi** saat status→Nonaktif / ganti password / ganti role.
- Audit setiap aksi; password Argon2id (tidak pernah dicatat/di-log).

### 8.5 DB
- **Tanpa migrasi baru** (kolom `role/status/tipe_user/provinsi_id/kabupaten_id/deleted_at` + partial unique email sudah ada).
- Pencarian email/nama skala besar memakai indeks dari `000021_search_indexes`.

---

## 10. Optimisasi Skala (Search & Pagination)

### 10.1 Masalah
- `ILIKE '%q%'` tak memakai btree → sequential scan.
- `OFFSET` besar lambat; `COUNT(*)` tiap halaman mahal.
- SK list memakai subquery korelatif `jumlah_pengurus` per baris.

### 10.2 Rencana
- **Migrasi `000021_search_indexes`**:
  - `CREATE EXTENSION pg_trgm`.
  - GIN trigram: `anggota(nama_lengkap)`, `anggota(nia)`, `surat_keputusan(nomor_sk)`, `surat_keputusan(judul)`.
  - Komposit: `anggota(created_at DESC, id DESC)`, `pendaftaran(status, created_at DESC)`, `pendaftaran(provinsi_id, kabupaten_id, created_at DESC)`, `pengurus(level, created_at DESC)`, `surat_keputusan(level, provinsi_id, kabupaten_id, created_at DESC)`.
- **Cursor (keyset) pagination** untuk Anggota/Pendaftaran/Pengurus/SK: `WHERE (created_at, id) < cursor ORDER BY created_at DESC, id DESC LIMIT n`. Offset tetap untuk daftar kecil (Jabatan).
- **Total opsional** (`with_total=false` default pada daftar besar).
- Ganti subquery korelatif SK → `LEFT JOIN LATERAL` (atau hitung hanya pada detail).
- Frontend: **debounce 300ms**, Next/Prev + infinite scroll.
- Monitoring: `pg_stat_statements` + log slow query (>200ms).

### 10.3 Catatan
- Pencarian **NIK tidak tersedia** (AEAD + blind index exact); search hanya nama/NIA/nomor.
- Bila data > jutaan: pertimbangkan partisi tabel atau mesin pencari eksternal — **belum perlu saat ini**.

---

## 11. Migrasi & Urutan Eksekusi

**Migrasi**: `000019_jabatan_level` · `000020_pendaftaran_draft` · `000021_search_indexes`.
(Master Wilayah, Admin Nasional, dan Manajemen Pengguna **tanpa migrasi baru** — memakai kolom/tabel yang sudah ada.)

**Fase**:
1. **Fase 0** — Migrasi (000019, 000020, 000021).
2. **Fase 1** — Backend: Jabatan(level) → SK(draft/ajukan, level/wilayah, tanggal_berakhir, filter) → Pengurus(filter+stats) → Pendaftaran(rename+expiry) → Wilayah(admin+audit) → Manajemen Pengguna(CRUD, Super-only).
3. **Fase 2** — Frontend: Jabatan → SK → Pengurus → Pendaftaran → Wilayah → Manajemen Pengguna → nav role (Admin Nasional) → draf klien 1 hari.
4. **Fase 3** — Optimisasi skala (cursor, total opsional, debounce, monitoring).
5. **Fase 4** — Testing/regresi/E2E + update dokumen.

---

## 12. Testing & Kriteria Selesai

- **Unit**: validasi `jabatan.level == sk.level`; filter pengurus (level/masa jabatan) + stats; alur `DRAFT→…→KEDALUWARSA`; SK `DRAFT→Ajukan` multi-level + Single Active; Wilayah (detail/statistik/ketua/audit); Manajemen Pengguna (validasi role/wilayah, email unik, cegah lockout).
- **Regresi**: `gofmt`, `go build`, `go vet`, `go test ./...`; `npm run build` + `lint`.
- **E2E**: Kab buat SK DRAFT → Ajukan → Prov teruskan → Nas sahkan; Super buat SK Nasional → Ajukan → final + Single Active; pengurus (kartu/tab/filter seluruh Indonesia); pendaftaran DRAFT → DIVERIFIKASI → DISETUJUI; expiry 30 hari; Master Wilayah (list/detail/edit/tambah/status); Manajemen Pengguna (CRUD + password auto-generate + soft delete).
- **Kriteria**: seluruh alur berjalan, otorisasi server-side (Super-only untuk Manajemen Pengguna), tanpa query berat pada data besar.

---

## 13. Dokumen Terkait
- [`IMPLEMENTASI_PENGANGKATAN.md`](../IMPLEMENTASI_PENGANGKATAN.md)
- [`IMPLEMENTASI_SK_MULTILEVEL.md`](../IMPLEMENTASI_SK_MULTILEVEL.md)
- [`PERBANDINGAN_ADMIN_LAMA_BARU.md`](../PERBANDINGAN_ADMIN_LAMA_BARU.md)
- [`AUTH_GUIDE.md`](../../AUTH_GUIDE.md)

---

## 14. Status Implementasi (Batch B0–B9)

Seluruh batch **selesai & terverifikasi** (backend `build`/`vet`/`test` hijau, frontend `tsc`/`lint`/`build` hijau):

| Batch | Isi | Catatan |
|---|---|---|
| **B0** | Migrasi `000019`(jabatan level), `000020`(pendaftaran draft), `000021`(search indexes) | DB v21 |
| **B1** | SK **Draft→Ajukan**; level/wilayah server-side; `tanggal_berakhir` wajib; file wajib; filter `status`+`approval`; Single Active | `SKActionAjukan`; `CreateFinal` dihapus |
| **B2** | Jabatan berlevel (FE): badge/filter/form Level; dropdown per level | `validateJabatan`, `UNIQUE(nama,level)` |
| **B3** | Pengurus: filter level+masa jabatan, endpoint `stats`, kartu+tab+filter, Tambah global, Export; `canManageSK` **Opsi A** (Prov kelola Kab) | `GET /admin/pengurus/stats` |
| **B4** | Pendaftaran FE: tab+stepper+jumlah; draf klien **TTL 1 hari**; resume langkah terakhir; OTP ulang saat kirim | `DRAFT`/`KEDALUWARSA` |
| **B5** | **Master Wilayah** (BE+FE): cards/list/detail/pengurus/status/tambah + audit `activity_logs` | Super/Nasional |
| **B6** | **Manajemen Pengguna** (Super-only): CRUD, password auto-generate tampil sekali, reset, soft delete, cegah lockout, cabut sesi, audit | Tanpa migrasi |
| **B7** | Role & nav: Nasional = Super minus "Pengaturan"; Provinsi **lihat saja** (verifikasi ditolak server); Kab verifikasi; wilayah/jabatan/users ter-gate | `verification_service.go` tolak Prov |
| **B8** | Optimisasi: keyset cursor (Anggota/Pendaftaran), `with_total`, SK `LEFT JOIN LATERAL`, debounce 300 ms, `pg_stat_statements` + slow-request log | Migrasi `000022`, DB v22 |
| **B9** | Regresi penuh + update dokumen | — |

### Catatan hardening (Fase 0–1 + batch)
- Jabatan duplikat → **409**; ubah `level` jabatan yang dipakai pengurus → **409**.
- Endpoint publik GET tidak menulis; `KEDALUWARSA` dihitung read-only saat tracking, persist hanya di antrean admin.
- Rate-limit registry test dijaga: setiap policy baru (`wil_mut`, `usr_mut`) didaftarkan di `ratePolicies` (fail-fast server).

### Batasan diketahui
- **Cursor/keyset** diterapkan untuk Anggota & Pendaftaran; SK/Pengurus memakai `with_total` (offset) + SK `LEFT JOIN LATERAL`.
- **Tambah/Edit Anggota** (URD §8.1/8.2) belum memiliki endpoint — kandidat batch lanjutan.
- `pg_stat_statements` perlu `shared_preload_libraries` untuk data penuh.

### Tindak lanjut (B-W1–B-W4) — Master Wilayah
- **B-W1**: paginasi **offset** daftar admin wilayah (`page`/`limit`/`total`) untuk Provinsi & Kabupaten/Kota.
- **B-W2**: FE pakai komponen `Pagination` + "Menampilkan X dari Y" + penomoran baris mengikuti halaman.
- **B-W3**: FE **skeleton tabel**, `EmptyState`, error + "Coba lagi", **per-row busy** (toggle status) tanpa overlay global.
- **B-W4**: FE **modal bersama** (`components/ui/modal.tsx`) dengan ESC/backdrop-close/scroll-lock + loading di modal Detail.
- Di luar cakupan (sesuai keputusan): URL sync & Export/Cetak.

### Tindak lanjut (B-P1–B-P6) — UX Pengangkatan Kader → Pengurus
- **Wizard terpadu** `PromotePengurusWizard` (Stepper: SK → Anggota → Jabatan → Konfirmasi) dipakai di **3 pintu masuk**: halaman Pengurus, detail SK (preset SK), detail Anggota (preset Anggota).
- **UX**: jabatan inti terisi = disabled; empty-state informatif; ringkasan efek (tipe→PENGURUS, sesi dicabut, email) + konfirmasi kelayakan; busy per-aksi (bukan Overlay global); state sukses.
- **Panduan aturan SK** ditampilkan: "Buat → Susun Pengurus → Ajukan → Sahkan" + penjelasan **Single Active SK** (di detail SK & form buat SK). Ajukan SK butuh minimal 1 pengurus.
- Helper `canPromotePengurus` (roles.ts) + tombol "Jadikan Pengurus" di `AnggotaDetailPage`.
- **Tanpa perubahan backend** (endpoint & validasi sudah ada).

### Revisi alur pengangkatan (B1H–B1D, 04 Okt 2026)
- **B1H**: perbaikan **404 berkas SK** — `ResolveOwner` mengenali `surat_keputusan.file_sk_key` (berkas SK kini dapat dibuka Kab/Prov/Nas).
- **B1R**: **form Buat SK** memuat langkah **Kader + Jabatan + Tanggal Mulai** → submit membuat SK (Draf) + mengangkat kader; **Ajukan manual** dari detail SK.
- **B1T**: **Tanggal Mulai** jabatan dapat diisi manual (default tanggal terbit).
- **B1P**: mode **"Promosi Pengurus"** (`GET /admin/pengurus/promosi`) di samping "Dari Anggota (Baru)".
- **B1D**: **Nonaktifkan SK** mendemosi pengurus aktif (Demisioner/Diberhentikan + keterangan) secara atomik.
- Diadopsi dari project lama; perbandingan lengkap ada di catatan diskusi (project lama: SK dulu lalu tambah pengurus, tanpa DRAFT).

### Tindak lanjut (P1–P7) — Antrian Email & Kredensial (Opsi A)
- **P1**: antrean `/admin/pendaftaran` mengecualikan `DISETUJUI` (pindah ke Data Anggota).
- **P2**: migrasi `000023_email_outbox` + domain + repository + test.
- **P3**: worker email + config (`EMAIL_WORKER_*`, `SETUP_TOKEN_TTL`) + template `AccountSetupEmail`/`AccountLinkedEmail`.
- **P4**: enqueue di `verification_service` (kredensial via tautan), `one_time_password` dihapus dari respons/UI; reset password anggota → antrian.
- **P5**: `POST /auth/set-password` + admin `/admin/email-outbox` (list/retry/retry-pending/batch).
- **P6**: FE hapus tampilan password, halaman `/set-password`, menu **Antrian Email**.
- **P7**: regresi + docs. Email dev via **Mailtrap sandbox** (worker otomatis + tombol manual).
- Detail: `docs/IMPLEMENTASI_ANTRIAN_EMAIL_STATUS.md` (§13).

