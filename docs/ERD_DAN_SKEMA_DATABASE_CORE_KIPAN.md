# 🗄️ ERD & SKEMA DATABASE — CORE SYSTEM SIM-KIPAN INDONESIA
## Versi Final v3.0 — Disempurnakan dengan Keputusan Arsitektur Teknis

> **Ruang Lingkup**: Core Engine Saja — Keanggotaan, Pendaftaran, SK & Pengurus, Auth  
> **CMS (Berita, Galeri, Program Kerja, Profil Organisasi)**: Terpisah penuh di **Laravel**  
> **Database Engine**: PostgreSQL 16 di IDCloudHost VPS (Private Network)  
> **Object Storage**: IDCloudHost IS3 (S3-Compatible) — Data Residency Indonesia 🇮🇩  

---

## KEPUTUSAN ARSITEKTUR FINAL

| Keputusan | Pilihan Final | Justifikasi |
|:---|:---|:---|
| **Row-Level Security (RLS)** | ❌ Tidak dipakai | RBAC yurisdiksi wilayah ditangani di Go Service Layer. RLS + PgBouncer transaction pooling tidak kompatibel dengan andal |
| **Manajemen Kunci Enkripsi** | ✅ Env variable (Fase 1) → HashiCorp Vault self-hosted (Fase Produksi) | IDCloudHost tidak memiliki managed KMS; Vault dijalankan di VPS kecil terpisah |
| **Soft Delete `anggota`** | ✅ **Status-based** (`AKTIF`, `NONAKTIF`, `DEMISIONER`, `DIBERHENTIKAN`, `MENINGGAL`) | Transisi status keanggotaan bermakna hukum — bukan penghapusan teknis |
| **Soft Delete `pendaftaran`** | ✅ **Status-based** (`DIAJUKAN`, `DIVERIFIKASI`, `PERBAIKAN`, `DISETUJUI`, `DITOLAK`) | Tidak ada skenario bisnis "hapus pendaftaran" |
| **Soft Delete `surat_keputusan`** | ✅ **Status-based** (`Aktif`, `TidakAktif`, `Digantikan`) | SK yang tidak aktif tetap bernilai sebagai rekam jejak legal |
| **Soft Delete `users`** | ✅ **`deleted_at` + Partial Unique Index** pada `email` | Email akun admin yang dihapus harus bisa didaftarkan ulang |
| **Soft Delete `provinsi`/`kabupaten`** | ✅ **Kolom `is_active BOOLEAN`** | Master wilayah tidak pernah dihapus, hanya dinonaktifkan (pemekaran) |

---

## 1. DIAGRAM ERD LENGKAP (CORE SYSTEM)

