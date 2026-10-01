-- ============================================================
-- 3. MEMBERSHIP / PENDAFTARAN & ANGGOTA
-- Focus: secure intake, verification, approval, audit trail
-- ============================================================

CREATE TABLE IF NOT EXISTS pendaftaran (
    id SERIAL PRIMARY KEY,
    nomor_pendaftaran VARCHAR(30) NOT NULL UNIQUE,
    nama_lengkap VARCHAR(150) NOT NULL,
    nik_hash CHAR(64) NOT NULL,
    nik_encrypted TEXT NOT NULL,
    tempat_lahir VARCHAR(100) NOT NULL,
    tanggal_lahir TIMESTAMP WITH TIME ZONE NOT NULL,
    jenis_kelamin VARCHAR(2) NOT NULL,
    agama VARCHAR(30),
    pendidikan VARCHAR(50),
    pekerjaan VARCHAR(100),
    status_pribadi VARCHAR(50),
    alamat TEXT NOT NULL,
    provinsi_id INT NOT NULL REFERENCES wilayah_provinsi(id) ON DELETE RESTRICT,
    kabupaten_id INT NOT NULL REFERENCES wilayah_kabupaten(id) ON DELETE RESTRICT,
    kecamatan VARCHAR(100),
    desa VARCHAR(100),
    kode_pos VARCHAR(10),
    email VARCHAR(255) NOT NULL,
    whatsapp VARCHAR(25) NOT NULL,
    motivasi TEXT,
    foto_key VARCHAR(255),
    ktp_key VARCHAR(255),
    cv_key VARCHAR(255),
    sk_key VARCHAR(255),
    surat_pernyataan_key VARCHAR(255),
    surat_sehat_key VARCHAR(255),
    status VARCHAR(30) NOT NULL DEFAULT 'DIAJUKAN',
    catatan_perbaikan TEXT,
    revisi_token_hash CHAR(64),
    revisi_token_expires_at TIMESTAMP WITH TIME ZONE,
    anggota_id INT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_pendaftaran_status ON pendaftaran(status);
CREATE INDEX IF NOT EXISTS idx_pendaftaran_wilayah ON pendaftaran(provinsi_id, kabupaten_id);
CREATE INDEX IF NOT EXISTS idx_pendaftaran_nomor ON pendaftaran(nomor_pendaftaran);
CREATE UNIQUE INDEX IF NOT EXISTS uq_pendaftaran_nik_hash ON pendaftaran(nik_hash);

CREATE TABLE IF NOT EXISTS pendaftaran_riwayat (
    id SERIAL PRIMARY KEY,
    pendaftaran_id INT NOT NULL REFERENCES pendaftaran(id) ON DELETE CASCADE,
    aksi VARCHAR(30) NOT NULL,
    actor_id UUID,
    actor_name VARCHAR(150),
    actor_role VARCHAR(30),
    catatan TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_pendaftaran_riwayat ON pendaftaran_riwayat(pendaftaran_id, created_at);

CREATE TABLE IF NOT EXISTS anggota (
    id SERIAL PRIMARY KEY,
    nia VARCHAR(50) NOT NULL UNIQUE,
    nama_lengkap VARCHAR(150) NOT NULL,
    nik_hash CHAR(64) NOT NULL UNIQUE,
    nik_encrypted TEXT NOT NULL,
    tempat_lahir VARCHAR(100) NOT NULL,
    tanggal_lahir TIMESTAMP WITH TIME ZONE NOT NULL,
    jenis_kelamin VARCHAR(2) NOT NULL,
    agama VARCHAR(30),
    pendidikan VARCHAR(50),
    pekerjaan VARCHAR(100),
    alamat TEXT NOT NULL,
    provinsi_id INT NOT NULL REFERENCES wilayah_provinsi(id) ON DELETE RESTRICT,
    kabupaten_id INT NOT NULL REFERENCES wilayah_kabupaten(id) ON DELETE RESTRICT,
    kecamatan VARCHAR(100),
    desa VARCHAR(100),
    kode_pos VARCHAR(10),
    email VARCHAR(255) NOT NULL,
    whatsapp VARCHAR(25) NOT NULL,
    foto_key VARCHAR(255),
    ktp_key VARCHAR(255),
    cv_key VARCHAR(255),
    sk_key VARCHAR(255),
    surat_pernyataan_key VARCHAR(255),
    surat_sehat_key VARCHAR(255),
    status VARCHAR(30) NOT NULL DEFAULT 'AKTIF',
    angkatan VARCHAR(20) NOT NULL,
    kta_qr_hash CHAR(64),
    kta_pdf_key VARCHAR(255),
    pendaftaran_id INT REFERENCES pendaftaran(id) ON DELETE SET NULL,
    user_id UUID,
    tanggal_daftar TIMESTAMP WITH TIME ZONE NOT NULL,
    tanggal_angkat TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_anggota_wilayah ON anggota(provinsi_id, kabupaten_id);
CREATE INDEX IF NOT EXISTS idx_anggota_status ON anggota(status);

CREATE TABLE IF NOT EXISTS anggota_nia_sequence (
    provinsi_id INT NOT NULL,
    kabupaten_id INT NOT NULL,
    tahun INT NOT NULL,
    next_value INT NOT NULL DEFAULT 1,
    PRIMARY KEY (provinsi_id, kabupaten_id, tahun)
);
