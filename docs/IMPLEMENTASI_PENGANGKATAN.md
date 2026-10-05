# Implementasi Pengangkatan Kader → Pengurus (via SK)

> Status: **SELESAI (P0–P4)** — 03 Oktober 2026
> Tanggal desain: 02 Oktober 2026
> Ruang lingkup: backend Go (`backend/`) + frontend React (`frontend/`)
> Kode dokumen: `IMPLEMENTASI_PENGANGKATAN.md`

> **Catatan implementasi:** seluruh §2–§8 sudah dibangun.
> - Migrasi `000016_kepengurusan_sk` (tabel `jabatan`/`surat_keputusan`/`pengurus` + seed 9 jabatan).
> - Backend: `domain/kepengurusan.go`, `repository/kepengurusan_repository.go`,
>   `service/kepengurusan_service.go`, `handler/kepengurusan_handler.go`, rute `/admin/sk|pengurus|jabatan`.
> - Frontend: fitur `kepengurusan` (SK list/detail, Pengurus, Jabatan) + navigasi admin.
> - Test: `internal/service/kepengurusan/kepengurusan_service_test.go` (rangkaian peran & validasi);
>   verifikasi E2E KAB→PROV→NAS + pengangkatan (flip `tipe` + cabut sesi) di dev.
> - Endpoint ringkas terdokumentasi di `AUTH_GUIDE.md` §8.

> **Lanjutan (03 Okt 2026):** revisi SK multi-level (Kab/Prov/Nas-Super), *Single Active SK rule*, ruang lingkup pengelola pengurus per level, dan fitur **Ganti Jabatan Pengurus** didokumentasikan di `IMPLEMENTASI_SK_MULTILEVEL.md`.


---

## 1. Latar Belakang & Tujuan

Pendaftaran hanya untuk **kader** (jalur pengurus dihapus — lihat rencana penghapusan). Satu-satunya jalan menjadi pengurus adalah **pengangkatan oleh Admin Kabupaten/Kota** melalui SK, mengikuti alur proyek lama dengan penyesuaian yang sudah dikunci.

Tujuan: Admin Kab/Kota dapat menerbitkan SK, menambahkan kader ke SK dengan jabatan, dan status kader berubah menjadi pengurus — tercatat audit dan ternotifikasi.

---

## 2. Keputusan Final (dikunci — 02 Okt 2026)

1. **Hanya Admin Kabupaten/Kota di wilayahnya** yang mengangkat (Provinsi/Nasional tidak punya hak angkat langsung).
2. **File SK wajib** terlampir sebelum anggota bisa ditambahkan.
3. **Jabatan master terkonfigurasi** (tabel + kelola admin), jabatan inti tidak boleh ganda per SK.
4. Tambah anggota **saat SK DRAFT/MENUNGGU**; **dikunci saat SK final (DISETUJUI)** — mau ubah → buat SK baru.
5. **Rantai approval SK penuh**: KAB buat → PROV teruskan → NAS sahkan final.
6. Saat diangkat: **`tipe` berubah KADER→PENGURUS** (`anggota.tipe` + `users.tipe_user`), **paksa logout** (cabut sesi), **email notifikasi**.
7. **Satu admin** memutuskan (kontrol lewat rantai SK, tanpa persetujuan kedua).
8. **Syarat kelayakan di luar sistem** (penilaian admin); sistem hanya minta **checkbox konfirmasi** + validasi teknis (anggota AKTIF, wilayah sama, SK valid, jabatan valid).
9. **Fase**: penghapusan jalur-daftar dulu sampai hijau, pengangkatan menyusul (dokumen ini).
10. **Admin Provinsi boleh melihat antrean provinsinya, tetapi tidak boleh aksi verifikasi** (Setuju/Tolak/Perbaikan/Mulai-verifikasi) — lihat §2A. Ini prasyarat agar matriks peran konsisten sebelum modul SK/pengurus dibangun.

---

## 2A. Batasan Admin Provinsi pada Verifikasi Pendaftaran (prasyarat)

Sesuai proyek lama (`KIPAN_INDONESIA`: `pendaftaran/route.ts:22-28`, `verifikasi/route.ts:36-41`), Admin Provinsi **tidak ikut approve anggota/kader yang daftar**. Rumusan final yang dikunci:

> **Admin Provinsi boleh melihat antrean provinsinya, tetapi tombol aksi verifikasi tidak tersedia untuknya.**

Tugas Admin Provinsi selengkapnya (acuan proyek lama):

