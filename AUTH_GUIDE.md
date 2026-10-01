# 🔐 PANDUAN PENGUJIAN MODUL AUTENTIKASI & OTORISASI (SIM-KIPAN CORE)

Backend saat ini **100% fokus dan terisolasi khusus pada subsistem Autentikasi dan Otorisasi (RBAC & Scoping Wilayah)**.

---

## 🔑 AKUN PENGUJIAN 4 LEVEL RBAC (SEEDING MANUAL)

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
go run ./cmd/seed
```

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