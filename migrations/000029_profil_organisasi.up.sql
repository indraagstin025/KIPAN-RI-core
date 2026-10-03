-- ============================================================
-- PROFIL ORGANISASI (tabel 1-baris)
-- Version: 000029_profil_organisasi.up.sql
-- ============================================================
-- Menyimpan profil organisasi (nama, deskripsi, visi/misi, kontak, sosmed,
-- logo) untuk dikelola Super Admin & ditampilkan di halaman publik. Satu baris
-- (id = 1). Sifat: IDEMPOTEN.

CREATE TABLE IF NOT EXISTS organisasi_profile (
    id SMALLINT PRIMARY KEY DEFAULT 1,
    nama VARCHAR(150) NOT NULL DEFAULT '',
    singkatan VARCHAR(50) NOT NULL DEFAULT '',
    deskripsi TEXT NOT NULL DEFAULT '',
    visi TEXT NOT NULL DEFAULT '',
    misi TEXT NOT NULL DEFAULT '',
    alamat TEXT NOT NULL DEFAULT '',
    email VARCHAR(150) NOT NULL DEFAULT '',
    telepon VARCHAR(50) NOT NULL DEFAULT '',
    whatsapp VARCHAR(25) NOT NULL DEFAULT '',
    website VARCHAR(255) NOT NULL DEFAULT '',
    instagram VARCHAR(255) NOT NULL DEFAULT '',
    facebook VARCHAR(255) NOT NULL DEFAULT '',
    youtube VARCHAR(255) NOT NULL DEFAULT '',
    tiktok VARCHAR(255) NOT NULL DEFAULT '',
    logo_url VARCHAR(500) NOT NULL DEFAULT '',
    updated_by UUID REFERENCES users(id) ON DELETE SET NULL,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_organisasi_single_row CHECK (id = 1)
);

INSERT INTO organisasi_profile (id, nama, singkatan, deskripsi)
VALUES (1, 'Kader Inti Pemuda Anti Narkoba', 'KIPAN', 'Organisasi pembinaan pemuda anti narkoba di bawah Kementerian Pemuda dan Olahraga.')
ON CONFLICT (id) DO NOTHING;
