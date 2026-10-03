# 🔐 PANDUAN PENGUJIAN MODUL AUTENTIKASI & OTORISASI (SIM-KIPAN CORE)

Backend saat ini **100% fokus dan terisolasi khusus pada subsistem Autentikasi dan Otorisasi (RBAC & Scoping Wilayah)**.

---

## 🔑 AKUN PENGUJIAN 5 LEVEL (SEEDING MANUAL)

> **Tidak ada lagi password default di repository.** Migrasi database
> (`000002_seed_initial_data.up.sql`) hanya berisi master wilayah; akun admin
> **tidak lagi** dibuat oleh migrasi. Ini menutup temuan CRITICAL audit Fase 1:
> kredensial produksi yang bocor publik (RULES #9), karena `make migrate-up`
> juga dijalankan di server produksi.

Akun admin dibuat lewat seeder yang **wajib** diberi password dari environment:

```bash
cd backend
export SEED_ADMIN_PASSWORD=''<Password-Kuat-Anda-2026!>''   # bash
# PowerShell: $env:SEED_ADMIN_PASSWORD=''<Password-Kuat-Anda-2026!>''
go run ./cmd/seed                 # 5 akun uji tetap
go run ./cmd/seed -all-regions    # + 1 admin per provinsi & kabupaten
go run ./cmd/seed -email a@x.id -role ADMIN_KABUPATEN -prov-kode 32 -kab-kode 3273   # satu admin regional
```

Mode `-all-regions` membuat akun berpola `adminprov.<kode-bps>@kipan.id` dan
`adminkab.<kode-bps>@kipan.id` (idempoten), di atas 5 akun tetap. Utility dev
lain: `go run ./cmd/flushcache` (state Redis) dan `go run ./cmd/flushstorage`
(kosongkan bucket MinIO).

Aturan password seeder:
- Minimal 12 karakter, wajib memuat huruf besar, huruf kecil, angka, dan simbol.
- Bila `SEED_ADMIN_PASSWORD` tidak diisi, seeder **membuat password acak** dan
  menampilkannya **sekali** di output — catat ke password manager.
- Seeder **menolak** berjalan kecuali `APP_ENV` = `development`/`test`/`local`.
- Menjalankan ulang seeder akan **mengganti** password akun tersebut dengan nilai baru.

Akun yang dibuat (email sama dengan `pentest_suite.ps1`):

| Role Level | Email Akun | Cakupan Wilayah | Hak Akses |
|:---|:---|:---|:---|
| **SUPER_ADMIN** | `superadmin@kipan.id` | Global (Seluruh Indonesia) | Akses sistem penuh & konfigurasi |
| **ADMIN_NASIONAL** | `adminnasional@kipan.id` | Nasional (DPP) | Akses operasional seluruh wilayah |
| **ADMIN_PROVINSI** | `adminprov.jabar@kipan.id` | Provinsi Jawa Barat (ID: 32) | Hanya berwenang atas wilayah Jabar |
| **ADMIN_KABUPATEN** | `adminkab.bandung@kipan.id` | Kota Bandung (ID: 3273) | Hanya berwenang atas Kota Bandung |
| **USER (KADER)** | `user.kader@kipan.id` | Mandiri (data sendiri via `anggota.user_id`) | Login JWT sama; **tanpa akses `/admin/*`** (403); KTA sendiri via `/user/kta` |

---

## 🌐 DAFTAR ENDPOINT AKTIF

### 1. Autentikasi Publik
* **`POST /api/v1/auth/login`**
  - **Body**: `{"email": "superadmin@kipan.id", "password": "<SEED_ADMIN_PASSWORD>"}`
  - **Response**: Mengembalikan `access_token` (JWT 15 menit) dan objek profil `user`.
    Refresh token HANYA dikirim via cookie HttpOnly, tidak pernah di JSON body.

* **`POST /api/v1/auth/refresh`**
  - Memakai cookie `refresh_token` (HttpOnly). Body JSON TIDAK diterima.
  - **Fitur Keamanan**: *Refresh Token Rotation (RTR)* dengan deteksi pencurian token (*reuse detection*).

* **`POST /api/v1/auth/logout`**
  - **Publik (tanpa Bearer wajib)**: mencabut sesi via cookie `refresh_token`
    bahkan saat access token sudah kedaluwarsa/invalid.
  - Access token di header (jika ada) dimasukkan ke **Redis Blacklist**
    secara best-effort; refresh token family dicabut di database.
  - Selalu mengembalikan sukses (idempoten) dan membersihkan cookie.

* **`POST /api/v1/auth/forgot-password`** (Batch 3)
  - **Body**: `{"email": "user@kipan.id"}` → **selalu 200** dengan pesan sama
    (anti-enumeration), baik email terdaftar maupun tidak.
  - Bila email terdaftar & aktif: tautan reset (`{APP_PUBLIC_URL}/reset-password?token=...`)
    dikirim via email. Token 256-bit, TTL **30 menit**, **sekali pakai**
    (hash disimpan di Redis). Limiter: 3/15 menit per email+IP, 5/15 menit per IP.

* **`POST /api/v1/auth/reset-password`** (Batch 3)
  - **Body**: `{"token": "...", "new_password": "<min 8, kuat>"}`.
  - Menukar token dengan password baru (Argon2id) + **mencabut seluruh sesi**.
  - Token salah/kedaluwarsa/bekas → `422` dengan pesan seragam.

### 2. Autentikasi Terproteksi (Wajib Header `Authorization: Bearer <access_token>`)
* **`GET /api/v1/auth/me`**
  - Mengembalikan informasi profil user yang sedang login beserta nama wilayahnya.

* **`PUT /api/v1/auth/password`**
  - **Body**: `{"old_password": "<SEED_ADMIN_PASSWORD>", "new_password": "NewSecretPassword2026!"}`
  - Mengubah kata sandi dengan validasi sandi lama dan hashing Argon2id.
  - Semua refresh token milik user dicabut setelah perubahan berhasil.

### 3. Pengujian Otorisasi & Scoping Wilayah (Protected)
* **`GET /api/v1/admin/me-scope`**
  - Menampilkan hasil evaluasi middleware `ScopeWilayah()`.
  - Admin Provinsi akan terisi `filter_provinsi_id`, Admin Kabupaten akan terisi `filter_kabupaten_id`, sedangkan Super Admin & Nasional bernilai `null` (bebas akses).

* **`GET /api/v1/admin/super-only`**
  - Dilindungi middleware `RequireRoles(SUPER_ADMIN)`.
  - Jika dicoba diakses oleh Admin Provinsi/Kabupaten akan otomatis ditolak dengan `403 Forbidden`.

* **`GET /api/v1/admin/nasional-or-super`**
  - Dilindungi middleware `RequireRoles(SUPER_ADMIN, ADMIN_NASIONAL)`.

### 4. Layanan Mandiri Anggota (Wajib `Bearer` role `USER`)
* **`GET /api/v1/user/kta`**
  - Tiket unduh PDF KTA milik sendiri. Otorisasi = `anggota.user_id`
    (bukan wilayah). Tanpa anggota terhubung → `404`.
  - Akun `USER` dibuat otomatis saat admin menyetujui pendaftaran
    (`POST /admin/pendaftaran/:id/setujui`): respons sukses memuat
    `one_time_password` **sekali** (password awal acak untuk diteruskan
    ke anggota via kanal resmi; tidak pernah masuk log/audit).
    Email yang sudah punya akun **ber-role `USER`** → dihubungkan tanpa
    password baru. Email milik akun non-anggota (admin, dsb) → `409`
    (admin harus memperbaiki email pendaftar; mencegah data anggota
    "dimiliki" akun orang lain).

* **`POST /api/v1/admin/anggota/:id/reset-password`** (Batch T1)
  - Admin (dalam yurisdiksi anggota) menerbitkan password awal **baru**
    untuk akun `USER` anggota. Respons memuat `one_time_password` **sekali**.
  - Seluruh sesi anggota dicabut; aksi tercatat di audit (`PASSWORD_RESET`,
    tanpa password). Anggota belum punya akun → `409`.
  - Dipakai saat password awal hasil approve hilang/terlewat.

### 5. OTP WhatsApp Submit Pendaftaran (Publik, Anti-Bot)
* **`POST /api/v1/pendaftaran/otp/whatsapp/request`** `{"whatsapp":"081234567890"}`
  → kode 6-digit (TTL 5 menit, cooldown 60 dtk, maks 5/jam/nomor).
* **`POST /api/v1/pendaftaran/otp/whatsapp/verify`** `{"whatsapp":"...","code":"123456"}`
  → `verified_token` sekali pakai (TTL 15 menit, maks 5x coba).
* **`POST /api/v1/pendaftaran`** wajib sertakan `wa_otp_token` (milik nomor
  yang sama). Tanpa/tidak valid/bekas pakai → `422`. Token tidak hangus
  bila submit gagal validasi lain (dokumen, dsb) — hangus saat data tersimpan.
* Kesalahan kode selalu `Kode salah atau kedaluwarsa` (tanpa oracle).
* **Batch 2:** pengiriman memakai gateway nyata **Fonnte** (`WA_GATEWAY_PROVIDER=fonnte`
  + `FONNTE_TOKEN_KEY`). `WA_GATEWAY_PROVIDER=log` hanya untuk dev/test dan
  **ditolak saat start di production** (mencegah OTP bocor ke log).
* Respons `debug_code` tetap hanya ada di non-production (untuk pentest/dev).

### 6. Kirim Nomor Pendaftaran via WhatsApp (Batch 2)
* Setelah `POST /pendaftaran` sukses, backend mengirim **nomor REG + tautan lacak**
  (`{APP_PUBLIC_URL}/lacak?nomor=...`) ke WhatsApp pendaftar secara
  **async best-effort** — submit tetap `201` walau gateway gagal/lambat.
* Base tautan diambil dari `APP_PUBLIC_URL` (wajib diisi di production).

### 7. Notifikasi Email & Token Revisi via Email (Batch 3)
* **Status pendaftaran** dikirim via **email** (async best-effort):
  `PERBAIKAN` (+catatan), `DISETUJUI` (+NIA), `DITOLAK` (+alasan).
* **Token revisi (Opsi B):** `POST /pendaftaran/revisi/request-token` tetap
  mewajibkan bukti nomor+email+WA, tetapi token kini **dikirim ke email
  terdaftar** dan **tidak lagi** ada di respons API (respons hanya
  `expires_at`). TTL 24 jam, sekali pakai.
* SMTP: dev Mailpit (`MAIL_HOST=127.0.0.1:1025`, tanpa auth) atau langsung
  Mailtrap sandbox (`sandbox.smtp.mailtrap.io:2525` + username/password tab SMTP).
  `MAIL_HOST` kosong = log-only (dev; isi email tampil di log server).
  **Production fail-fast** bila `MAIL_HOST` kosong/lokal atau `MAIL_USERNAME` kosong.

### 8. Kepengurusan: SK, Pengurus, Jabatan
Pendaftaran hanya untuk **Kader**; menjadi Pengurus hanya via **pengangkatan SK**.
Alur baru: SK dibuat **`DRAFT`** → aksi **`AJUKAN`** (KAB→`MENUNGGU_PROVINSI`,
PROV→`MENUNGGU_NASIONAL`, NAS→`DISETUJUI` final) → PROV `TERUSKAN` → NAS
`SAHKAN`/`TOLAK`. Detail: `docs/IMPLEMENTASI_PENGANGKATAN.md` +
`docs/IMPLEMENTASI_SK_MULTILEVEL.md` + `docs/Document/IMPLEMENTASI_SUPER_ADMIN_LANJUTAN.md`.

* **`POST /api/v1/admin/sk`** — buat SK (**selalu `DRAFT`**). Body menyertakan `nomor_sk`, `judul`, `level`, `provinsi_id`, `kabupaten_id`, `tanggal_terbit`, **`tanggal_berakhir`** (wajib, > terbit), `file_sk_key` (wajib). **Level & wilayah ditentukan server dari role**: Super/Nasional bebas pilih level+wilayah (divalidasi ke master); Provinsi dipaksa `PROVINSI` + provnya; Kabupaten dipaksa `KABUPATEN` + wilayahnya.
* **`GET /api/v1/admin/sk`** — daftar ter-scope; filter `level`, `status` (Aktif/TidakAktif), `approval` (DRAFT/MENUNGGU_*/DISETUJUI/DITOLAK), `search`; `with_total=0` melewati COUNT.
* **`GET /api/v1/admin/sk/:id`** — detail + susunan pengurus.
* **`POST /api/v1/admin/sk/:id/approve`** — `{"action":"AJUKAN|TERUSKAN|SAHKAN|TOLAK","catatan":"..."}`. `AJUKAN`=pengelola SK (Kab/Prov/Nas/Super sesuai level); `TERUSKAN`=PROV; `SAHKAN`/`TOLAK`=NAS/SUPER (TOLAK wajib catatan). Saat **`AJUKAN` SK Nasional** atau **`SAHKAN`** berlaku **Single Active SK rule**: SK lain selevel+wilayah → `TidakAktif` & pengurusnya → `Demisioner` (atomik).
* **`POST /api/v1/admin/sk/:id/status`** — aktif/nonaktifkan SK.
* **`POST /api/v1/admin/sk/:id/pengurus`** — tambah pengurus; peran mengikuti **level SK** (Opsi A: Kabupaten sekab **atau** Provinsi seprov); body `{anggota_id,jabatan_id,konfirmasi:true}`; SK `Aktif` & belum `DISETUJUI`. Efek **atomik**: pengurus lama → Demisioner; `anggota.tipe` + `users.tipe_user` → `PENGURUS`; seluruh sesi dicabut; email notifikasi async.
* **`DELETE /api/v1/admin/sk/:id/pengurus/:pengurusId`** — lepas pengurus (SK non-final).
* **`GET /api/v1/admin/pengurus`** — daftar ter-scope; filter `level`, `status`, `masa_jabatan` (Aktif/AkanBerakhir/Berakhir), `provinsi_id`, `kabupaten_id` (Nasional/Super), `search`, `with_total`; **`GET /api/v1/admin/pengurus/stats`** — kartu (total/per-level/akan-berakhir).
* **`PATCH /api/v1/admin/pengurus/:id`** — ubah **status** (keterangan wajib bila non-`Aktif`).
* **`PATCH /api/v1/admin/pengurus/:id/jabatan`** — **ganti jabatan** (`{"jabatan_id":n}`); SK non-final; jabatan inti tunggal per SK.
* **`GET /api/v1/admin/jabatan`** — master jabatan **berlevel** (`?level=NASIONAL|PROVINSI|KABUPATEN`); **`POST`/`PUT`** kelola hanya `SUPER_ADMIN`/`ADMIN_NASIONAL`. Validasi `jabatan.level == sk.level`.

Aturan: jabatan **inti** maksimum satu pemegang per SK; anggota wajib `AKTIF` &
satu wilayah dengan SK; SK `DISETUJUI` **terkunci** (buat SK baru untuk perubahan).

### 9. Dasbor Analitik
* **`GET /api/v1/admin/dashboard`** — ringkasan ter-scope: Nasional → per Provinsi; Provinsi → per Kabupaten/Kota; Kabupaten → per Kecamatan. Akses `USER` ditolak (403).

### 10. Master Wilayah (Super/Nasional)
* **`GET /api/v1/admin/wilayah/cards`** — total provinsi/kabupaten/pengurus.
* **`GET /api/v1/admin/wilayah?type=provinsi|kabupaten&search=&status=&provinsi_id=&page=&limit=`** — daftar **ter-paginasi** (offset) + `jml_kabupaten`/`jml_pengurus`/`ketua`; meta memuat `total`/`total_pages` (limit maks 100).
* **`GET /api/v1/admin/wilayah/:type/:id/detail`** — info + pengurus + statistik (total/aktif, total kabupaten, tren 6 bulan) + **activity** (`activity_logs`).
* **`GET /api/v1/admin/wilayah/:type/:id/pengurus?all=`** — pengurus level sesuai.
* **`PATCH /api/v1/admin/wilayah/:type/:id`** `{is_active}` — aktif/nonaktifkan + audit.
* **`POST /api/v1/admin/wilayah`** `{type,provinsi_id,kabupaten_id}` — aktifkan dari master + audit.

### 11. Manajemen Pengguna (Super Admin)
* **`GET /api/v1/admin/users`** — list akun admin (filter `role`/`status`/`search`) + pagination.
* **`GET /api/v1/admin/users/counts`** — kartu total + per role.
* **`POST /api/v1/admin/users`** — buat akun (`tipe_user=ADMIN`), **password auto-generate tampil sekali**.
* **`PUT /api/v1/admin/users/:id`** — ubah nama/email/role/wilayah/status; `reset_password:true` → password baru (tampil sekali); password eksplisit opsional.
* **`DELETE /api/v1/admin/users/:id`** — **soft delete** (`deleted_at`).
* Keamanan: **Super-only**; **cegah lockout** (tak bisa menonaktifkan/menghapus/ubah-role akun sendiri; tak bisa menghilangkan Super Admin aktif terakhir); **cabut sesi** saat Nonaktif/ganti role/reset password; audit tanpa PII.

### 12. Optimisasi Skala (B8)
* Keyset pagination (opt-in `?paginate=cursor` + `meta.next_cursor`) untuk **Anggota** & **Pendaftaran**.
* `with_total=0` (lewati COUNT) untuk SK/Pengurus; SK list memakai `LEFT JOIN LATERAL` untuk hitungan pengurus.
* Frontend: debounce 300 ms; pager keyset. Monitoring: `pg_stat_statements` + log request >200 ms.

---

## 🧪 MENJALANKAN PENTEST SUITE

```powershell
cd backend
$env:PENTEST_ADMIN_PASSWORD = ''<SEED_ADMIN_PASSWORD>''
./scripts/pentest_suite.ps1
```

Suite akan **abort (exit 1)** bila `PENTEST_ADMIN_PASSWORD` belum diset —
tidak ada kredensial default di dalam skrip.

Untuk mengaktifkan regresi `SEED-01` (memastikan password default lama sudah
tidak bisa dipakai), sediakan nilai lamanya lewat environment:

```powershell
$env:PENTEST_LEGACY_PASSWORD = ''<password-default-lama>''
```

Suite mengembalikan **exit code non-zero** bila ada test FAIL, sehingga dapat
dipakai langsung sebagai release gate.

---

## 🔒 CATATAN KEAMANAN (HARDENING FASE 1)

Migrasi `000004_harden_fase1_security.up.sql` menutup temuan audit Fase 1:

1. **Penonaktifan akun seed lama** — akun seed dengan password default publik
   di-set `status = ''Nonaktif''` dan seluruh refresh token-nya dicabut. Akun tidak
   dihapus agar audit trail tetap utuh.
   *Rollback migrasi ini sengaja TIDAK mengaktifkan kembali akun tersebut*,
   karena mengaktifkan akun berpassword publik = mengembalikan kerentanan.
   Gunakan seeder dengan password baru untuk mengaktifkannya kembali.
2. **Master wilayah menjadi protected** — foreign key `users.provinsi_id` dan
   `users.kabupaten_id` diubah dari `ON DELETE SET NULL` menjadi `ON DELETE RESTRICT`
   sesuai keputusan ERD v3.0 (master wilayah tidak boleh terhapus).

## 🛡️ VALIDASI DOKUMEN PENDAFTARAN (8 LAPIS)

1. Pola key allowlist (anti traversal) — `checkObjectKey`.
2. Presign allowlist MIME + ukuran per kategori (2/5MB) — tiket berupa POST
   policy dengan `content-length-range`, sehingga S3/MinIO menolak body di
   luar batas di level storage (Batch 4). Klien mengirim multipart dengan
   field policy lebih dulu dan part `file` terakhir.
3. Keberadaan objek (HeadObject) sebelum nomor dialokasikan.
4. Ukuran + Content-Type tersimpan sesuai kategori.
5. Magic bytes 512 awal (anti rename ekstensi).
6. **Tolak key ganda antar-slot** — satu key di 2 slot (mis. foto=ktp) → `422`
   "tidak boleh memakai berkas yang sama". Berlaku submit + revisi.
7. **Tolak PDF ber-password** — deteksi `/Encrypt` via `SniffTail` (8KB ekor) →
   `422` "terkunci password". Password dokumen tidak pernah diminta/disimpan.
8. Verifikasi manual admin di antrean (backstop terakhir).

Browser ikut mencegah dini (duplikat file + pindai `/Encrypt` via `File.slice`)
tetapi server selalu memutus. Cek isi-identik-beda-key via ETag masih backlog
(butuh riset ETag multipart MinIO vs R2/IS3 agar tanpa false-positive).
## 🛡️ HARDENING OPS (Batch 6)

* **`/internal/health` (#2):** detail env + status dependensi tidak lagi
  terbuka. Bila `INTERNAL_HEALTH_TOKEN` diisi, wajib header
  `X-Internal-Token` (constant-time; gagal -> `404` agar tidak jadi sinyal
  recon). Bila token kosong: endpoint hanya didaftarkan di non-production;
  **di production tanpa token, route tidak didaftarkan sama sekali**.
* **Endpoint uji (#6):** `/admin/super-only` dan `/admin/nasional-or-super`
  kini hanya aktif di non-production (production -> `404`). `/admin/me-scope`
  tetap ada karena dipakai dasbor (hanya mengembalikan klaim pemanggil).
* **Roadmap keamanan (T14, belum dikerjakan):** MFA untuk akun admin,
  notifikasi login gagal ke pemilik, dan batas jumlah sesi paralel.

## ✅ ROADMAP PRE-PRODUCTION (belum, butuh keputusan/infra)

* **Antivirus scan dokumen (T3):** belum ada; perlu engine (mis. ClamAV
  sidecar) sebelum go-live. Admin membuka dokumen via presigned URL.
* **OTP WhatsApp:** pastikan `WA_GATEWAY_PROVIDER=fonnte` di produksi
  (sudah fail-fast) dan token valid.
* **Email:** pastikan `MAIL_HOST` non-lokal + `MAIL_USERNAME` di produksi
  (sudah fail-fast).

## 🧹 HYGIENE (Batch 7)

* **Token akses di memori (frontend, #8):** access token tidak lagi disimpan
  di localStorage (mengurangi dampak XSS). Sesi dipulihkan dari refresh cookie
  HttpOnly saat aplikasi dimuat; penanda non-sensitif `kipan_has_session`
  dipakai agar pengunjung anonim tidak menembak `/auth/me` sia-sia.
* **Audit best-effort (#9):** kegagalan menulis activity_logs hanya dicatat
  (warning) dan TIDAK menggagalkan operasi utama (ketersediaan). Aksi sangat
  sensitif tetap teraudit saat DB normal; janji append-only dijamin trigger DB.
* **SameSite=None (#10):** ditolak browser tanpa Secure; produksi selalu Secure.
  Di non-production, server mencetak peringatan bila dikonfigurasi None.
* **CORS multi-origin (#11):** origin dipisah koma; spasi dirapikan otomatis
  (lihat juga T12). Wildcard dilarang di produksi.
