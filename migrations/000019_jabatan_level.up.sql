-- ============================================================
-- JABATAN BERLEVEL (NASIONAL / PROVINSI / KABUPATEN)
-- Version: 000019_jabatan_level.up.sql
-- ============================================================
-- Jabatan kini per tingkat (seperti project lama). Nama boleh sama di
-- 3 level; keunikan = (nama, level). Jabatan lama (tanpa level) → NASIONAL,
-- lalu diduplikasi ke PROVINSI & KABUPATEN.
-- Sifat: IDEMPOTEN.
-- ============================================================

ALTER TABLE jabatan ADD COLUMN IF NOT EXISTS level VARCHAR(20);
UPDATE jabatan SET level = 'NASIONAL', updated_at = CURRENT_TIMESTAMP WHERE level IS NULL;
ALTER TABLE jabatan ALTER COLUMN level SET NOT NULL;

ALTER TABLE jabatan DROP CONSTRAINT IF EXISTS jabatan_nama_key;
ALTER TABLE jabatan DROP CONSTRAINT IF EXISTS uq_jabatan_nama_level;
ALTER TABLE jabatan ADD CONSTRAINT uq_jabatan_nama_level UNIQUE (nama, level);

ALTER TABLE jabatan DROP CONSTRAINT IF EXISTS chk_jabatan_level;
ALTER TABLE jabatan
    ADD CONSTRAINT chk_jabatan_level
    CHECK (level IN ('NASIONAL', 'PROVINSI', 'KABUPATEN'));

-- Duplikasi jabatan NASIONAL ke PROVINSI & KABUPATEN.
INSERT INTO jabatan (nama, level, is_inti, is_active, urutan)
SELECT nama, 'PROVINSI', is_inti, is_active, urutan FROM jabatan WHERE level = 'NASIONAL'
ON CONFLICT (nama, level) DO NOTHING;

INSERT INTO jabatan (nama, level, is_inti, is_active, urutan)
SELECT nama, 'KABUPATEN', is_inti, is_active, urutan FROM jabatan WHERE level = 'NASIONAL'
ON CONFLICT (nama, level) DO NOTHING;
