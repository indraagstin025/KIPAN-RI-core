-- ============================================================
-- GUARD NAMA WILAYAH (ANTI STORED-XSS)
-- Version: 000015_wilayah_name_guard.up.sql
-- ============================================================
-- Nama provinsi/kabupaten berasal dari master eksternal (wilayah.id) dan
-- dirender di dropdown + detail admin. CHECK ini menjadi pagar terakhir di
-- DB agar karakter < > " tidak pernah tersimpan, melengkapi validasi
-- generator (scripts/gen_wilayah_seed.ps1) dan escape React.
-- Sifat: IDEMPOTEN (DROP IF EXISTS lalu ADD).
-- ============================================================

ALTER TABLE wilayah_provinsi DROP CONSTRAINT IF EXISTS chk_wilayah_provinsi_nama;
ALTER TABLE wilayah_provinsi
    ADD CONSTRAINT chk_wilayah_provinsi_nama
    CHECK (nama NOT LIKE '%<%' AND nama NOT LIKE '%>%' AND nama NOT LIKE '%"%');

ALTER TABLE wilayah_kabupaten DROP CONSTRAINT IF EXISTS chk_wilayah_kabupaten_nama;
ALTER TABLE wilayah_kabupaten
    ADD CONSTRAINT chk_wilayah_kabupaten_nama
    CHECK (nama NOT LIKE '%<%' AND nama NOT LIKE '%>%' AND nama NOT LIKE '%"%');
