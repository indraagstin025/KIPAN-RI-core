-- ============================================================
-- ROLLBACK: 000012_riwayat_actor_fk
-- ============================================================
DROP INDEX IF EXISTS idx_riwayat_actor;
ALTER TABLE pendaftaran_riwayat DROP CONSTRAINT IF EXISTS fk_riwayat_actor;
