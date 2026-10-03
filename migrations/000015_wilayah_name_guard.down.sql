-- ROLLBACK 000015_wilayah_name_guard
-- Melepas guard CHECK (non-destruktif terhadap data).
ALTER TABLE wilayah_provinsi DROP CONSTRAINT IF EXISTS chk_wilayah_provinsi_nama;
ALTER TABLE wilayah_kabupaten DROP CONSTRAINT IF EXISTS chk_wilayah_kabupaten_nama;
