# 🗺️ ROADMAP & TAHAPAN PENGERJAAN CORE SIM-KIPAN
## Panduan Migrasi Komprehensif dari Next.js ke Go Fiber + React SPA

> **Tujuan Dokumen**: Memetakan seluruh kompleksitas backend dari proyek lama (`KIPAN_INDONESIA/src/app`) ke dalam tahapan pengerjaan (fase) yang terstruktur, terukur, dan aman.

---

## 🔍 ANALISIS KOMPLEKSITAS DARI PROJECT LAMA (NEXT.JS)

Setelah membedah seluruh rute API dan skema Prisma di `KIPAN_INDONESIA/src/app/api`:

```
KIPAN_INDONESIA/src/app/api/
├── activity-log/               -> Audit trail setiap aksi CRUD
├── anggota/                    -> Pangkalan data kader + cek status
├── auth/                       -> Login + ganti password
├── berita/                     -> CMS berita berjenjang (Nasional/Prov/Kab)
├── dashboard/                  -> Agregasi statistik admin
├── galeri/                     -> Dokumentasi kegiatan
├── jabatan/                    -> Master data posisi struktural
├── notifications/              -> Notifikasi in-app per user admin
├── pendaftaran/                -> Alur registrasi, tracking, perbaikan berkas, verifikasi
├── pengurus/                   -> SK pengangkatan, promosi, demisioner, ganti jabatan
├── profil/                     -> Pengaturan akun admin
├── program/                    -> Rencana kerja KIPAN (Direncanakan/Berjalan/Selesai)
├── surat-keputusan/            -> Alur draf SK, approval berjenjang, nonaktifkan SK lama
├── upload/                     -> Penanganan berkas identitas
├── users/                      -> Manajemen akun pengurus/admin
└── wilayah/                    -> Hirarki BPS, kodepos, status keaktifan cabang
```

### Mengapa Versi Next.js Sebelumnya Terasa Kompleks & Berat?
1. **Server Actions & Route Handlers bercampur**: Logika bisnis, enkripsi, dan mutasi database bercampur dengan komponen UI.
2. **Ketergantungan State Kepengurusan**: Entitas `Pengurus` sangat bergantung pada `SuratKeputusan` dan `Anggota`. Jika ada SK baru, pengurus lama harus otomatis didemisionerkan tanpa merusak riwayat masa bakti.
3. **Scoping Wilayah Multilevel**: Akses dibatasi berdasarkan 4 tingkatan (Super Admin, Nasional, Provinsi, Kab/Kota) di hampir setiap tabel (`pendaftaran`, `anggota`, `sk`, `pengurus`, `berita`).

---

## 📋 ROADMAP PENGERJAAN BERTAHAP (6 FASE)

```
[FASE 0: Fondasi & Kriptografi]  -->  SELESAI (Monorepo, DB Migrations, Go+React Base)
            ↓
[FASE 1: Membership & Pendaftaran] ->  Fokus Saat Ini (Alur Berkas, Verifikasi, KTA)
            ↓
[FASE 2: Tata Kelola SK & Pengurus] -> Governance Engine (SK, Jabatan, Promosi, Demisioner)
            ↓
[FASE 3: CMS & Informasi Publik]   -> Berita, Galeri Kegiatan, Program Kerja
            ↓
[FASE 4: Audit Trail & Notifikasi] -> Activity Log Imutabel & Notifikasi Verifikator
            ↓
[FASE 5: Hardening, QA & Deploy]   -> Caddy, IDCloudHost, S3 Migration, CI/CD
```

---

### FASE 0: FONDASI ARSITEKTUR & KEAMANAN KRIPTOGRAFI
> **Status**: ✅ **SELESAI**

