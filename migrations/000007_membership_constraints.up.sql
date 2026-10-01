-- ============================================================
-- 7. CONSTRAINT MEMBERSHIP (BATCH 4 FASE 2)
-- Version: 000007_membership_constraints.up.sql
-- ============================================================
-- Menutup F-29/F-33/F-35: state machine dan append-only sebelumnya hanya
-- dijaga aplikasi. Constraint DB menjadi defense-in-depth:
--   1. CHECK status/enum pendaftaran, anggota, riwayat, jenis kelamin.
--   2. pendaftaran_riwayat: CASCADE -> RESTRICT (riwayat tak ikut hilang).
--   3. anggota.pendaftaran_id: SET NULL -> RESTRICT (anti anggota yatim).
--   4. Index revisi_token_hash untuk lookup token.
--   5. CHECK format NIA kanonis strip (KIPAN-%).
-- Seluruhnya IF NOT EXISTS / kondisional agar idempoten.
-- ============================================================

-- 1. CHECK status & enum
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_pendaftaran_status') THEN
        ALTER TABLE pendaftaran ADD CONSTRAINT chk_pendaftaran_status
            CHECK (status IN ('DIAJUKAN','DIVERIFIKASI','PERBAIKAN','DISETUJUI','DITOLAK'));
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_pendaftaran_jk') THEN
        ALTER TABLE pendaftaran ADD CONSTRAINT chk_pendaftaran_jk
            CHECK (jenis_kelamin IN ('L','P'));
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_anggota_status') THEN
        ALTER TABLE anggota ADD CONSTRAINT chk_anggota_status
            CHECK (status IN ('AKTIF','NONAKTIF','DEMISIONER','DIBERHENTIKAN','MENINGGAL'));
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_anggota_jk') THEN
        ALTER TABLE anggota ADD CONSTRAINT chk_anggota_jk
            CHECK (jenis_kelamin IN ('L','P'));
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_anggota_nia_format') THEN
        ALTER TABLE anggota ADD CONSTRAINT chk_anggota_nia_format
            CHECK (nia LIKE 'KIPAN-%');
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_riwayat_aksi') THEN
        ALTER TABLE pendaftaran_riwayat ADD CONSTRAINT chk_riwayat_aksi
            CHECK (aksi IN ('SUBMIT','VERIFIKASI','PERBAIKAN','TOLAK','SETUJUI','REVISI','REVISI_TOKEN'));
    END IF;
END
$$;

-- 2. Riwayat: CASCADE -> RESTRICT
ALTER TABLE pendaftaran_riwayat DROP CONSTRAINT IF EXISTS pendaftaran_riwayat_pendaftaran_id_fkey;
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_riwayat_pendaftaran_restrict') THEN
        ALTER TABLE pendaftaran_riwayat ADD CONSTRAINT fk_riwayat_pendaftaran_restrict
            FOREIGN KEY (pendaftaran_id) REFERENCES pendaftaran(id) ON DELETE RESTRICT;
    END IF;
END
$$;

-- 3. Link anggota->pendaftaran: SET NULL -> RESTRICT
ALTER TABLE anggota DROP CONSTRAINT IF EXISTS anggota_pendaftaran_id_fkey;
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_anggota_pendaftaran_restrict') THEN
        ALTER TABLE anggota ADD CONSTRAINT fk_anggota_pendaftaran_restrict
            FOREIGN KEY (pendaftaran_id) REFERENCES pendaftaran(id) ON DELETE RESTRICT;
    END IF;
END
$$;

-- 4. Index lookup token revisi
CREATE INDEX IF NOT EXISTS idx_pendaftaran_revisi_token
    ON pendaftaran(revisi_token_hash);
