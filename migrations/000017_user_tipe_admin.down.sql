-- ROLLBACK 000017_user_tipe_admin
ALTER TABLE users DROP CONSTRAINT IF EXISTS chk_users_tipe;
ALTER TABLE users
    ADD CONSTRAINT chk_users_tipe
    CHECK (tipe_user IN ('KADER', 'PENGURUS'));

UPDATE users SET tipe_user = 'KADER', updated_at = CURRENT_TIMESTAMP WHERE role <> 'USER';

COMMENT ON COLUMN users.tipe_user IS 'KADER (default saat approve) | PENGURUS (diangkat via SK). Hanya bermakna untuk role=USER.';
