# 📋 PANDUAN TAHAPAN PENGERJAAN & CHECKLIST FITUR SIM-KIPAN CORE
## Rencana Aksi Eksekusi Bertahap (Step-by-Step Task Tracker)

> **Tujuan Dokumen**: Menjadi panduan kerja teknis harian agar pengerjaan kode terarah, tidak ada fitur yang tumpang tindih, dan setiap langkah memiliki kriteria penyelesaian (*acceptance criteria*) yang jelas dan teruji.
> **Branch Pengerjaan Saat Ini**: `backend-fase-1-authentication-authorization`

---

## 🧭 PETA KEMAJUAN PROYEK (STATUS DASHBOARD)

| Fase | Nama Modul / Area Kerja | Status | Output Utama |
|:---:|:---|:---:|:---|
| **Fase 1** | Fondasi, Kriptografi, Database, Autentikasi & Otorisasi Core | ✅ **SELESAI** | Monorepo, PostgreSQL 16, AES-256-GCM, Argon2id, JWT, HttpOnly Cookie RTR, RBAC Scope Wilayah, Pentest Suite v2.1 (20/20 PASS) |
| **Fase 2** | **Core Membership Engine (Pendaftaran & Kader)** | 🎯 **SEDANG BERJALAN** | Pendaftaran Mandiri, NIK Blind Index, Verifikasi Berjenjang, Generator NIA & KTA |
| **Fase 3** | Tata Kelola Surat Keputusan (SK) & Pengurus | ⏳ Antrean | Draf SK, Alur Pengesahan, Mutasi Jabatan, Otomasi Demisioner |
| **Fase 4** | CMS Publik & Informasi Organisasi | ⏳ Antrean | Berita Berjenjang, Galeri Dokumentasi, Program Kerja |
| **Fase 5** | Audit Trail & Notifikasi In-App | ⏳ Antrean | Activity Log Imutabel & Notifikasi Verifikator |
| **Fase 6** | Hardening, Storage S3 & Deploy Produksi | ⏳ Antrean | Caddy Auto-SSL, Cloudflare/IDCloudHost, CI/CD Actions |

---

## 🎯 TAHAPAN DETAIL FASE 2: CORE MEMBERSHIP ENGINE

Tahapan ini berfokus menyelesaikan **seluruh siklus hidup pendaftaran calon anggota hingga resmi menjadi kader ber-Nomor Induk Anggota (NIA) dan memiliki kartu KTA digital**.

```
[CALON ANGGOTA]                             [ADMIN DPC / DPD]
      │                                             │
      ▼                                             │
1. Isi Formulir Mandiri ──(Upload KTP & Foto)──► Simpan (Status: DIAJUKAN)
      │                                             │
      ▼                                             ▼
2. Dapatkan No. Registrasi                   3. Antrean Masuk (Auto-filter Kab/Prov)
   (REG-YYYYMM-XXXXX)                               │
      │                                             ▼
      ▼                                      4. Verifikasi Berkas Fisik/Identitas
   Lacak Status Mandiri                             │
   (/pendaftaran/lacak)                             ├─► Butuh Revisi ➔ Status: PERBAIKAN
      │                                             ├─► Tidak Sesuai ➔ Status: DITOLAK
      │                                             └─► Memenuhi Syarat ➔ Status: DISETUJUI
      │                                                       │
      ▼                                                       ▼
5. Jika PERBAIKAN:                           5. Terbitkan Kader Resmi:
   Unggah ulang berkas revisi                   - Insert ke tabel `anggota`
                                                - Generate Sequence NIA Nasional
                                                - Generate QR Signature KTA Digital
```

---

