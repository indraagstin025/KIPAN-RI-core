-- ROLLBACK 000014_seed_wilayah_nasional
-- SENGAJA NON-DESTRUKTIF: data master wilayah dipertahankan agar FK
-- (pendaftaran/anggota/users) tidak yatim. Rollback = no-op.
SELECT 1;