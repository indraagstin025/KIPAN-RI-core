-- ============================================================
-- ROLLBACK 000008_persyaratan_checklist
-- ============================================================

ALTER TABLE pendaftaran DROP CONSTRAINT IF EXISTS chk_pendaftaran_persyaratan_array;
ALTER TABLE pendaftaran DROP COLUMN IF EXISTS persyaratan_checklist;
