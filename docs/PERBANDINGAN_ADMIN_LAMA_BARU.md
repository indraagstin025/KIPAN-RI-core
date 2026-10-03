# Perbandingan Fitur Admin: KIPAN_INDONESIA (Lama) vs SIM-KIPAN Core (Baru)

> Status: dokumen analisis (perbandingan fitur admin).
> Tanggal: 03 Oktober 2026
> Ruang lingkup: modul **admin** saja. **CMS publikasi (Berita/Galeri/Program) sengaja dilewati.**
> Sumber lama: `KIPAN_INDONESIA/` (`src/lib/admin-menu.ts`, `src/components/admin/pages/*`, `prisma/seed-users.ts`, `src/app/api/dashboard/route.ts`).
> Sumber baru: `backend/` + `frontend/`.

---

## 1. Ringkasan Stack

| Aspek | Lama (`KIPAN_INDONESIA`) | Baru (`backend` + `frontend`) |
|---|---|---|
| Frontend | Next.js (App Router) + Tailwind + Framer Motion | React 19 + Vite + Tailwind v4 |
| Backend | API Routes Next.js | Go Fiber v2 (REST `/api/v1`) |
| ORM/DB | Prisma + PostgreSQL | `sqlx` + PostgreSQL (`kipan_core`) |
| Migrasi | Prisma migrate | golang-migrate (bernomor `0000NN`) |
| Auth | bcrypt; login **username** (mis. `superadminkipan`), akun uji password default `123`; akses admin via `#admin` / Ctrl+Shift+A | Argon2id; login **email**; admin via shortcut (Ctrl+A+I); JWT + refresh token rotation |
| Scope wilayah | filter by `role` + `wilayah` di API | `ScopeWilayah`/`CanAccessWilayah` server-side dari JWT |
| Storage | lokal/DB | MinIO/S3 presign (public/private/uploads) |
| WhatsApp | – | Fonnte (OTP pendaftaran) |
| Email | – | SMTP (notifikasi status, token revisi, pengangkatan) |
| KTA | cetak (desain lama) | PDF server-side + QR bertanda tangan HMAC + halaman verifikasi publik |

---

## 2. Matriks Peran

Peran sama (4 tingkat admin) + tambahan `USER` di project baru.

| Peran | Kemampuan (Lama) | Kemampuan (Baru) |
|---|---|---|
| **SUPER_ADMIN** | Akses penuh; eksklusif: Role & Wewenang, Manajemen User, Profil Organisasi, Database Backup | Akses penuh; eksklusif master jabatan; (UI User/Role/Backup belum ada) |
| **ADMIN_NASIONAL** | Operasional nasional + master wilayah/jabatan + program + statistik | Nasional: verifikasi, anggota, SK (sahkan final), pengurus, jabatan |
| **ADMIN_PROVINSI** | Anggota/pengurus/SK/laporan (lingkup provinsi) | **Lihat** antrean provinsi + **teruskan** SK; **tidak boleh** verifikasi/setujui pendaftaran (§2A) |
| **ADMIN_KABUPATEN** | Anggota/pengurus/pendaftaran/verifikasi/SK (lingkup kab/kota) | Kab/Kota: **buat SK + angkat pengurus**, anggota, lihat SK |
| **USER (KADER/PENGURUS)** | – | Baru: akun mandiri, KTA sendiri (`/user/kta`), status KADER→PENGURUS saat diangkat |

> Catatan lama: menu Pendaftaran/Verifikasi **tidak** tersedia untuk ADMIN_PROVINSI (gate di `admin-menu.ts`), meski deskripsi `RolePage` menyebut sebaliknya — gate menu yang berlaku. Di project baru ini dipertegas di backend (Provinsi ditolak 422 untuk aksi verifikasi).

---

## 3. Status Modul Admin (non-CMS)

Legenda: **Sudah** = ada & fungsional · **Sebagian** = ada tapi belum selengkap lama · **Belum** = belum ada.

| Modul (lama) | Deskripsi lama | Status baru | Catatan |
|---|---|---|---|
| Dashboard | Stat cards, grafik, tren bulanan, aksi cepat, drilldown wilayah | **Sudah (analitik)** | Baru: `/admin/dashboard` ter-scope + stat cards, tren, distribusi, terbaru, aksi cepat |
| Master Wilayah | CRUD provinsi/kabupaten | **Belum** | Baru: read-only dropdown (proxy `wilayah.id`) + seed 38 prov/514 kab |
| Data Anggota | List/detail/edit, unduh KTA, reset password | **Sebagian** | Baru: list/detail, unduh KTA, reset password (belum edit field anggota) |
| Pengurus | CRUD susunan pengurus | **Sudah** | Baru: lewat SK (angkat/lepas + ubah status) |
| Pendaftaran Baru | List pendaftar + tindak lanjut | **Sudah** | Antrean + detail |
| Verifikasi Anggota | Setujui/tolak/minta perbaikan | **Sudah** | + reveal NIK, checklist persyaratan, KTA otomatis |
| Surat Keputusan | SK + approval | **Sudah (lebih baik)** | Baru: rantai KAB→PROV→NAS + pengangkatan otomatis (flip tipe, cabut sesi, email) |
| Jabatan | Master jabatan CRUD | **Sudah** | List + CRUD (Nasional/Super) |
| Statistik | Grafik keanggotaan | **Belum** | – |
| Cetak Laporan | Export/cetak laporan | **Belum** | – |
| Manajemen User | CRUD akun admin | **Belum (UI)** | Baru: provisioning via seeder (`cmd/seed`, mode `-all-regions`) |
| Role & Wewenang | Halaman matriks + kelola | **Sebagian** | RBAC ditegakkan server-side; halaman matriks belum ada |
| Profil Organisasi | Kelola profil organisasi | **Belum** | – |
| Database Backup | Backup/restore | **Belum** | – |
| Akun Saya | Profil + ganti password | **Sebagian** | Baru: ganti kata sandi (`/akun/password`) |

