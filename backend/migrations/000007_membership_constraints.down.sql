-- ============================================================
-- ROLLBACK 000007_membership_constraints
-- ============================================================
-- Mengembalikan FK ke definisi 000003 (CASCADE / SET NULL) dan melepas
-- CHECK + index. Data tidak diubah.
-- ============================================================

ALTER TABLE pendaftaran DROP CONSTRAINT IF EXISTS chk_pendaftaran_status;
ALTER TABLE pendaftaran DROP CONSTRAINT IF EXISTS chk_pendaftaran_jk;
ALTER TABLE anggota DROP CONSTRAINT IF EXISTS chk_anggota_status;
ALTER TABLE anggota DROP CONSTRAINT IF EXISTS chk_anggota_jk;
ALTER TABLE anggota DROP CONSTRAINT IF EXISTS chk_anggota_nia_format;
ALTER TABLE pendaftaran_riwayat DROP CONSTRAINT IF EXISTS chk_riwayat_aksi;

ALTER TABLE pendaftaran_riwayat DROP CONSTRAINT IF EXISTS fk_riwayat_pendaftaran_restrict;
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'pendaftaran_riwayat_pendaftaran_id_fkey') THEN
        ALTER TABLE pendaftaran_riwayat ADD CONSTRAINT pendaftaran_riwayat_pendaftaran_id_fkey
            FOREIGN KEY (pendaftaran_id) REFERENCES pendaftaran(id) ON DELETE CASCADE;
    END IF;
END
$$;

ALTER TABLE anggota DROP CONSTRAINT IF EXISTS fk_anggota_pendaftaran_restrict;
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'anggota_pendaftaran_id_fkey') THEN
        ALTER TABLE anggota ADD CONSTRAINT anggota_pendaftaran_id_fkey
            FOREIGN KEY (pendaftaran_id) REFERENCES pendaftaran(id) ON DELETE SET NULL;
    END IF;
END
$$;

DROP INDEX IF EXISTS idx_pendaftaran_revisi_token;