```mermaid
erDiagram

    %% ============================================================
    %% MASTER DATA WILAYAH
    %% ============================================================

    PROVINSI {
        int         id          PK
        string      kode        UK  "Kode BPS 2 digit, e.g. 32"
        string      nama
        boolean     is_active       "FALSE = nonaktif karena pemekaran"
        timestamp   created_at
        timestamp   updated_at
    }

    KABUPATEN {
        int         id          PK
        int         provinsi_id FK
        string      kode        UK  "Kode BPS 4 digit, e.g. 3204"
        string      nama
        boolean     is_active
        timestamp   created_at
        timestamp   updated_at
    }

    JABATAN {
        int         id          PK
        string      nama            "Ketua Umum, Sekretaris Jenderal, dst"
        string      level           "NASIONAL | PROVINSI | KABUPATEN"
        int         urutan          "Urutan hierarki jabatan (untuk tampilan)"
        timestamp   created_at
        timestamp   updated_at
    }

    %% ============================================================
    %% AUTH & USERS
    %% Soft Delete: deleted_at + Partial Unique Index pada email
    %% ============================================================

    USERS {
        uuid        id          PK
        string      email           "UNIQUE WHERE deleted_at IS NULL (Partial Index)"
        string      password_hash   "Argon2id — min Memory 64MB, Iter 3, Salt 16B"
        string      name
        string      role            "SUPER_ADMIN | ADMIN_NASIONAL | ADMIN_PROVINSI | ADMIN_KABUPATEN"
        string      status          "Aktif | Nonaktif | Suspended"
        string      avatar_url      "IDCloudHost IS3 Object Key, bukan Base64"
        int         provinsi_id FK  "NULL jika SUPER_ADMIN atau ADMIN_NASIONAL"
        int         kabupaten_id FK "NULL jika bukan ADMIN_KABUPATEN"
        timestamp   last_login_at
        timestamp   created_at
        timestamp   updated_at
        timestamp   deleted_at      "Soft Delete. Partial Unique Index berlaku di sini"
    }

    USER_REFRESH_TOKENS {
        uuid        id          PK
        uuid        user_id     FK
        char        token_hash      "SHA-256 dari raw refresh token (64 hex)"
        uuid        family_id       "Mendeteksi token reuse — jika reuse: revoke all family"
        boolean     is_revoked
        timestamp   expires_at      "TTL 7 hari"
        timestamp   created_at
    }

    %% ============================================================
    %% PENDAFTARAN CALON ANGGOTA
    %% Soft Delete: TIDAK ADA — Status-based, data tidak pernah dihapus
    %% ============================================================

    PENDAFTARAN {
        int         id          PK
        string      nomor_pendaftaran UK "REG-YYYYMM-XXXX — untuk tracking mandiri"
        string      nama_lengkap
        char        nik_hash        "HMAC-SHA256(NIK, BLIND_INDEX_KEY) — 64 hex — Blind Index"
        text        nik_encrypted   "AES-256-GCM: base64(iv):base64(ct):base64(tag)"
        string      tempat_lahir
        date        tanggal_lahir
        string      jenis_kelamin   "L | P"
        string      agama
        string      pendidikan
        string      pekerjaan
        string      status_pribadi
        text        alamat
        int         provinsi_id FK
        int         kabupaten_id FK
        string      kecamatan
        string      desa
        string      kode_pos
        string      email
        string      whatsapp        "Format: 08xx atau +628xx"
        text        motivasi
        string      foto_key        "IS3 Object Key"
        string      ktp_key         "IS3 Object Key — PRIVATE bucket"
        string      cv_key          "IS3 Object Key"
        string      sk_key          "IS3 Object Key"
        string      surat_pernyataan_key "IS3 Object Key"
        string      surat_sehat_key "IS3 Object Key — PRIVATE bucket"
        json        persyaratan_checklist
        string      status          "DIAJUKAN | DIVERIFIKASI | PERBAIKAN | DISETUJUI | DITOLAK"
        text        catatan_perbaikan
        char        revisi_token_hash   "SHA-256 OTP magic link untuk akses revisi berkas"
        timestamp   revisi_token_expires_at "TTL 24 jam"
        int         anggota_id FK   "NULL sampai DISETUJUI — DATA TIDAK DIHAPUS saat approval"
        timestamp   created_at
        timestamp   updated_at
    }

    PENDAFTARAN_RIWAYAT {
        int         id          PK
        int         pendaftaran_id FK
        string      aksi            "SUBMIT | VERIFY | REQUEST_REVISION | APPROVE | REJECT"
        uuid        actor_id FK     "NULL jika aksi sistem otomatis"
        string      actor_name      "Snapshot nama aktor saat aksi terjadi"
        string      actor_role      "Snapshot role aktor saat aksi terjadi"
        text        catatan
        timestamp   created_at
    }

    %% ============================================================
    %% ANGGOTA RESMI (KADER KIPAN)
    %% Soft Delete: Status-based — AKTIF|NONAKTIF|DEMISIONER|DIBERHENTIKAN|MENINGGAL
    %% ============================================================

    ANGGOTA {
        int         id          PK
        string      nia         UK  "KIPAN-[PROV]-[KAB]-[TAHUN]-[NO_URUT]"
        string      nama_lengkap
        char        nik_hash    UK  "HMAC Blind Index UNIQUE — 1 NIK = 1 Anggota Aktif"
        text        nik_encrypted   "AES-256-GCM"
        string      tempat_lahir
        date        tanggal_lahir
        string      jenis_kelamin   "L | P"
        string      agama
        string      pendidikan
        string      pekerjaan
        text        alamat
        int         provinsi_id FK
        int         kabupaten_id FK
        string      kecamatan
        string      desa
        string      kode_pos
        string      email
        string      whatsapp
        string      foto_key        "IS3 Object Key — PUBLIC bucket (untuk tampil di profil)"
        string      ktp_key         "IS3 Object Key — PRIVATE bucket"
        string      cv_key          "IS3 Object Key"
        string      sk_key          "IS3 Object Key"
        string      surat_pernyataan_key "IS3 Object Key"
        string      surat_sehat_key "IS3 Object Key — PRIVATE bucket"
        string      status          "AKTIF | NONAKTIF | DEMISIONER | DIBERHENTIKAN | MENINGGAL"
        string      angkatan        "Tahun angkatan kader, e.g. 2026"
        char        kta_qr_hash     "HMAC-SHA256(NIA+TanggalAngkat, KTA_SIGNING_KEY) — Anti-Palsu"
        string      kta_pdf_key     "IS3 Object Key PDF KTA resmi yang diterbitkan server"
        int         pendaftaran_id FK "Jejak audit ke pendaftaran asal — nullable"
        uuid        user_id FK  UK  "NULL jika belum punya akun portal admin"
        date        tanggal_daftar
        date        tanggal_angkat
        timestamp   created_at
        timestamp   updated_at
    }

    %% ============================================================
    %% SURAT KEPUTUSAN
    %% Soft Delete: Status-based — Aktif|TidakAktif|Digantikan
    %% ============================================================

    SURAT_KEPUTUSAN {
        int         id          PK
        string      nomor_sk    UK
        string      judul
        string      level           "NASIONAL | PROVINSI | KABUPATEN"
        int         provinsi_id FK  "NULL jika level NASIONAL"
        int         kabupaten_id FK "NULL jika bukan level KABUPATEN"
        date        tanggal_terbit
        date        tanggal_berakhir "NULL = berlaku tidak terbatas"
        string      file_sk_key     "IS3 Object Key PDF naskah SK resmi — PRIVATE bucket"
        string      status          "Aktif | TidakAktif | Digantikan"
        string      approval_status "DRAFT | MENUNGGU_PROVINSI | MENUNGGU_NASIONAL | DISETUJUI | DITOLAK"
        text        catatan_penolakan
        uuid        created_by FK   "Admin pembuat draf — AUDIT TRAIL"
        uuid        approved_by FK  "Admin DPP pengesah final — AUDIT TRAIL"
        timestamp   approved_at
        timestamp   created_at
        timestamp   updated_at
    }

    %% ============================================================
    %% PENGURUS
    %% Soft Delete: Status-based kolom status
    %% ============================================================

    PENGURUS {
        int         id          PK
        int         anggota_id  FK
        int         surat_keputusan_id FK
        string      level           "NASIONAL | PROVINSI | KABUPATEN"
        int         provinsi_id FK
        int         kabupaten_id FK
        int         jabatan_id  FK  "NOT NULL — pengurus wajib punya jabatan"
        string      status          "Aktif | Demisioner | Diberhentikan | Mengundurkan Diri | Meninggal"
        text        keterangan_status
        date        tanggal_mulai
        date        tanggal_selesai "NULL = masih aktif menjabat"
        timestamp   created_at
        timestamp   updated_at
    }

    %% ============================================================
    %% AUDIT LOG — Non-Repudiasi Forensik
    %% Tidak ada soft delete / update — append-only
    %% ============================================================

    ACTIVITY_LOGS {
        bigint      id          PK
        uuid        actor_id FK     "NULL = aksi sistem otomatis"
        string      actor_name      "Snapshot nama riil saat aksi (bukan hardcoded Admin)"
        string      actor_role      "Snapshot role saat aksi terjadi"
        string      ip_address      "IPv4 / IPv6 publik pemanggil"
        text        user_agent      "Browser / client string"
        string      entity_name     "pendaftaran | anggota | surat_keputusan | pengurus | users"
        string      entity_id       "PK record yang diubah"
        string      action          "CREATE | UPDATE | DELETE | APPROVE | REJECT | DEMISIONER | LOGIN | LOGOUT"
        json        metadata        "before/after values — PII (NIK) wajib dimasking"
        string      request_id      "X-Request-ID untuk korelasi log server"
        timestamp   created_at
    }

    %% ============================================================
    %% NOTIFIKASI
    %% ============================================================

    NOTIFICATIONS {
        bigint      id          PK
        uuid        user_id FK
        string      title
        text        message
        string      type            "PENDAFTARAN | SK | SISTEM"
        string      link
        boolean     is_read
        timestamp   created_at
    }

    %% ============================================================
    %% RELASI ANTAR ENTITAS
    %% ============================================================

    PROVINSI    ||--o{  KABUPATEN           : "memiliki kabupaten/kota"
    PROVINSI    ||--o{  USERS               : "scope yurisdiksi admin"
    PROVINSI    ||--o{  PENDAFTARAN         : "domisili pendaftar"
    PROVINSI    ||--o{  ANGGOTA             : "wilayah kader"
    PROVINSI    ||--o{  SURAT_KEPUTUSAN     : "SK wilayah provinsi"
    PROVINSI    ||--o{  PENGURUS            : "kepengurusan provinsi"

    KABUPATEN   ||--o{  USERS               : "scope yurisdiksi admin DPC"
    KABUPATEN   ||--o{  PENDAFTARAN         : "domisili pendaftar"
    KABUPATEN   ||--o{  ANGGOTA             : "kader kab/kota"
    KABUPATEN   ||--o{  SURAT_KEPUTUSAN     : "SK kab/kota"
    KABUPATEN   ||--o{  PENGURUS            : "kepengurusan kab/kota"

    JABATAN     ||--o{  PENGURUS            : "jabatan dalam SK"

    USERS       ||--o{  USER_REFRESH_TOKENS : "token sesi aktif"
    USERS       ||--o|  ANGGOTA             : "akun portal anggota"
    USERS       ||--o{  NOTIFICATIONS       : "menerima notifikasi"
    USERS       ||--o{  PENDAFTARAN_RIWAYAT : "aktor tindakan verifikasi"
    USERS       ||--o{  SURAT_KEPUTUSAN     : "sebagai created_by drafter"
    USERS       ||--o{  ACTIVITY_LOGS       : "aktor dalam audit log"

    PENDAFTARAN ||--o{  PENDAFTARAN_RIWAYAT : "kronologi verifikasi berkas"
    PENDAFTARAN ||--o|  ANGGOTA             : "menghasilkan kader sah saat DISETUJUI"
    PENDAFTARAN }o--||  PROVINSI            : "domisili"
    PENDAFTARAN }o--||  KABUPATEN           : "domisili"

    ANGGOTA     ||--o{  PENGURUS            : "menjabat dalam SK"
    ANGGOTA     }o--||  PROVINSI            : "wilayah kader"
    ANGGOTA     }o--||  KABUPATEN           : "wilayah kader"

    SURAT_KEPUTUSAN ||--o{ PENGURUS         : "memuat daftar pengurus aktif"
    SURAT_KEPUTUSAN }o--|| PROVINSI         : "SK wilayah"
    SURAT_KEPUTUSAN }o--|| KABUPATEN        : "SK wilayah"
```

