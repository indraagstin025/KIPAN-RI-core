-- ============================================================
-- LARANGAN JABATAN GANDA LINTAS TINGKAT (TDD D10)
-- Version: 000027_pengurus_single_active.up.sql
-- ============================================================
-- Satu anggota hanya boleh memiliki SATU jabatan pengurus berstatus 'Aktif'
-- pada satu waktu, lintas tingkat/SK. Alur pengangkatan sudah menutup record
-- lama sebelum membuka baru (AddWithPromotion); indeks unik parsial ini adalah
-- jaring pengaman DB terhadap kondisi balapan / jalur tulis lain (mis.
-- reaktivasi status). Sifat: IDEMPOTEN.

-- Cleanup defensif: bila (karena data lama) ada >1 baris Aktif per anggota,
-- pertahankan yang paling baru (tanggal_mulai, lalu id) dan demosikan sisanya.
WITH ranked AS (
    SELECT id,
           row_number() OVER (
               PARTITION BY anggota_id
               ORDER BY tanggal_mulai DESC, id DESC
           ) AS rn
    FROM pengurus
    WHERE status = 'Aktif'
)
UPDATE pengurus p
SET status = 'Demisioner',
    keterangan_status = COALESCE(
        NULLIF(p.keterangan_status, ''),
        'Otomatis: penegakan aturan satu jabatan aktif per anggota'
    ),
    tanggal_selesai = COALESCE(p.tanggal_selesai, CURRENT_DATE),
    updated_at = CURRENT_TIMESTAMP
FROM ranked r
WHERE p.id = r.id AND r.rn > 1;

CREATE UNIQUE INDEX IF NOT EXISTS uq_pengurus_single_active
    ON pengurus (anggota_id) WHERE status = 'Aktif';

COMMENT ON INDEX uq_pengurus_single_active IS
    'Satu anggota hanya satu jabatan pengurus Aktif (TDD D10).';
