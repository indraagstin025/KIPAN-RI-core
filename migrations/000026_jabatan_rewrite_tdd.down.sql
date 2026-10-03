-- ROLLBACK 000026_jabatan_rewrite_tdd
-- Mengembalikan kolom level (default NASIONAL) + keunikan (nama, level).
-- Catatan: baris PROVINSI/KABUPATEN yang telah didedupe TIDAK dapat dipulihkan.

ALTER TABLE jabatan DROP CONSTRAINT IF EXISTS uq_jabatan_nama;

ALTER TABLE jabatan ADD COLUMN IF NOT EXISTS level VARCHAR(20);
UPDATE jabatan SET level = 'NASIONAL', updated_at = CURRENT_TIMESTAMP WHERE level IS NULL;
ALTER TABLE jabatan ALTER COLUMN level SET NOT NULL;

ALTER TABLE jabatan DROP CONSTRAINT IF EXISTS chk_jabatan_level;
ALTER TABLE jabatan ADD CONSTRAINT chk_jabatan_level
    CHECK (level IN ('NASIONAL', 'PROVINSI', 'KABUPATEN'));

ALTER TABLE jabatan DROP CONSTRAINT IF EXISTS uq_jabatan_nama_level;
ALTER TABLE jabatan ADD CONSTRAINT uq_jabatan_nama_level UNIQUE (nama, level);

ALTER TABLE jabatan DROP COLUMN IF EXISTS is_ketua_umum;
