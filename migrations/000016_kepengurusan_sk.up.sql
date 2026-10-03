-- ============================================================
-- KEPENGURUSAN: JABATAN, SURAT KEPUTUSAN, PENGURUS
-- Version: 000016_kepengurusan_sk.up.sql
-- ============================================================
-- Menopang modul pengangkatan kader -> pengurus
-- (IMPLEMENTASI_PENGANGKATAN.md). Pendaftaran hanya untuk kader; pengurus
-- diangkat lewat SK oleh Admin Kabupaten/Kota, disahkan berjenjang
-- KAB -> PROV -> NAS.
-- Sifat: IDEMPOTEN (aman dijalankan berkali-kali).
-- ============================================================

CREATE TABLE IF NOT EXISTS jabatan (
    id SERIAL PRIMARY KEY,
    nama VARCHAR(100) NOT NULL UNIQUE,
    is_inti BOOLEAN NOT NULL DEFAULT FALSE,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    urutan INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS surat_keputusan (
    id SERIAL PRIMARY KEY,
    nomor_sk VARCHAR(100) NOT NULL UNIQUE,
    judul VARCHAR(255) NOT NULL,
    level VARCHAR(20) NOT NULL,
    provinsi_id INT REFERENCES wilayah_provinsi(id) ON DELETE RESTRICT,
    kabupaten_id INT REFERENCES wilayah_kabupaten(id) ON DELETE RESTRICT,
    tanggal_terbit DATE NOT NULL,
    tanggal_berakhir DATE,
    file_sk_key VARCHAR(255) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'Aktif',
    approval_status VARCHAR(30) NOT NULL DEFAULT 'DRAFT',
    catatan_penolakan TEXT,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    approved_by UUID REFERENCES users(id) ON DELETE SET NULL,
    approved_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_sk_level CHECK (level IN ('NASIONAL', 'PROVINSI', 'KABUPATEN')),
    CONSTRAINT chk_sk_status CHECK (status IN ('Aktif', 'TidakAktif', 'Digantikan')),
    CONSTRAINT chk_sk_approval CHECK (approval_status IN ('DRAFT', 'MENUNGGU_PROVINSI', 'MENUNGGU_NASIONAL', 'DISETUJUI', 'DITOLAK'))
);

CREATE INDEX IF NOT EXISTS idx_sk_level_wilayah ON surat_keputusan (level, provinsi_id, kabupaten_id);
CREATE INDEX IF NOT EXISTS idx_sk_approval_status ON surat_keputusan (approval_status);
CREATE INDEX IF NOT EXISTS idx_sk_status ON surat_keputusan (status);

CREATE TABLE IF NOT EXISTS pengurus (
    id SERIAL PRIMARY KEY,
    anggota_id INT NOT NULL REFERENCES anggota(id) ON DELETE RESTRICT,
    surat_keputusan_id INT NOT NULL REFERENCES surat_keputusan(id) ON DELETE RESTRICT,
    level VARCHAR(20) NOT NULL,
    provinsi_id INT REFERENCES wilayah_provinsi(id) ON DELETE RESTRICT,
    kabupaten_id INT REFERENCES wilayah_kabupaten(id) ON DELETE RESTRICT,
    jabatan_id INT NOT NULL REFERENCES jabatan(id) ON DELETE RESTRICT,
    status VARCHAR(30) NOT NULL DEFAULT 'Aktif',
    keterangan_status TEXT,
    tanggal_mulai DATE NOT NULL,
    tanggal_selesai DATE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_pengurus_sk_anggota UNIQUE (surat_keputusan_id, anggota_id),
    CONSTRAINT chk_pengurus_level CHECK (level IN ('NASIONAL', 'PROVINSI', 'KABUPATEN'))
);

CREATE INDEX IF NOT EXISTS idx_pengurus_anggota ON pengurus (anggota_id, status);
CREATE INDEX IF NOT EXISTS idx_pengurus_sk ON pengurus (surat_keputusan_id);
CREATE INDEX IF NOT EXISTS idx_pengurus_jabatan ON pengurus (jabatan_id, status);

-- Seed jabatan master. Jabatan inti hanya boleh satu pemegang per SK.
INSERT INTO jabatan (nama, is_inti, is_active, urutan) VALUES
    ('Ketua', TRUE, TRUE, 1),
    ('Wakil Ketua', FALSE, TRUE, 2),
    ('Sekretaris', TRUE, TRUE, 3),
    ('Wakil Sekretaris', FALSE, TRUE, 4),
    ('Bendahara', TRUE, TRUE, 5),
    ('Wakil Bendahara', FALSE, TRUE, 6),
    ('Ketua Bidang', FALSE, TRUE, 7),
    ('Koordinator', FALSE, TRUE, 8),
    ('Anggota', FALSE, TRUE, 9)
ON CONFLICT (nama) DO NOTHING;
