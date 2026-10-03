-- ============================================================
-- TIPE PENDAFTARAN KADER / PENGURUS
-- Version: 000011_tipe_pendaftaran.up.sql
-- ============================================================
-- Dua jalur pendaftaran memakai form yang sama: KADER (tanpa SK) vs
-- PENGURUS (wajib SK). Tipe disimpan di pendaftaran, disalin ke anggota
-- saat DISETUJUI, lalu ke users.tipe_user oleh ensureMemberAccount.
-- Data lama (pra-tipe) dianggap KADER via DEFAULT.
--
-- Sifat migrasi: IDEMPOTEN (aman dijalankan berkali-kali).
-- ============================================================

ALTER TABLE pendaftaran ADD COLUMN IF NOT EXISTS tipe_pendaftaran VARCHAR(20) NOT NULL DEFAULT 'KADER';
ALTER TABLE pendaftaran DROP CONSTRAINT IF EXISTS chk_pendaftaran_tipe;
ALTER TABLE pendaftaran
    ADD CONSTRAINT chk_pendaftaran_tipe
    CHECK (tipe_pendaftaran IN ('KADER', 'PENGURUS'));

ALTER TABLE anggota ADD COLUMN IF NOT EXISTS tipe VARCHAR(20) NOT NULL DEFAULT 'KADER';
ALTER TABLE anggota DROP CONSTRAINT IF EXISTS chk_anggota_tipe;
ALTER TABLE anggota
    ADD CONSTRAINT chk_anggota_tipe
    CHECK (tipe IN ('KADER', 'PENGURUS'));

CREATE INDEX IF NOT EXISTS idx_pendaftaran_tipe ON pendaftaran (tipe_pendaftaran);