---

## 2. STRATEGI SOFT DELETE PER TABEL (FINAL)

| Tabel | Strategi | Kolom | Alasan |
|:---|:---|:---|:---|
| `provinsi` | **`is_active BOOLEAN`** | `is_active DEFAULT TRUE` | Master wilayah tidak pernah dihapus, hanya nonaktif saat pemekaran |
| `kabupaten` | **`is_active BOOLEAN`** | `is_active DEFAULT TRUE` | Idem |
| `jabatan` | **`is_active BOOLEAN`** | `is_active DEFAULT TRUE` | Jabatan yang tidak berlaku lagi dinonaktifkan |
| `users` | **`deleted_at` + Partial Index** | `deleted_at TIMESTAMPTZ NULL` | Email harus bisa didaftar ulang. `UNIQUE idx ON users(email) WHERE deleted_at IS NULL` |
| `pendaftaran` | **Status-based** | `status VARCHAR(30)` | Tidak ada skenario hapus pendaftaran. Status: `DIAJUKAN \| PERBAIKAN \| DISETUJUI \| DITOLAK` |
| `anggota` | **Status-based** | `status VARCHAR(30)` | Transisi status bermakna hukum AD/ART. `AKTIF \| NONAKTIF \| DEMISIONER \| DIBERHENTIKAN \| MENINGGAL` |
| `surat_keputusan` | **Status-based** | `status VARCHAR(20)` | SK lama tetap tersimpan sebagai arsip legal. `Aktif \| TidakAktif \| Digantikan` |
| `pengurus` | **Status-based** | `status VARCHAR(30)` | Status kepengurusan bermakna organisasi. `Aktif \| Demisioner \| Diberhentikan` |
| `activity_logs` | **Append-only, TIDAK ADA delete** | — | Log forensik tidak boleh dimodifikasi/dihapus dalam kondisi apapun |
| `notifications` | **Hard delete** diperbolehkan | — | Tidak ada nilai bisnis dalam menyimpan notifikasi lama yang sudah dibaca |

