-- ROLLBACK 000018_nia_format_guard
ALTER TABLE anggota DROP CONSTRAINT IF EXISTS chk_anggota_nia_format;
ALTER TABLE anggota
    ADD CONSTRAINT chk_anggota_nia_format
    CHECK (nia LIKE 'KIPAN-%');
