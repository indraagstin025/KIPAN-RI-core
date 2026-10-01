-- ============================================================
-- 8. CHECKLIST PERSYARATAN PENDAFTARAN (PENYESUAIAN FASE 1-2)
-- Version: 000008_persyaratan_checklist.up.sql
-- ============================================================
-- Form KIPAN_INDONESIA mewajibkan checklist persyaratan dicentang semua
-- (PendaftaranAnggota.tsx). Kolom ini menyimpan pilihan pendaftar sebagai
-- JSON array string agar tampil di detail admin. Validasi isi (maks 20
-- item, tiap item 1-100 karakter, tanpa <>) ditegakkan di service —
-- database hanya menjamin nilai berupa JSON array.
-- ============================================================

ALTER TABLE pendaftaran
    ADD COLUMN IF NOT EXISTS persyaratan_checklist JSONB NOT NULL DEFAULT '[]';

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_pendaftaran_persyaratan_array') THEN
        ALTER TABLE pendaftaran ADD CONSTRAINT chk_pendaftaran_persyaratan_array
            CHECK (jsonb_typeof(persyaratan_checklist) = 'array');
    END IF;
END
$$;
