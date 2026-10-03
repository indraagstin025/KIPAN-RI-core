-- ROLLBACK 000021_search_indexes
DROP INDEX IF EXISTS idx_anggota_nama_trgm;
DROP INDEX IF EXISTS idx_anggota_nia_trgm;
DROP INDEX IF EXISTS idx_sk_nomor_trgm;
DROP INDEX IF EXISTS idx_sk_judul_trgm;
DROP INDEX IF EXISTS idx_anggota_created_id;
DROP INDEX IF EXISTS idx_pendaftaran_status_created;
DROP INDEX IF EXISTS idx_pendaftaran_wilayah_created;
DROP INDEX IF EXISTS idx_pengurus_level_created;
DROP INDEX IF EXISTS idx_sk_level_wilayah_created;
-- Ekstensi pg_trgm SENGAJA tidak di-drop (mungkin dipakai objek lain).
