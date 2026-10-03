-- ============================================================
-- INDEKS PENCARIAN & PAGINATION SKALA BESAR
-- Version: 000021_search_indexes.up.sql
-- ============================================================
-- pg_trgm GIN agar ILIKE '%q%' (nama/NIA/nomor/judul) terindeks, plus
-- indeks komposit untuk keyset pagination (created_at, id).
-- Sifat: IDEMPOTEN.
-- ============================================================

CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- Trigram (substring search)
CREATE INDEX IF NOT EXISTS idx_anggota_nama_trgm ON anggota USING gin (nama_lengkap gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_anggota_nia_trgm ON anggota USING gin (nia gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_sk_nomor_trgm ON surat_keputusan USING gin (nomor_sk gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_sk_judul_trgm ON surat_keputusan USING gin (judul gin_trgm_ops);

-- Komposit untuk urutan + filter (keyset pagination)
CREATE INDEX IF NOT EXISTS idx_anggota_created_id ON anggota (created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_pendaftaran_status_created ON pendaftaran (status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_pendaftaran_wilayah_created ON pendaftaran (provinsi_id, kabupaten_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_pengurus_level_created ON pengurus (level, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_sk_level_wilayah_created ON surat_keputusan (level, provinsi_id, kabupaten_id, created_at DESC);