- [x] Inisialisasi Monorepo (`backend/` Go Fiber + `frontend/` React 19 SPA)
- [x] Isolasi proyek lama `KIPAN_INDONESIA` via `.gitignore`
- [x] Docker Compose lokal (PostgreSQL 16, Redis 7, MinIO, Mailpit)
- [x] Modul Kriptografi:
  - Enkripsi AES-256-GCM untuk data NIK (Kepatuhan UU PDP)
  - HMAC-SHA256 Blind Index untuk deteksi NIK ganda tanpa membuka kunci
  - Password hashing Argon2id
  - Generator & Validator Nomor Induk Anggota (NIA) resmi BPS
- [x] Migrasi Skema DDL Database PostgreSQL & Seed 38 Provinsi BPS

---

### FASE 1: CORE MEMBERSHIP ENGINE (PENDAFTARAN & KADER)
> **Fokus Utama**: Menyelesaikan seluruh siklus hidup calon anggota hingga menjadi kader ber-NIA.

#### Backend (Go Fiber):
1. **Endpoint Pendaftaran Mandiri**:
   - Validasi data & NIK via Blind Index (mencegah pendaftaran ganda).
   - Upload berkas identitas (KTP, pasfoto, surat bebas narkoba) ke storage (R2/IS3).
   - Generate nomor registrasi `REG-YYYYMM-XXXXX`.
2. **Endpoint Pelacakan & Perbaikan Berkas**:
   - Lacak status publik berdasarkan nomor registrasi.
   - Endpoint perbaikan berkas (`/pendaftaran/perbaikan/:nomor`) dengan verifikasi token revisi.
3. **Verifikasi Berjenjang oleh Admin**:
   - Filter antrean otomatis berdasarkan cakupan wilayah admin (`ADMIN_KABUPATEN` hanya melihat kab miliknya).
   - Transisi status: `DIAJUKAN` → `DIVERIFIKASI` → `PERBAIKAN` / `DITOLAK` / `DISETUJUI`.
   - Jika `DISETUJUI`:
     - Otomatis hitung sequence nomor urut wilayah.
     - Terbitkan NIA format `KIPAN.{prov}.{kab}.{tahun}.{seq}`.
     - Buat HMAC signature anti-pemalsuan QR Code KTA.
4. **Validasi KTA Digital**:
   - Endpoint publik verifikasi scan QR Code KTA (`/api/v1/public/kta/verifikasi`).

#### Frontend (React SPA):
- [x] Halaman Formulir Pendaftaran multi-step.
- [x] Halaman Lacak Status dengan visualisasi timeline riwayat.
- [x] Halaman Verifikasi KTA Publik.
- [x] Portal Admin: Antrean Pendaftaran & Verifikasi Berkas.
- [ ] *Next*: Fitur unduh / cetak fisik KTA Digital (PDF card generator).

---

### FASE 2: TATA KELOLA SK & KEPENGURUSAN (GOVERNANCE ENGINE)
> **Fokus Utama**: Menggantikan logika kompleks SK & Pengurus yang sebelumnya ada di Next.js.

#### Fitur yang Dikerjakan:
1. **Master Jabatan & Urutan Hierarki**:
   - Referensi jabatan (Ketua, Sekretaris, Bendahara, Bidang) per tingkatan (Nasional, Provinsi, Kab/Kota).
2. **Manajemen Surat Keputusan (SK)**:
   - Draf pengajuan SK Kepengurusan baru.
   - Alur persetujuan SK: `DRAFT` → `MENUNGGU_PROVINSI` → `MENUNGGU_NASIONAL` → `DISETUJUI` / `DITOLAK`.
   - Upload naskah SK resmi (PDF).
3. **Pengangkatan Pengurus**:
   - Memilih anggota ber-NIA untuk masuk ke dalam susunan kepengurusan SK.
4. **Logika Demisioner Otomatis**:
   - Saat SK baru diterbitkan untuk wilayah X, kepengurusan dari SK lama otomatis berubah status menjadi `Demisioner` dan tanggal selesai dicatat.
5. **Promosi Kader & Mutasi Jabatan**:
   - Fitur ganti jabatan dalam satu periode.
   - Pindah wilayah/promosi dari DPC ke DPD/DPP (`list-promosi`).