### SPRINT 1.1: Persiapan Domain, DTO, & Generator Nomor Registrasi
- [ ] **Task 1.1.1 — Definisi DTO & Model Pendaftaran**:
  - Berkas: `backend/internal/domain/pendaftaran.go`
  - Struct `PendaftaranCreateRequest`: Nama lengkap, NIK, tempat/tgl lahir, jenis kelamin, alamat lengkap, provinsi_id, kabupaten_id, kecamatan, desa, email, whatsapp, motivasi.
  - Struct `PendaftaranResponse`, `PendaftaranTrackingResponse`, dan `PendaftaranDetailResponse`.
  - Aturan validasi validator tag: format email, nomor WA Indonesia (`numeric,min=10,max=15`), NIK 16 digit angka.
- [ ] **Task 1.1.2 — Generator Nomor Registrasi Unik**:
  - Berkas: `backend/pkg/generator/registration.go`
  - Format: `REG-YYYYMM-XXXXX` (contoh: `REG-202609-00001`).
  - Berbasis sequence database atomik agar tidak ada race condition saat ribuan orang mendaftar bersamaan.
- [ ] **Task 1.1.3 — Service Abstraksi Penyimpanan Berkas (Upload)**:
  - Berkas: `backend/internal/service/storage_service.go`
  - Validasi tipe MIME: hanya izinkan `image/jpeg`, `image/png`, `application/pdf`.
  - Validasi ukuran maksimum: Foto (maks 2MB), KTP (maks 2MB), Surat Bebas Narkoba (maks 5MB).
  - Sanitasi nama file dan pembuatan path unik berbasis UUID: `uploads/pendaftaran/{uuid}-{safe_name}.ext`.

---

### SPRINT 1.2: Repository & Keamanan Enkripsi NIK Pendaftaran
- [ ] **Task 1.2.1 — Query Repository Pendaftaran Mandiri**:
  - Berkas: `backend/internal/repository/pendaftaran_repository.go`
  - Method `Create(ctx, pendaftaran)`: Menggunakan parameterized raw SQL dengan sqlx.
  - Method `FindByRegistrationNumber(ctx, noReg)`: Pencarian publik berdasarkan nomor registrasi.
  - Method `ExistsByNIKBlindIndex(ctx, blindIndex)`: Pengecekan kilat apakah NIK sudah pernah terdaftar tanpa perlu dekripsi database.
- [ ] **Task 1.2.2 — Integrasi Kriptografi UU PDP pada Service Pendaftaran**:
  - Berkas: `backend/internal/service/pendaftaran_service.go`
  - Hitung HMAC-SHA256 Blind Index dari NIK calon anggota ➔ cek duplikasi di database. Jika sudah ada, kembalikan error `CONFLICT: NIK sudah terdaftar`.
  - Enkripsi nilai asli NIK dengan AES-256-GCM menggunakan `AESMasterKey` sebelum SQL insert.
  - Simpan riwayat pertama ke tabel `pendaftaran_riwayat` (`Aksi: DIAJUKAN`, `Catatan: Pendaftaran mandiri diterima`).

---

### SPRINT 1.3: Endpoint Publik Pendaftaran & Pelacakan Status
- [ ] **Task 1.3.1 — Handler Pendaftaran Mandiri**:
  - Endpoint: `POST /api/v1/public/pendaftaran`
  - Berkas: `backend/internal/handler/pendaftaran_public_handler.go`
  - Menangani form multipart: data diri + berkas upload (KTP, pasfoto, dll).
  - Mengembalikan respon standar dengan data `nomor_registrasi`, `status: DIAJUKAN`, dan estimasi waktu verifikasi.
- [ ] **Task 1.3.2 — Handler Lacak Status Pendaftaran**:
  - Endpoint: `GET /api/v1/public/pendaftaran/lacak/:nomor_registrasi`
  - Menampilkan timeline transisi berkas, tanggal pengajuan, status saat ini, dan catatan perbaikan (jika ada).
  - Masking data pribadi: Sensor NIK (`3273************`), sembunyikan nomor HP/email untuk keamanan privasi publik.
