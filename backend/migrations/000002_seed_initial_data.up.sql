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

-- 2. SEED AKUN PENGUJIAN 4 LEVEL RBAC
-- Password default semua akun pengujian: AdminKipan2026!
-- Hash Argon2id: $argon2id$v=19$m=65536,t=1,p=12$0FOIel2hzFOFaiWLOKQ0Gg$OM4fW0400Z1hvOX7A/c7PiuqmNhywirXBfj55NPoDNE

-- Level 1: SUPER_ADMIN
INSERT INTO users (email, password_hash, name, role, status) VALUES
('superadmin@kipan.id', '$argon2id$v=19$m=65536,t=1,p=12$0FOIel2hzFOFaiWLOKQ0Gg$OM4fW0400Z1hvOX7A/c7PiuqmNhywirXBfj55NPoDNE', 'Super Administrator Pusat', 'SUPER_ADMIN', 'Aktif')
ON CONFLICT DO NOTHING;

-- Level 2: ADMIN_NASIONAL
INSERT INTO users (email, password_hash, name, role, status) VALUES
('adminnasional@kipan.id', '$argon2id$v=19$m=65536,t=1,p=12$0FOIel2hzFOFaiWLOKQ0Gg$OM4fW0400Z1hvOX7A/c7PiuqmNhywirXBfj55NPoDNE', 'Sekretariat DPP KIPAN Nasional', 'ADMIN_NASIONAL', 'Aktif')
ON CONFLICT DO NOTHING;

-- Level 3: ADMIN_PROVINSI (Jawa Barat)
INSERT INTO users (email, password_hash, name, role, status, provinsi_id)
SELECT 'adminprov.jabar@kipan.id', '$argon2id$v=19$m=65536,t=1,p=12$0FOIel2hzFOFaiWLOKQ0Gg$OM4fW0400Z1hvOX7A/c7PiuqmNhywirXBfj55NPoDNE', 'Admin DPD KIPAN Jawa Barat', 'ADMIN_PROVINSI', 'Aktif', p.id
FROM wilayah_provinsi p WHERE p.kode = '32'
ON CONFLICT DO NOTHING;

-- Level 4: ADMIN_KABUPATEN (Kota Bandung)
INSERT INTO users (email, password_hash, name, role, status, provinsi_id, kabupaten_id)
SELECT 'adminkab.bandung@kipan.id', '$argon2id$v=19$m=65536,t=1,p=12$0FOIel2hzFOFaiWLOKQ0Gg$OM4fW0400Z1hvOX7A/c7PiuqmNhywirXBfj55NPoDNE', 'Admin DPC KIPAN Kota Bandung', 'ADMIN_KABUPATEN', 'Aktif', k.provinsi_id, k.id
FROM wilayah_kabupaten k WHERE k.kode = '3273'
ON CONFLICT DO NOTHING;
