-- ============================================================
-- RETENSI PENDAFTAR DITOLAK + UNIQUE NIK PARSIAL (TDD D16)
-- Version: 000028_pendaftaran_nik_partial_unique.up.sql
-- ============================================================
-- Data pendaftar berstatus DITOLAK disimpan permanen sebagai arsip, dan
-- pendaftar dapat mendaftar ulang dengan NIK yang sama. Karena itu keunikan
-- nik_hash dibatasi pada baris non-DITOLAK (unique parsial). Sifat: IDEMPOTEN.

DROP INDEX IF EXISTS uq_pendaftaran_nik_hash;

CREATE UNIQUE INDEX IF NOT EXISTS uq_pendaftaran_nik_hash
    ON pendaftaran (nik_hash) WHERE status <> 'DITOLAK';

COMMENT ON INDEX uq_pendaftaran_nik_hash IS
    'NIK unik hanya untuk pendaftaran non-DITOLAK (pendaftar ditolak boleh daftar ulang).';