---

## 3. SKEMA PENGELOLAAN KUNCI ENKRIPSI (KMS STAGED)

```
┌─────────────────────────────────────────────────────────────────┐
│ FASE 1 — Development & Soft Launch (Env Variable)               │
│                                                                 │
│  .env (permission 600, tidak pernah di-commit ke Git):          │
│    AES_MASTER_KEY   = [32-byte random hex, generate sekali]     │
│    BLIND_INDEX_KEY  = [32-byte random hex, BERBEDA dari atas]   │
│    KTA_SIGNING_KEY  = [32-byte random hex, BERBEDA lagi]        │
│                                                                 │
│  Go Fiber startup (main.go):                                    │
│    if len(cfg.AESMasterKey) < 32 { log.Fatal("MISSING KEY") }  │
│    → Server menolak start jika kunci tidak ada / pendek         │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼ (Setelah Go-Live & Stabil)
┌─────────────────────────────────────────────────────────────────┐
│ FASE 2 — Produksi (HashiCorp Vault Self-Hosted di IDCloudHost)  │
│                                                                 │
│  VPS Vault: 1 vCPU, 2 GB RAM (terpisah dari App VPS)           │
│  Vault KV Secret Engine:                                        │
│    vault kv put secret/kipan/prod \                             │
│      aes_master_key=... blind_index_key=... kta_signing_key=... │
│                                                                 │
│  Go Fiber startup:                                              │
│    1. Auth ke Vault dengan AppRole credentials                  │
│    2. Fetch kunci dari Vault KV API                             │
│    3. Simpan di memory (struct), tidak pernah ke disk/log       │
│    4. Lease renewal otomatis setiap 12 jam                      │
│                                                                 │
│  Keuntungan:                                                    │
│    ✅ Key rotation tanpa restart server                         │
│    ✅ Audit siapa mengakses kunci (Vault audit log)             │
│    ✅ Developer tidak tahu nilai kunci produksi                 │
│    ✅ Compliance bukti kontrol akses UU PDP Pasal 35           │
└─────────────────────────────────────────────────────────────────┘
```