- [ ] **Task 1.3.3 — Handler Perbaikan Berkas Calon Anggota**:
  - Endpoint: `PUT /api/v1/public/pendaftaran/perbaikan/:nomor_registrasi`
  - Hanya dapat diakses jika status pendaftaran adalah `PERBAIKAN`.
  - Mengunggah berkas revisi baru ➔ catat riwayat ➔ ubah status kembali menjadi `DIAJUKAN`.

---

### SPRINT 1.4: Modul Verifikasi Berjenjang oleh Admin (Admin Portal)
- [ ] **Task 1.4.1 — Queue Antrean Pendaftaran Berdasarkan Yurisdiksi Wilayah**:
  - Endpoint: `GET /api/v1/admin/pendaftaran`
  - Berkas: `backend/internal/handler/pendaftaran_admin_handler.go`
  - Otomatis memanfaatkan `c.Locals("wilayah_scope")` dari middleware RBAC:
    - `ADMIN_KABUPATEN`: Hanya bisa melihat antrean pendaftaran di kabupatennya sendiri.
    - `ADMIN_PROVINSI`: Melihat antrean di seluruh kabupaten dalam provinsinya.
    - `ADMIN_NASIONAL` & `SUPER_ADMIN`: Melihat seluruh antrean se-Indonesia.
  - Dilengkapi fitur pagination, filter status (`DIAJUKAN`, `PERBAIKAN`, `DISETUJUI`), dan pencarian nama.
- [ ] **Task 1.4.2 — Detail Berkas Pendaftar untuk Admin Verifikator**:
  - Endpoint: `GET /api/v1/admin/pendaftaran/:id`
  - Menyediakan link berkas dokumen yang aman.
  - Menampilkan dekripsi NIK khusus untuk admin berwenang demi verifikasi kesesuaian KTP.
- [ ] **Task 1.4.3 — Aksi Verifikasi & Transisi Status**:
  - Endpoint: `PUT /api/v1/admin/pendaftaran/:id/verifikasi`
  - Pilihan aksi verifikator:
    1. **`MINTA_PERBAIKAN`**: Wajib mengisi field `catatan_perbaikan` (misal: "Foto KTP buram"). Status berubah jadi `PERBAIKAN`.
    2. **`TOLAK`**: Wajib mengisi alasan penolakan. Status berubah jadi `DITOLAK`.
    3. **`SETUJUI`**: Lolos verifikasi administrasi. Memulai proses penerbitan kader.

---

### SPRINT 1.5: Generator NIA Otomatis, Penerbitan Anggota & KTA Digital
- [ ] **Task 1.5.1 — Algoritma Penomoran NIA (Nomor Induk Anggota)**:
  - Berkas: `backend/pkg/nia/generator.go`
  - Format standar resmi BPS KIPAN: `KIPAN.{kode_prov}.{kode_kab}.{tahun_angkat}.{nomor_urut_5_digit}`
  - Contoh: `KIPAN.32.73.2026.00142` (Jawa Barat - Kota Bandung - Tahun 2026 - Urutan ke-142).
  - Mutex / Sequence Lock database untuk menjamin nomor urut tidak pernah loncat atau dobel (*gapless sequence*).
- [ ] **Task 1.5.2 — Penerbitan Record Anggota (Kader)**:
  - Saat pendaftaran `DISETUJUI`:
    - Simpan data kader baru ke tabel `anggota`.
    - Set status keanggotaan `Aktif`.
    - Catat relasi `pendaftaran.anggota_id = anggota.id`.
- [ ] **Task 1.5.3 — Signature Anti-Pemalsuan QR Code KTA**:
  - Berkas: `backend/pkg/crypto/kta_signature.go`
  - Payload QR: `nia:nama:tgl_angkat:prov_id:kab_id`
  - Generate Signature HMAC-SHA256 menggunakan `KTASigningKey`.
- [ ] **Task 1.5.4 — Endpoint Verifikasi Keaslian KTA Publik**:
  - Endpoint: `GET /api/v1/public/kta/verifikasi/:nia`
  - Menerima scan QR Code dari smartphone publik/aparat:
    - Menampilkan informasi keaslian kartu: Foto resmi, Nama, NIA, Wilayah DPD/DPC, Status Keaktifan.
    - Menolak kartu jika signature tidak valid atau nomor NIA tidak terdaftar.

