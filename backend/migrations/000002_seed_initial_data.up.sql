-- ============================================================
-- SIM-KIPAN CORE SEED INITIAL DATA (AUTH & AUTHORIZATION)
-- Version: 000002_seed_initial_data.up.sql
-- ============================================================

-- 1. SEED PROVINSI & KABUPATEN SAMPLE (REFERENSI WILAYAH ADMIN)
INSERT INTO wilayah_provinsi (kode, nama) VALUES
('31', 'DKI JAKARTA'),
('32', 'JAWA BARAT'),
('33', 'JAWA TENGAH'),
('35', 'JAWA TIMUR')
ON CONFLICT (kode) DO NOTHING;

INSERT INTO wilayah_kabupaten (provinsi_id, kode, nama)
SELECT p.id, k.kode, k.nama FROM wilayah_provinsi p,
(VALUES 
    ('32', '3273', 'KOTA BANDUNG'),
    ('32', '3204', 'KABUPATEN BANDUNG'),
    ('31', '3171', 'KOTA JAKARTA PUSAT')
) AS k(prov_kode, kode, nama)
WHERE p.kode = k.prov_kode
ON CONFLICT (kode) DO NOTHING;

-- 2. AKUN ADMIN - SENGAJA TIDAK DI-SEED LEWAT MIGRASI
-- ------------------------------------------------------------
-- SEBELUMNYA blok ini menyisipkan 4 akun admin dengan SATU password
-- default yang tertulis terbuka di repository. Itu adalah backdoor
-- kredensial produksi (temuan CRITICAL audit Fase 1, RULES #9), karena
-- `make migrate-up` juga dijalankan di server produksi sehingga akun
-- tersebut langsung aktif dan dapat diambil alih siapa pun.
--
-- MIGRASI BUKAN JALUR DISTRIBUSI KREDENSIAL. Akun admin sekarang dibuat
-- lewat seeder terpisah yang WAJIB diberi password eksplisit dan hanya
-- boleh berjalan di environment development/test/local:
--
--     cd backend
--     $env:SEED_ADMIN_PASSWORD='<password-kuat>'    (PowerShell)
--     export SEED_ADMIN_PASSWORD='<password-kuat>'  (bash)
--     go run ./cmd/seed
--
-- Akun seed lama yang sudah terlanjur ada di database dinonaktifkan oleh
-- migrasi 000004_harden_fase1_security.up.sql.