| Fitur | Melihat (prov sendiri) | Aksi yang boleh |
|---|---|---|
| Pendaftaran | ✅ | ❌ verifikasi/setujui/tolak (dilarang) |
| Anggota | ✅ | ✅ kelola prov sendiri |
| SK | ✅ (+ yg disetujui) | ✅ buat → teruskan ke nasional; nonaktifkan SK prov/kab (tanpa setuju final) |
| Pengurus | ✅ (prov + kab) | ✅ kelola prov + kab (tanpa level Nasional) |
| Berita | ✅ | ✅ ajukan konten prov (detail di requirement CMS) |
| User/Role/Database/Wilayah/Jabatan/Program/Statistik | ❌ | ❌ (Super eksklusif) |

**Implikasi teknis** (dikerjakan sebelum/seiring modul SK agar matriks peran koheren): `ProcessApproval` (`internal/service/pendaftaran/verification_service.go`) saat ini hanya cek wilayah, sehingga Admin Provinsi lolos ikut menyetujui. Tambahkan **satu penolakan peran**: bila `actor.Role == ADMIN_PROVINSI` dan aksi ∈ {verifikasi, perbaikan, tolak, setujui} → 422 ("khusus Kab/Kota"). Melihat (list/detail) tidak diubah. Sertakan unit test (tolak aksi provinsi, tetap lolos lihat + Kab/Kota/Nas/Super tidak terpengaruh).

---

## 3. Alur Detail (sesuai proyek lama + delta)

### 3.1 Buat SK (Admin Kab/Kota, wilayahnya sendiri)
1. Isi `nomorSK + judul + tanggalTerbit` (wajib) + **upload file SK** (wajib, PDF via kategori `sk`, maks 5 MB, bucket PRIVATE).
2. SK lahir `approvalStatus = MENUNGGU_PROVINSI`, `status = Aktif`.
3. Validasi: nomor SK unik; wilayah = wilayah admin pembuat (tolak lintas wilayah 403); audit `CREATE`.

### 3.2 Rantai persetujuan SK
- `MENUNGGU_PROVINSI` → Admin Provinsi (DPW ybs) meneruskan → `MENUNGGU_NASIONAL`.
- `MENUNGGU_NASIONAL` → Admin Nasional / Super Admin mengesahkan → `DISETUJUI` (final, terkunci) atau `DITOLAK` (wajib catatan).
- SK `DISETUJUI` tidak bisa tambah/hapus pengurus dan tidak bisa diedit isi (buat SK baru untuk perubahan).

### 3.3 Tambah pengurus ke SK (pengangkatan)
Aktor: **hanya `ADMIN_KABUPATEN`** yang `kabupaten_id`-nya sama dengan SK (403 bila beda).

1. Pilih **Jabatan (wajib)** dari master.
2. Cari anggota by nama/NIA (2 mode: *Dari Anggota (Baru)* / *Promosi Pengurus*), hanya yang `status = AKTIF`.
3. Centang konfirmasi kelayakan (syarat offline).
4. Backend memvalidasi berurutan (gagal cepat):
   a. SK ada + `status = Aktif` + `approvalStatus ≠ DISETUJUI` (belum final);
   b. SK.`file_sk_key` terisi (file wajib);
   c. anggota ada + `status = AKTIF` + satu wilayah (`kabupaten_id` sama dengan SK);
   d. anggota belum tercantum di SK ini (anti-duplikat);
   e. jabatan inti belum terisi orang lain di SK ini.
5. Efek atomik (satu transaksi):
   a. Pengurus Aktif lama milik orang tsb (bila ada) → `Demisioner` + keterangan + `tanggal_selesai`;
   b. Buat baris `pengurus` (Aktif, `tanggal_mulai = SK.tanggal_terbit`, level/wilayah = SK);
   c. **`anggota.tipe` KADER → PENGURUS**;
   d. **`users.tipe_user` KADER → PENGURUS** (bila akun terhubung) + **cabut seluruh sesi**;
   e. Audit `CREATE` (pengurus) + `UPDATE` (tipe) + kirim **email notifikasi** pengangkatan (async best-effort).
6. Hapus dari SK: hanya bila SK belum final; else 403.

---

## 4. Perubahan Backend

### 4.1 Skema DB (migrasi baru, berurutan)

