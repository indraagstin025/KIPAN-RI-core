-- ============================================================
-- ROLLBACK: 000010_user_role
-- ============================================================
-- ASIMETRIS DENGAN SENGAJA (selaras filosofi 000004): akun USER yang
-- sudah terbit TIDAK dihapus — rollback hanya melepas struktur. Akun
-- USER yang tersisa tidak bisa login bila aplikasi ikut di-rollback
-- (role tidak dikenal), aktifkan kembali dengan migrate-up.
-- ============================================================

ALTER TABLE anggota DROP CONSTRAINT IF EXISTS fk_anggota_user;
DROP INDEX IF EXISTS idx_anggota_user_id;
ALTER TABLE users DROP CONSTRAINT IF EXISTS chk_users_tipe;
ALTER TABLE users DROP CONSTRAINT IF EXISTS chk_users_role;
ALTER TABLE users DROP COLUMN IF EXISTS tipe_user;