---

## 4. SPESIFIKASI KOLOM KRIPTOGRAFI

### 4.1. `nik_hash` — Blind Index (Pencarian Cepat)
```
Algorithm : HMAC-SHA256
Secret Key : BLIND_INDEX_KEY (32 bytes dari Vault/env, TIDAK SAMA dengan AES_MASTER_KEY)
Input      : NIK plaintext (16 digit)
Output     : 64-char hex string → disimpan di kolom CHAR(64)

Query Pencarian (O(1) via B-Tree Index):
  target_hash = HMAC_SHA256(input_nik, BLIND_INDEX_KEY)
  SELECT * FROM anggota WHERE nik_hash = $1

Keamanan:
  - Deterministik → NIK sama selalu menghasilkan hash sama (diperlukan untuk search)
  - One-way → tidak bisa di-reverse ke NIK asli
  - Berbeda secret key dari enkripsi → kompromi satu kunci tidak kompromi keduanya
```

### 4.2. `nik_encrypted` — Enkripsi Reversible (Tampilkan NIK kepada Admin)
```
Algorithm  : AES-256-GCM (Authenticated Encryption)
Secret Key : AES_MASTER_KEY (32 bytes)
Nonce/IV   : crypto.random(12 bytes) — UNIK per enkripsi (tidak pernah diulang)
Auth Tag   : 16 bytes (verifikasi integritas + otentisitas)

Format Storage : base64(nonce) + ":" + base64(ciphertext) + ":" + base64(authTag)
Contoh         : "abc12=:xyz99=:def45="

Dekripsi hanya boleh terjadi:
  1. Admin membuka halaman detail anggota spesifik
  2. NIK yang ditampilkan SELALU dimasking: "3204************"
  3. Untuk lihat NIK utuh: butuh konfirmasi re-autentikasi + dicatat di audit_logs
```