---

## 4. Isi Dashboard: Lama vs Baru

| Elemen | Lama | Baru |
|---|:-:|:-:|
| Kartu statistik (Total/Aktif Anggota, Menunggu Verifikasi, Anggota Baru) | ✅ | ✅ |
| Kartu statistik wilayah (Provinsi/Kabupaten/Coverage) | ✅ | ✅ (Provinsi/Kabupaten) |
| Grafik distribusi status + tren pendaftaran bulanan | ✅ | ✅ (bar) |
| Distribusi per wilayah + drilldown provinsi→kabupaten | ✅ | ✅ distribusi per level (drilldown belum) |
| Aksi cepat (ke Verifikasi/Pendaftaran/Pengurus/Berita) | ✅ | ✅ (Verifikasi/Anggota/SK/Pengurus) |
| Scope data otomatis per role/wilayah (`/api/dashboard?role=&wilayah=`) | ✅ | ✅ (`/admin/dashboard`, scope dari JWT) |
| Auto-refresh berkala | ✅ (60s) | ❌ |
| Lonceng notifikasi (baca satu/semua) | ✅ | ✅ |
| Cakupan wilayah + uji RBAC | implisit | ✅ (uji RBAC dev dipindah/hapus dari dashboard) |
| Status layanan (health) | ❌ | ✅ |

---

## 5. Fitur Baru (tidak ada di project lama)

- **OTP WhatsApp** pada submit pendaftaran (anti-bot/spam).
- **KTA digital**: PDF dirender server-side + QR bertanda tangan HMAC + halaman verifikasi publik (`/verifikasi-kta`).
- **Pengangkatan kader → pengurus via SK berjenjang** (KAB buat → PROV teruskan → NAS sahkan) dengan efek otomatis: `anggota.tipe` + `users.tipe_user` → PENGURUS, cabut sesi, email notifikasi.
- **Tracking & revisi mandiri** (token revisi via email, nomor pendaftaran).
- **Notifikasi email** (status pendaftaran: PERBAIKAN/DISETUJUI/DITOLAK, pengangkatan).
- **Wilayah nasional** 38 provinsi + 514 kab/kota (master + dropdown kecamatan/desa/kodepos).
- **Hardening keamanan** (audit OWASP S1–S9): rate limiter, refresh token rotation + reuse detection, validasi dokumen berlapis, guard XSS wilayah, dsb.
- **Arsitektur terpisah** backend Go + frontend React (lama monolitik Next.js).

---

## 6. Backlog / Gap (non-CMS) — Prioritas

| Prioritas | Item | Nilai |
|---|:--|:--|
| **P1** | **Dashboard analitik**: stat cards + tren + aksi cepat (+ endpoint agregat ber-scope) — **SELESAI** | Nilai cepat untuk admin; fondasi modul statistik |
| **P1** | **Cetak/Export Laporan** (anggota, antrean, SK/pengurus) | Kebutuhan operasional rutin |
| **P2** | **Manajemen User & Role** (UI) + **edit data anggota** | Administrasi akun & koreksi data |
| **P2** | **Statistik** (grafik per wilayah/status) | Monitoring keanggotaan |
| **P3** | **Profil Organisasi**, **Database Backup**, **Master Wilayah CRUD** | Pelengkap; sebagian bisa dibatasi Super/Nasional |

---

## 7. Catatan Keamanan & Kepatuhan

- Otorisasi project baru berlapis: **peran (route) + yurisdiksi (`CanAccessWilayah`) + aturan domain (rantai SK)**, ditegakkan di backend (bukan hanya menyembunyikan tombol di UI).
- Admin Provinsi **tidak** boleh aksi verifikasi pendaftaran (melengkapi celah pada project lama).
- Semua aksi sensitif tercatat di `activity_logs` (append-only).
- KTA diverifikasi kriptografis (HMAC), bukan hanya "NIA ada di DB".

---

## 8. Kesimpulan

- **Sudah setara/lebih baik di project baru**: Pendaftaran, Verifikasi, Anggota (inti), SK & Pengurus, Jabatan, Auth/Keamanan, plus fitur baru (OTP, KTA digital, pengangkatan berjenjang, email).
- **Belum diport (non-CMS)**: Dashboard analitik, Statistik, Cetak Laporan, Manajemen User & Role (UI), Profil Organisasi, Database Backup, Master Wilayah CRUD, edit data anggota.
- **CMS publikasi** (Berita/Galeri/Program) di luar lingkup dokumen ini (sesuai keputusan: di-skip dulu).
