-- ============================================================
-- PENDAFTARAN: STATUS DRAFT & KEDALUWARSA
-- Version: 000020_pendaftaran_draft.up.sql
-- ============================================================
-- Status awal pendaftaran berubah dari DIAJUKAN → DRAFT; ditambah status
-- KEDALUWARSA (DRAFT > 30 hari, ditandai lazy-on-access).
-- Sifat: IDEMPOTEN.
-- ============================================================

UPDATE pendaftaran SET status = 'DRAFT', updated_at = CURRENT_TIMESTAMP WHERE status = 'DIAJUKAN';

ALTER TABLE pendaftaran DROP CONSTRAINT IF EXISTS chk_pendaftaran_status;
ALTER TABLE pendaftaran
    ADD CONSTRAINT chk_pendaftaran_status
    CHECK (status IN ('DRAFT', 'DIVERIFIKASI', 'PERBAIKAN', 'DISETUJUI', 'DITOLAK', 'KEDALUWARSA'));