### 4.3. `kta_qr_hash` — Signature Anti-Pemalsuan KTA
```
Algorithm  : HMAC-SHA256
Secret Key : KTA_SIGNING_KEY (32 bytes, berbeda dari dua kunci di atas)
Input      : NIA + ":" + TanggalAngkat.Format("2006-01-02") + ":" + AnggotaID
Output     : 64-char hex string

QR Code URL: https://sim.kipan.id/v/{nia}?sig={kta_qr_hash}

Alur Verifikasi Lapangan:
  1. Scan QR → Buka URL publik
  2. Backend ambil anggota dari DB by NIA
  3. Hitung ulang HMAC dari data DB
  4. Bandingkan dengan sig dari URL
  5. Cocok  → Tampilkan KTA VALID ✅ (Foto, Nama, NIA, Status, Jabatan dari DB)
  6. Beda   → Tampilkan KTA TIDAK VALID ❌ (indikasi pemalsuan fisik)
```

---

## 5. INDEKS DATABASE (LENGKAP)

### 5.1. Indeks Bisnis Kritis
```sql
-- Pencarian NIK (O(1) — paling sering dipakai)
CREATE INDEX idx_pendaftaran_nik_hash  ON pendaftaran(nik_hash);
CREATE UNIQUE INDEX idx_anggota_nik_hash ON anggota(nik_hash);

-- Dashboard DPC: Filter antrean pendaftaran per kab/kota
CREATE INDEX idx_pendaftaran_wilayah_status
  ON pendaftaran(provinsi_id, kabupaten_id, status, created_at DESC);

-- Tracking pendaftaran mandiri oleh publik
CREATE INDEX idx_pendaftaran_nomor ON pendaftaran(nomor_pendaftaran);

-- Revisi token (OTP lookup)
CREATE INDEX idx_pendaftaran_revisi_token
  ON pendaftaran(revisi_token_hash) WHERE revisi_token_expires_at > NOW();

-- NIA lookup (verifikasi KTA publik)
CREATE UNIQUE INDEX idx_anggota_nia ON anggota(nia);

-- Dashboard DPD/DPC: daftar kader wilayah aktif
CREATE INDEX idx_anggota_wilayah_status
  ON anggota(provinsi_id, kabupaten_id, status);

-- SK filter berdasarkan level dan wilayah
CREATE INDEX idx_sk_level_wilayah
  ON surat_keputusan(level, provinsi_id, kabupaten_id, status);

-- Struktur pengurus aktif (tampilan publik & admin)
CREATE INDEX idx_pengurus_wilayah_status
  ON pengurus(level, provinsi_id, kabupaten_id, status);

-- Riwayat jabatan seorang anggota
CREATE INDEX idx_pengurus_anggota ON pengurus(anggota_id);

-- Load pengurus dari SK tertentu
CREATE INDEX idx_pengurus_sk ON pengurus(surat_keputusan_id);
```

