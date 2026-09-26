# 🔐 PANDUAN PENGUJIAN MODUL AUTENTIKASI & OTORISASI (SIM-KIPAN CORE)

Backend saat ini **100% fokus dan terisolasi khusus pada subsistem Autentikasi dan Otorisasi (RBAC & Scoping Wilayah)**.

---

## 👥 AKUN PENGUJIAN 4 LEVEL RBAC
Semua akun pengujian di bawah ini telah disiapkan dalam migrasi database (`000002_seed_initial_data.up.sql`) dengan kata sandi bawaan: **`AdminKipan2026!`** (menggunakan hash **Argon2id**).

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
  - **Body**: `{"email": "superadmin@kipan.id", "password": "AdminKipan2026!"}`
  - **Response**: Mengembalikan `access_token` (JWT 15 menit), `refresh_token` (opaque 7 hari), dan objek profil `user`.

* **`POST /api/v1/auth/refresh`**
  - **Body**: `{"refresh_token": "<refresh_token>"}`
  - **Fitur Keamanan**: *Refresh Token Rotation (RTR)* dengan deteksi pencurian token (*reuse detection*).

### 2. Autentikasi Terproteksi (Wajib Header `Authorization: Bearer <access_token>`)
* **`GET /api/v1/auth/me`**
  - Mengembalikan informasi profil user yang sedang login beserta nama wilayahnya.

* **`PUT /api/v1/auth/password`**
  - **Body**: `{"old_password": "AdminKipan2026!", "new_password": "NewSecretPassword2026!"}`
  - Mengubah kata sandi dengan validasi sandi lama dan hashing Argon2id.

* **`POST /api/v1/auth/logout`**
  - **Body**: `{"refresh_token": "<refresh_token>"}`
  - Memasukkan access token ke **Redis Blacklist** seketika dan mencabut token family di database.

### 3. Pengujian Otorisasi & Scoping Wilayah (Protected)
* **`GET /api/v1/admin/me-scope`**
  - Menampilkan hasil evaluasi middleware `ScopeWilayah()`. 
  - Admin Provinsi akan terisi `filter_provinsi_id`, Admin Kabupaten akan terisi `filter_kabupaten_id`, sedangkan Super Admin & Nasional bernilai `null` (bebas akses).

* **`GET /api/v1/admin/super-only`**
  - Dilindungi middleware `RequireRoles(SUPER_ADMIN)`.
  - Jika dicoba diakses oleh Admin Provinsi/Kabupaten akan otomatis ditolak dengan `403 Forbidden`.

* **`GET /api/v1/admin/nasional-or-super`**
  - Dilindungi middleware `RequireRoles(SUPER_ADMIN, ADMIN_NASIONAL)`.
