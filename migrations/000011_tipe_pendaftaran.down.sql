-- ============================================================
-- ROLLBACK: 000011_tipe_pendaftaran
-- ============================================================
-- Melepas kolom + constraint tipe. Data pendaftaran/anggota TIDAK
-- dihapus (selaras filosofi rollback 000004/000008/000010).
-- ============================================================

DROP INDEX IF EXISTS idx_pendaftaran_tipe;
ALTER TABLE anggota DROP CONSTRAINT IF EXISTS chk_anggota_tipe;
ALTER TABLE anggota DROP COLUMN IF EXISTS tipe;
ALTER TABLE pendaftaran DROP CONSTRAINT IF EXISTS chk_pendaftaran_tipe;
ALTER TABLE pendaftaran DROP COLUMN IF EXISTS tipe_pendaftaran;
