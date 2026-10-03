-- ============================================================
-- GUARD FORMAT NIA BARU (KIPAN-IND-...)
-- Version: 000018_nia_format_guard.up.sql
-- ============================================================
-- Format NIA berubah ke KIPAN-IND-[KAB 4 digit]-[TAHUN]-[6 digit]
-- (contoh: KIPAN-IND-3204-2026-000001). Constraint diperketat agar hanya
-- format baru yang diterima.
--
-- PENTING: bila masih ada baris anggota berformat LAMA, backfill dulu
-- (lihat docs/IMPLEMENTASI_MIGRASI_NIA.md) sebelum migrasi ini dijalankan,
-- agar penambahan constraint tidak gagal.
--
-- Sifat: IDEMPOTEN (DROP IF EXISTS lalu ADD).
-- ============================================================

ALTER TABLE anggota DROP CONSTRAINT IF EXISTS chk_anggota_nia_format;
ALTER TABLE anggota
    ADD CONSTRAINT chk_anggota_nia_format
    CHECK (nia LIKE 'KIPAN-IND-%');