### 5.2. Indeks Auth & Session
```sql
-- Verifikasi refresh token (hot path setiap 15 menit per user aktif)
CREATE UNIQUE INDEX idx_refresh_token_hash
  ON user_refresh_tokens(token_hash) WHERE is_revoked = FALSE;

-- Cabut semua token satu keluarga saat deteksi reuse
CREATE INDEX idx_refresh_token_family ON user_refresh_tokens(family_id);

-- KUNCI: Email unik hanya untuk akun yang belum dihapus (Partial Unique Index)
CREATE UNIQUE INDEX idx_users_email_active
  ON users(email) WHERE deleted_at IS NULL;
```

### 5.3. Indeks Audit & Notifikasi
```sql
-- Audit trail per aktor admin
CREATE INDEX idx_activity_actor ON activity_logs(actor_id, created_at DESC);

-- Riwayat perubahan 1 record spesifik
CREATE INDEX idx_activity_entity ON activity_logs(entity_name, entity_id);

-- Load notifikasi belum dibaca per admin (hot path setiap buka dashboard)
CREATE INDEX idx_notifications_unread
  ON notifications(user_id, is_read) WHERE is_read = FALSE;
```

---

## 6. ATURAN INTEGRITAS DATABASE (CONSTRAINT)

```sql
-- Provinsi & Kabupaten: RESTRICT delete jika masih ada relasi aktif
ALTER TABLE kabupaten
  ADD CONSTRAINT fk_kab_provinsi
  FOREIGN KEY (provinsi_id) REFERENCES provinsi(id)
  ON UPDATE CASCADE ON DELETE RESTRICT;

-- Users: CASCADE delete token saat user dihapus
ALTER TABLE user_refresh_tokens
  ADD CONSTRAINT fk_token_user
  FOREIGN KEY (user_id) REFERENCES users(id)
  ON DELETE CASCADE;

-- Pendaftaran: SET NULL ke anggota_id jika anggota dihapus (edge case)
ALTER TABLE pendaftaran
  ADD CONSTRAINT fk_pendaftaran_anggota
  FOREIGN KEY (anggota_id) REFERENCES anggota(id)
  ON UPDATE CASCADE ON DELETE SET NULL;

-- Pengurus: RESTRICT hapus SK jika masih ada pengurus aktif
ALTER TABLE pengurus
  ADD CONSTRAINT fk_pengurus_sk
  FOREIGN KEY (surat_keputusan_id) REFERENCES surat_keputusan(id)
  ON UPDATE CASCADE ON DELETE RESTRICT;

-- Jabatan wajib ada (NOT NULL pada jabatan_id di pengurus)
-- Tidak ada pengurus tanpa jabatan yang jelas

-- Activity Logs: SET NULL actor jika akun dihapus (log tetap tersimpan)
ALTER TABLE activity_logs
  ADD CONSTRAINT fk_log_actor
  FOREIGN KEY (actor_id) REFERENCES users(id)
  ON DELETE SET NULL;

-- Notifikasi: CASCADE hapus saat user dihapus
ALTER TABLE notifications
  ADD CONSTRAINT fk_notif_user
  FOREIGN KEY (user_id) REFERENCES users(id)
  ON DELETE CASCADE;
```