---

### SPRINT 1.6: Automated Testing & Verifikasi Keamanan Pendaftaran
- [ ] **Task 1.6.1 — Penulisan Test Suite Pendaftaran (`backend/scripts/pendaftaran_test.ps1`)**:
  - Uji pencegahan duplikasi NIK (Blind index test).
  - Uji upload file ilegal (.exe, .php, script bypass).
  - Uji kebocoran yurisdiksi (Admin Kab B tidak bisa menyetujui pendaftar Kab A).
  - Uji siklus lengkap: `Daftar` ➔ `Perbaikan` ➔ `Setujui` ➔ `Terbit NIA` ➔ `Verifikasi KTA QR`.

---

## 📅 RANGKUMAN ROADMAP FASE BERIKUTNYA

```
┌─────────────────────────────────────────────────────────────────────────────┐
│ FASE 3: TATA KELOLA SURAT KEPUTUSAN (SK) & KEPENGURUSAN                    │
├─────────────────────────────────────────────────────────────────────────────┤
│ • Master Jabatan & Hirarki Organisasi (Ketua, Sekretaris, Bendahara, Bidang)│
│ • Siklus Draf SK Kepengurusan Baru (Upload Naskah SK PDF)                   │
│ • Approval Berjenjang: Draf DPC ➔ Disetujui DPD ➔ Disahkan DPP              │
│ • Logika Demisioner Otomatis: SK baru terbit ➔ SK lama otomatis demisioner   │
│ • Riwayat Mutasi, Rotasi Jabatan, dan Pelacakan Masa Bakti Pengurus         │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ FASE 4: CMS PUBLIK & INFORMASI ORGANISASI                                   │
├─────────────────────────────────────────────────────────────────────────────┤
│ • Manajemen Berita & Opini Berjenjang (Publik vs Internal Portal Kader)     │
│ • Galeri Dokumentasi Kegiatan Anti-Narkoba (Sosialisasi, Bimtek, Pelantikan)│
│ • Program Kerja & Agenda (Direncanakan, Berjalan, Selesai)                  │
│ • Profil Organisasi DPP, DPD, DPC, dan Kontak Sekretariat Resmi             │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ FASE 5: AUDIT TRAIL, OBSERVABILITY & NOTIFIKASI                             │
├─────────────────────────────────────────────────────────────────────────────┤
│ • Activity Log Imutabel (Snapshot Perubahan Data, Aktor, IP, Timestamp)     │
│ • Lonceng Notifikasi In-App Verifikator saat Berkas Masuk / Butuh Approval  │
│ • Notifikasi Eksternal (Opsional: Integrasi WhatsApp Gateway / Email)       │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ FASE 6: TESTING KOMPREHENSIF, QA & DEPLOYMENT IDCLOUDHOST                  │
├─────────────────────────────────────────────────────────────────────────────┤
│ • Setup Reverse Proxy Caddy v2 (Otomatis SSL Let's Encrypt)                 │
│ • Migrasi Berkas Identitas ke Penyimpanan Kompatibel S3                     │
│ • Setup Pipeline GitHub Actions CI/CD ke Server VPS                         │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 📌 ATURAN EKSEKUSI PENGERJAAN
1. **Satu Sprint Selesai ➔ Uji ➔ Commit**: Selesaikan per sprint kecil, pastikan test lulus, lalu commit dengan format Conventional Commits.
2. **Keamanan Data Pribadi (UU PDP)**: NIK **TIDAK BOLEH** disimpan dalam bentuk plain text di database atau terekspos di respon JSON publik. Selalu gunakan enkripsi AES-256-GCM dan HMAC Blind Index.
3. **Validasi Wilayah Ketat**: Seluruh endpoint admin pendaftaran wajib melewati middleware `RequireWilayahScope()`.
