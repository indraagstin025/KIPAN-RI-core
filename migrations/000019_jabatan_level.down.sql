-- ROLLBACK 000019_jabatan_level
DELETE FROM jabatan WHERE level IN ('PROVINSI', 'KABUPATEN');

ALTER TABLE jabatan DROP CONSTRAINT IF EXISTS uq_jabatan_nama_level;
ALTER TABLE jabatan DROP CONSTRAINT IF EXISTS chk_jabatan_level;
ALTER TABLE jabatan ADD CONSTRAINT jabatan_nama_key UNIQUE (nama);
ALTER TABLE jabatan DROP COLUMN IF EXISTS level;
