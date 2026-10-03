-- ROLLBACK 000020_pendaftaran_draft
UPDATE pendaftaran SET status = 'DIAJUKAN', updated_at = CURRENT_TIMESTAMP WHERE status = 'DRAFT';
UPDATE pendaftaran SET status = 'DITOLAK', updated_at = CURRENT_TIMESTAMP WHERE status = 'KEDALUWARSA';

ALTER TABLE pendaftaran DROP CONSTRAINT IF EXISTS chk_pendaftaran_status;
ALTER TABLE pendaftaran
    ADD CONSTRAINT chk_pendaftaran_status
    CHECK (status IN ('DIAJUKAN', 'DIVERIFIKASI', 'PERBAIKAN', 'DISETUJUI', 'DITOLAK'));