```sql
-- jabatan master (terkonfigurasi)
CREATE TABLE IF NOT EXISTS jabatan (
    id SERIAL PRIMARY KEY,
    nama VARCHAR(100) NOT NULL UNIQUE,
    is_inti BOOLEAN NOT NULL DEFAULT FALSE,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
-- seed: Ketua, Sekretaris, Bendahara, ... (is_inti = TRUE untuk jabatan tunggal)

-- surat_keputusan (struct domain sudah ada di schema.go:332)
CREATE TABLE IF NOT EXISTS surat_keputusan (
    id SERIAL PRIMARY KEY,
    nomor_sk VARCHAR(100) NOT NULL UNIQUE,
    judul VARCHAR(255) NOT NULL,
    level VARCHAR(20) NOT NULL CHECK (level IN ('NASIONAL','PROVINSI','KABUPATEN')),
    provinsi_id INT REFERENCES wilayah_provinsi(id),
    kabupaten_id INT REFERENCES wilayah_kabupaten(id),
    tanggal_terbit DATE NOT NULL,
    tanggal_berakhir DATE,
    file_sk_key VARCHAR(255) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'Aktif',
    approval_status VARCHAR(30) NOT NULL DEFAULT 'DRAFT',
    catatan_penolakan TEXT,
    created_by UUID REFERENCES users(id),
    approved_by UUID REFERENCES users(id),
    approved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- pengurus (struct domain sudah ada di schema.go:353)
CREATE TABLE IF NOT EXISTS pengurus (
    id SERIAL PRIMARY KEY,
    anggota_id INT NOT NULL REFERENCES anggota(id),
    surat_keputusan_id INT NOT NULL REFERENCES surat_keputusan(id),
    level VARCHAR(20) NOT NULL,
    provinsi_id INT REFERENCES wilayah_provinsi(id),
    kabupaten_id INT REFERENCES wilayah_kabupaten(id),
    jabatan_id INT NOT NULL REFERENCES jabatan(id),
    status VARCHAR(30) NOT NULL DEFAULT 'Aktif',
    keterangan_status TEXT,
    tanggal_mulai DATE NOT NULL,
    tanggal_selesai DATE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (surat_keputusan_id, anggota_id)
);
CREATE INDEX IF NOT EXISTS idx_pengurus_anggota ON pengurus (anggota_id, status);
```

### 4.2 Repository (baru: `internal/repository/`)
- `jabatan_repository.go`: `List`, `GetByID`, `Create/Update` (admin), seed-aware.
- `sk_repository.go`: `Create`, `GetByID`, `List` (filter level/wilayah/status), `SetApproval`, `SetStatus`, `SetFile`.
- `pengurus_repository.go`: `Add` (transaksional + demote), `Remove` (guard final), `List` (filter + search), `ExistsInSK`, `JabatanTaken`.
  Aturan jabatan inti tunggal (`CountJabatanInSK`) hanya menghitung pemegang **efektif Aktif**; Demisioner/berakhir tidak memblokir penggantian (TDD §5.4).

### 4.3 Service (baru + ubah)
- Baru `sk_service.go` / `pengurus_service.go` (atau satu `kepengurusan_service.go`): implementasi §3.1–§3.3 termasuk flip `tipe`, cabut sesi (`RevokeAllUserTokens`), email notifikasi (reuse `MailSender`, async best-effort), audit tiap aksi.
- Ubah `anggota_service.go` / `user_repository.go`: tambah `SetTipe` bila belum ada (update `anggota.tipe` + `users.tipe_user` dalam transaksi pengangkatan).
- Upload file SK: reuse kategori storage `sk` (PDF 5 MB, PRIVATE) — tanpa kategori baru.

### 4.4 Handler + route (baru)
```
POST   /admin/sk                        (buat; Kab/Kota wilayahnya)
GET    /admin/sk                        (list + filter; scope wilayah)
GET    /admin/sk/:id                    (detail + daftar pengurus)
POST   /admin/sk/:id/approve            (teruskan/sahkan per rantai; catatan wajib bila tolak)
POST   /admin/sk/:id/pengurus           (angkat; Kab/Kota wilayah SK saja)
DELETE /admin/sk/:id/pengurus?pengurusId= (lepas; kunci bila final)
GET    /admin/pengurus                  (list + search nama/NIA)
PATCH  /admin/pengurus/:id              (nonaktifkan/ubah status; alasan wajib bila non-Aktif)
GET    /admin/jabatan                   (list master; CRUD dibatasi Super/Nasional)
```
Semua: `Authenticate` + `RequireRoles` + `ScopeWilayah` + `CanAccessWilayah` + audit. Rate-limit grup mutasi seperti `/admin/pendaftaran`.

