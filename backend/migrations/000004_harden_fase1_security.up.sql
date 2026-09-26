-- ============================================================
-- HARDENING KEAMANAN FASE 1 (AUTHENTICATION & AUTHORIZATION)
-- Version: 000004_harden_fase1_security.up.sql
-- ============================================================
-- Menutup temuan audit keamanan Fase 1:
--
--   [CRITICAL] Akun admin dengan password default yang tertulis publik
--              pernah di-seed lewat migrasi 000002 (RULES #9 —
--              "Jangan menggunakan password default produksi").
--
--   [MEDIUM]   Relasi users -> master wilayah memakai ON DELETE SET NULL,
--              menyimpang dari keputusan ERD v3.0 yang mensyaratkan
--              RESTRICT (master wilayah bersifat immutable / tidak boleh
--              terhapus). RULES #16.
--
-- Sifat migrasi: IDEMPOTEN (aman dijalankan berkali-kali).
-- ============================================================

-- ============================================================
-- 1. NETRALKAN AKUN SEED LAMA (password default publik)
-- ============================================================
-- Akun TIDAK dihapus agar histori, audit trail, dan relasi tetap utuh,
-- tetapi dinonaktifkan. AuthService.Login dan RefreshToken sudah menolak
-- user dengan status != 'Aktif' (domain.UserStatusAktif), sehingga akun ini
-- tidak bisa login maupun melakukan refresh token.
--
-- Untuk mengaktifkan kembali DENGAN PASSWORD BARU, jalankan:
--     cd backend
--     $env:SEED_ADMIN_PASSWORD='<password-kuat>'; go run ./cmd/seed
UPDATE users
SET status     = 'Nonaktif',
    updated_at = CURRENT_TIMESTAMP
WHERE LOWER(email) IN (
        'superadmin@kipan.id',
        'adminnasional@kipan.id',
        'adminprov.jabar@kipan.id',
        'adminkab.bandung@kipan.id'
      )
  AND deleted_at IS NULL
  AND status <> 'Nonaktif';

-- Cabut seluruh sesi aktif milik akun seed lama, sehingga access token
-- maupun refresh token yang mungkin masih beredar tidak dapat dipakai.
UPDATE user_refresh_tokens
SET is_revoked = TRUE
WHERE is_revoked = FALSE
  AND user_id IN (
        SELECT u.id
        FROM users u
        WHERE LOWER(u.email) IN (
                'superadmin@kipan.id',
                'adminnasional@kipan.id',
                'adminprov.jabar@kipan.id',
                'adminkab.bandung@kipan.id'
              )
      );

-- ============================================================
-- 2. MASTER WILAYAH MENJADI PROTEKTED (RESTRICT)
-- ============================================================
-- Master wilayah (BPS) tidak boleh terhapus selama masih direferensikan
-- user. Sebelumnya ON DELETE SET NULL membuat user "yatim" tanpa wilayah
-- tanpa peringatan apa pun.
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_provinsi_id_fkey;
ALTER TABLE users
    ADD CONSTRAINT users_provinsi_id_fkey
    FOREIGN KEY (provinsi_id) REFERENCES wilayah_provinsi (id) ON DELETE RESTRICT;

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_kabupaten_id_fkey;
ALTER TABLE users
    ADD CONSTRAINT users_kabupaten_id_fkey
    FOREIGN KEY (kabupaten_id) REFERENCES wilayah_kabupaten (id) ON DELETE RESTRICT;
