-- ============================================================
-- 6. SEQUENCE NOMOR REGISTRASI PENDAFTARAN (BATCH 1 FASE 2)
-- Version: 000006_registration_sequence.up.sql
-- ============================================================
-- Menutup C-5/F-14: alokasi nomor urut REG-YYYYMM-XXXXX sebelumnya
-- hardcoded 1 di kode. Sequence per-bulan dialokasikan atomik via
-- UPSERT ... RETURNING (lihat NextRegistrationSequence), sehingga
-- ribuan pendaftar serentak tidak pernah menerima nomor yang sama.
-- UNIQUE pada pendaftaran.nomor_pendaftaran tetap menjadi backstop.
-- ============================================================

CREATE TABLE IF NOT EXISTS pendaftaran_nomor_sequence (
    tahun      INT NOT NULL,
    bulan      INT NOT NULL CHECK (bulan BETWEEN 1 AND 12),
    next_value INT NOT NULL DEFAULT 1 CHECK (next_value >= 1),
    PRIMARY KEY (tahun, bulan)
);
