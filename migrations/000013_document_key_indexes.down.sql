-- ============================================================
-- ROLLBACK: 000013_document_key_indexes
-- ============================================================
DROP INDEX IF EXISTS idx_pendaftaran_foto_key;
DROP INDEX IF EXISTS idx_pendaftaran_ktp_key;
DROP INDEX IF EXISTS idx_pendaftaran_cv_key;
DROP INDEX IF EXISTS idx_pendaftaran_sk_key;
DROP INDEX IF EXISTS idx_pendaftaran_surat_pernyataan_key;
DROP INDEX IF EXISTS idx_pendaftaran_surat_sehat_key;

DROP INDEX IF EXISTS idx_anggota_foto_key;
DROP INDEX IF EXISTS idx_anggota_ktp_key;
DROP INDEX IF EXISTS idx_anggota_cv_key;
DROP INDEX IF EXISTS idx_anggota_sk_key;
DROP INDEX IF EXISTS idx_anggota_surat_pernyataan_key;
DROP INDEX IF EXISTS idx_anggota_surat_sehat_key;