---

## 7. PETA BUCKET OBJECT STORAGE (IDCloudHost IS3)

| Bucket | Visibilitas | Konten | Akses |
|:---|:---:|:---|:---|
| `kipan-public` | Public Read | Foto profil anggota, avatar admin, thumbnail | URL langsung via CDN |
| `kipan-private` | Private (ACL Blocked) | Scan KTP, Surat Sehat, SK, KTA PDF | Presigned GET URL (TTL 5 menit) + Audit Log |
| `kipan-uploads` | Private (Temp) | Berkas yang baru di-upload sebelum pendaftaran disubmit | Presigned PUT URL (TTL 10 menit), di-move ke private setelah submit |

---

## 8. DIFF PERBAIKAN DARI SCHEMA.PRISMA LAMA (RINGKASAN)

| # | Tabel / Kolom Lama | Perubahan Final | Perbaikan Masalah |
|:---|:---|:---|:---:|
| 1 | `users.avatar @db.LongText` | `avatar_url VARCHAR(500)` S3 Key | Issue 24 |
| 2 | `users.role @default("SUPER_ADMIN")` | **Dihapus default** — wajib eksplisit | Issue 8 |
| 3 | `users.status` string bebas | `status CHECK IN (Aktif, Nonaktif, Suspended)` | Issue 2 |
| 4 | *(tidak ada)* | `user_refresh_tokens` baru | Issue 1 |
| 5 | *(tidak ada)* | `deleted_at` + Partial Unique Index email pada `users` | Issue 2 |
| 6 | `pendaftaran.nik String` | `nik_hash CHAR(64)` + `nik_encrypted TEXT` | Issue 13,14,15 |
| 7 | `pendaftaran.foto/ktp/cv/...@db.LongText` | 6 kolom `_key VARCHAR(255)` S3 | Issue 23,24 |
| 8 | *(tidak ada)* | `pendaftaran.anggota_id FK` | Issue 34 |
| 9 | *(tidak ada)* | `pendaftaran.revisi_token_hash` + expires | Issue 20 |
| 10 | `anggota.nik String` | `nik_hash CHAR(64)` UK + `nik_encrypted TEXT` | Issue 13,14,15 |
| 11 | `anggota.foto/ktp/...@db.LongText` | 6 kolom `_key VARCHAR(255)` S3 | Issue 24 |
| 12 | *(tidak ada)* | `anggota.kta_qr_hash CHAR(64)` | Issue 33 |
| 13 | *(tidak ada)* | `anggota.kta_pdf_key VARCHAR(255)` | Issue 33 |
| 14 | *(tidak ada)* | `anggota.pendaftaran_id FK` | Issue 34 |
| 15 | `surat_keputusan.fileSK @db.LongText` | `file_sk_key VARCHAR(255)` S3 | Issue 24 |
| 16 | *(tidak ada)* | `surat_keputusan.created_by FK` | Issue 11,29 |
| 17 | *(tidak ada)* | `surat_keputusan.approved_by FK` + `approved_at` | Issue 11,29 |
| 18 | `pengurus.jabatanId Int?` nullable | `jabatan_id INT NOT NULL` | Logika bisnis |
| 19 | `activity_logs.oleh @default("Admin")` | `actor_id UUID FK` + `actor_name` + `actor_role` + `ip_address` + `user_agent` + `request_id` | Issue 29 |
| 20 | `provinsi/kabupaten.deleted_at` | `is_active BOOLEAN` | Issue 27 |
| 21 | `Berita`, `Galeri`, `ProgramKerja`, `ProfilOrganisasi` | **Dihapus dari Core** (pindah ke Laravel) | Pemisahan sistem |