6. **Indikator Keaktifan Wilayah**:
   - Status wilayah: `Aktif` (ada SK berlaku), `Vakum` (SK habis masa berlaku), `Belum Terbentuk`.

---

### FASE 3: CMS PUBLIK & INFORMASI ORGANISASI
> **Fokus Utama**: Portal publik untuk transparansi kegiatan KIPAN.

#### Fitur yang Dikerjakan:
1. **CMS Berita & Opini**:
   - Jenis: `UMUM` (tampil di web publik) vs `INTERNAL` (khusus portal kader).
   - Kategori berjenjang: Nasional, Provinsi, Kabupaten/Kota.
   - Editor konten berita, upload gambar sampul (WebP auto-optimize), tagar, slug unik.
2. **Galeri Dokumentasi Kegiatan**:
   - Album kegiatan sosialisasi bahaya narkoba, pelatihan kader, jambore, rapat kerja.
   - Tagging wilayah pelaksanaan.
3. **Program Kerja & Agenda**:
   - Rencana program DPP, DPD, DPC.
   - Status program: `Direncanakan`, `Sedang Berjalan`, `Selesai`.
4. **Profil Organisasi & Kontak**:
   - Visi, Misi, Sejarah, Struktur DPP KIPAN RI, informasi kontak sekretariat.

---

### FASE 4: AUDIT LOGS, OBSERVABILITY & NOTIFIKASI
> **Fokus Utama**: Jejak audit kepatuhan & kenyamanan admin verifikator.

#### Fitur yang Dikerjakan:
1. **Activity Log Imutabel**:
   - Mencatat setiap perubahan penting (tabel, ID record, aksi, snapshot data lama & data baru, IP, User Agent).
   - Halaman audit log khusus Super Admin untuk inspeksi keamanan.
2. **Sistem Notifikasi In-App**:
   - Lonceng notifikasi di dashboard admin:
     - Notifikasi saat ada pendaftaran calon kader baru di wilayahnya.
     - Notifikasi saat ada draf SK yang perlu disahkan.
     - Notifikasi saat pengajuan perbaikan berkas diunggah ulang.
3. **Integrasi Gateway Eksternal (Opsional)**:
   - Notifikasi WhatsApp / Email otomatis ke pendaftar saat status berkas disetujui / perlu perbaikan.

---

### FASE 5: TESTING, HARDENING & DEPLOYMENT IDCLOUDHOST
> **Fokus Utama**: Memastikan sistem siap pakai di server produksi.

#### Fitur yang Dikerjakan:
1. **Testing**:
   - Unit test logika kriptografi & penomoran NIA.
   - API contract testing & load testing sederhana.
2. **Konfigurasi Produksi**:
   - Caddy v2 Reverse Proxy (Auto SSL Let''s Encrypt).
   - Setup migration script `golang-migrate` otomatis di server VPS.
   - Bucket sync Cloudflare R2 / IDCloudHost IS3.
3. **CI/CD Pipeline**:
   - GitHub Actions pipeline dengan path-filter (`backend/**` deploy ke VPS via SSH, `frontend/**` deploy ke Cloudflare Pages/CDN).

---

## 🎯 REKOMENDASI URUTAN EKSEKUSI KITA

Agar pengerjaan rapi dan tidak membingungkan:

| Urutan | Modul | Luaran Nyata |
|:---:|:---|:---|
| **Langkah 1** | **Finishing Fase 1 (Membership)** | Cetak KTA PDF digital + revisi berkas via nomor tracking |
| **Langkah 2** | **Fase 2 (SK & Kepengurusan)** | Modul Draf SK, Pengangkatan Pengurus, dan Logika Demisioner |
| **Langkah 3** | **Fase 3 (CMS Publik)** | Modul Berita, Galeri Kegiatan, dan Program Kerja |
| **Langkah 4** | **Fase 4 (Audit & Notifikasi)** | Activity Log dan Notification Center |
| **Langkah 5** | **Fase 5 (Deploy & Go-Live)** | Deployment ke VPS IDCloudHost & Cloudflare |
