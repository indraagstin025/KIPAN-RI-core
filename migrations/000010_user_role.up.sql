-- ============================================================
-- ROLE USER ANGGOTA (KADER / PENGURUS)
-- Version: 000010_user_role.up.sql (dinomori 000010 karena 000008/000009 sudah terpakai)
-- ============================================================
-- Menambah role ke-5 "USER" untuk akun anggota yang diterbitkan otomatis
-- saat pendaftaran DISETUJUI. Pembedaan Kader vs Pengurus memakai kolom
-- tipe_user (bukan role terpisah). Akun USER tidak memiliki yurisdiksi
-- admin (deny-by-default di RequireRoles + ScopeWilayah) dan mengakses
-- datanya sendiri via anggota.user_id.
--
-- Sifat migrasi: IDEMPOTEN (aman dijalankan berkali-kali).
-- ============================================================

-- 1. Kolom tipe_user (default KADER untuk seluruh baris existing).
ALTER TABLE users ADD COLUMN IF NOT EXISTS tipe_user VARCHAR(20) NOT NULL DEFAULT 'KADER';

-- 2. Allowlist role + tipe di level database (bukan sekadar konvensi aplikasi).
ALTER TABLE users DROP CONSTRAINT IF EXISTS chk_users_role;
ALTER TABLE users
    ADD CONSTRAINT chk_users_role
    CHECK (role IN ('SUPER_ADMIN', 'ADMIN_NASIONAL', 'ADMIN_PROVINSI', 'ADMIN_KABUPATEN', 'USER'));

ALTER TABLE users DROP CONSTRAINT IF EXISTS chk_users_tipe;
ALTER TABLE users
    ADD CONSTRAINT chk_users_tipe
    CHECK (tipe_user IN ('KADER', 'PENGURUS'));

COMMENT ON COLUMN users.tipe_user IS 'KADER (default saat approve) | PENGURUS (diangkat via SK). Hanya bermakna untuk role=USER.';

-- 3. Relasi anggota -> akun USER (sebelumnya kolom polos tanpa FK).
ALTER TABLE anggota DROP CONSTRAINT IF EXISTS fk_anggota_user;
ALTER TABLE anggota
    ADD CONSTRAINT fk_anggota_user
    FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_anggota_user_id ON anggota (user_id);
