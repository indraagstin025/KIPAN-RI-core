-- ============================================================
-- TIPE USER "ADMIN" UNTUK AKUN ADMIN
-- Version: 000017_user_tipe_admin.up.sql
-- ============================================================
-- Sebelumnya tipe_user hanya KADER/PENGURUS (hanya bermakna untuk
-- role=USER), sehingga akun admin (role<>USER) ikut terisi default KADER
-- dan menyesatkan. Nilai "ADMIN" ditambahkan untuk menandai akun admin;
-- level tetap dibedakan oleh kolom role.
-- Sifat: IDEMPOTEN (DROP IF EXISTS lalu ADD).
-- ============================================================

ALTER TABLE users DROP CONSTRAINT IF EXISTS chk_users_tipe;
ALTER TABLE users
    ADD CONSTRAINT chk_users_tipe
    CHECK (tipe_user IN ('KADER', 'PENGURUS', 'ADMIN'));

UPDATE users SET tipe_user = 'ADMIN', updated_at = CURRENT_TIMESTAMP WHERE role <> 'USER';

COMMENT ON COLUMN users.tipe_user IS 'KADER/PENGURUS (role=USER) | ADMIN (role admin).';