### 4.5 Tidak berubah
- Generator NIA, KTA, storage presign, auth/token, wilayah read-only, revisi, tracking.

---

## 5. Perubahan Frontend (admin)

> **Revisi UX (B-P1…B-P6, 03 Okt 2026):** pengangkatan disatukan menjadi
> **satu wizard** `PromotePengurusWizard` (Stepper: **SK → Anggota → Jabatan →
> Konfirmasi**) yang dipakai di **tiga pintu masuk**: halaman Pengurus, detail
> SK (preset SK terkunci), dan detail Anggota (preset Anggota). Wizard menandai
> **jabatan inti yang sudah terisi pemegang AKTIF** (disabled, beserta nama
> pemegang; pemegang Demisioner/berakhir tidak memblokir — TDD §5.4), punya empty-state informatif,
> ringkasan efek (tipe KADER→PENGURUS, sesi dicabut, email), dan busy per-aksi.
> Aturan SK ditampilkan sebagai panduan (Buat → Susun Pengurus → Ajukan →
> Sahkan) beserta penjelasan *Single Active SK*.

- Halaman **SK**: daftar + filter, buat (nomor/judul/tanggal/file), detail + panel approval per rantai, **wizard Angkat Pengurus** (dipicu tombol di panel Susunan Pengurus, SK terpreset).
- Halaman **Pengurus**: daftar + filter/stats + **wizard Angkat Pengurus** (pilih SK lebih dulu).
- Halaman **Anggota detail**: tombol **"Jadikan Pengurus"** (tampil untuk KADER + admin) → wizard dengan anggota terpreset.
- Halaman **Jabatan** (master, Super/Nasional).
- Notifikasi sukses/gagal + loading per aksi (pola `Spinner`/`Overlay` existing).

---

## 6. Keamanan & Operasional

- Otorisasi ganda: `RequireRoles` (peran) + `ScopeWilayah`/`CanAccessWilayah` (yurisdiksi) + cek kepemilikan SK per aksi; Kab/Kota **tidak bisa** menyentuh level/wilayah lain (403).
- SK final **immutable** (tambah/hapus/edit ditolak; satu-satunya jalan = SK baru atau nonaktifkan).
- File SK: PDF tanpa password, maks 5 MB, PRIVATE, via presign (pola dokumen existing); `file_sk_key` wajib non-kosong sebelum tambah anggota.
- `tipe` flip + cabut sesi dalam **satu transaksi**; email notifikasi async best-effort (gagal kirim tidak menggagalkan pengangkatan).
- Audit: `CREATE/UPDATE/APPROVE/REJECT` untuk SK + pengurus + flip tipe (actor, IP, request-id); PII dimasking di metadata.
- Anti-duplikat: `UNIQUE(surat_keputusan_id, anggota_id)` di DB + cek aplikasi; jabatan inti dicek per SK.

---

## 7. Testing

1. **Unit**: guard SK final-lock; jabatan inti ganda; duplikat anggota per SK; wilayah mismatch 403; flip `tipe` + cabut sesi; email ter-render (tanpa kirim).
2. **Integrasi**: rantai KAB→PROV→NAS ujung-ke-ujung; angkat → demote lama → tipe berubah → login ulang diwajibkan; hapus sebelum/sesudah final.
3. **Regression**: `go test ./...`, `go vet`, `gofmt`; `npm run build` + `lint`; pentest suite hijau.

---

## 8. Tahapan Eksekusi (urutan)

1. **P0 (prasyarat, terpisah)**: hapus jalur daftar-pengurus sampai hijau (dokumen rencana penghapusan).
2. **P1**: migrasi `jabatan` + `surat_keputusan` + `pengurus` + seed jabatan.
3. **P2**: repository + service + handler + route SK/pengurus/jabatan (+ flip tipe + email + audit).
4. **P3**: UI admin SK/Pengurus/Jabatan.
5. **P4**: test + regression + update URD (bagian pengangkatan) + `AUTH_GUIDE` bila perlu.

---

## 9. Kriteria Selesai

- Admin Kab/Kota dapat buat SK (file wajib) → tambah kader (jabatan wajib) → kader menjadi PENGURUS (tipe berubah, sesi dicabut, email terkirim), semua teraudit.
- Provinsi/Nasional/Kab lain ditolak (403) di luar yurisdiksinya; SK final terkunci.
