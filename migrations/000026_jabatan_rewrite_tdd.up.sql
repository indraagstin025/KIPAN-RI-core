-- ============================================================
-- MASTER JABATAN (TDD D14): tanpa level + is_ketua_umum
-- Version: 000026_jabatan_rewrite_tdd.up.sql
-- ============================================================
-- Jabatan tidak lagi berlevel (level ditentukan SK, bukan jabatan). Nama
-- menjadi unik global. Tambah penanda is_ketua_umum (etalase publik).
-- Baris berlevel (NASIONAL/PROVINSI/KABUPATEN) didedupe: pilih survivor
-- (prioritas NASIONAL, lalu id terkecil), remap pengurus ke survivor, hapus
-- duplikat. Sifat: IDEMPOTEN (aman diulang pada state yang sudah dimigrasi
-- sebagian lewat guard IF EXISTS).

ALTER TABLE jabatan ADD COLUMN IF NOT EXISTS is_ketua_umum BOOLEAN NOT NULL DEFAULT FALSE;

-- Tandai jabatan "Ketua" sebagai Ketua Umum (default; dapat diubah admin).
UPDATE jabatan SET is_ketua_umum = TRUE WHERE nama = 'Ketua';

-- Remap pengurus dari baris duplikat ke survivor per nama.
WITH ranked AS (
    SELECT id,
           first_value(id) OVER (
               PARTITION BY nama
               ORDER BY (level = 'NASIONAL') DESC, id ASC
           ) AS survivor_id,
           row_number() OVER (
               PARTITION BY nama
               ORDER BY (level = 'NASIONAL') DESC, id ASC
           ) AS rn
    FROM jabatan
)
UPDATE pengurus p
SET jabatan_id = r.survivor_id, updated_at = CURRENT_TIMESTAMP
FROM ranked r
WHERE p.jabatan_id = r.id AND r.rn > 1;

-- Hapus baris jabatan duplikat.
WITH ranked AS (
    SELECT id,
           row_number() OVER (
               PARTITION BY nama
               ORDER BY (level = 'NASIONAL') DESC, id ASC
           ) AS rn
    FROM jabatan
)
DELETE FROM jabatan j USING ranked r WHERE j.id = r.id AND r.rn > 1;

-- Ganti keunikan (nama,level) -> nama; buang kolom level.
ALTER TABLE jabatan DROP CONSTRAINT IF EXISTS uq_jabatan_nama_level;
ALTER TABLE jabatan DROP CONSTRAINT IF EXISTS chk_jabatan_level;
ALTER TABLE jabatan DROP CONSTRAINT IF EXISTS jabatan_nama_key;
ALTER TABLE jabatan DROP COLUMN IF EXISTS level;
ALTER TABLE jabatan DROP CONSTRAINT IF EXISTS uq_jabatan_nama;
ALTER TABLE jabatan ADD CONSTRAINT uq_jabatan_nama UNIQUE (nama);

COMMENT ON COLUMN jabatan.is_ketua_umum IS
    'Penanda jabatan Ketua Umum (dipakai etalase publik pimpinan).';
