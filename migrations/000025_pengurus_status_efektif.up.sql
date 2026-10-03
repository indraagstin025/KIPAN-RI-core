-- ============================================================
-- STATUS EFEKTIF PENGURUS (TDD §5.4 — Dynamic Demisioner Check)
-- Version: 000025_pengurus_status_efektif.up.sql
-- ============================================================
-- Satu sumber kebenaran "seorang pengurus dianggap menjabat": record
-- berstatus 'Aktif' DAN SK terkait berstatus 'Aktif' DAN masa bakti
-- (tanggal_berakhir) belum lewat. Dipakai BERSAMA oleh lapis tampilan (view)
-- dan job materialisasi (worker) agar definisi tidak berbeda.
--
-- fungsi menerima p_ref_date agar deterministik (dipakai test & materialisasi);
-- view menyuntikkan CURRENT_DATE. Sifat: IDEMPOTEN.

CREATE OR REPLACE FUNCTION pengurus_status_efektif(
    p_pengurus_status TEXT,
    p_sk_status TEXT,
    p_sk_tanggal_berakhir DATE,
    p_ref_date DATE
) RETURNS TEXT
LANGUAGE sql IMMUTABLE AS $$
    SELECT CASE
        WHEN p_pengurus_status <> 'Aktif' THEN p_pengurus_status
        WHEN p_sk_status = 'Aktif'
             AND (p_sk_tanggal_berakhir IS NULL OR p_sk_tanggal_berakhir >= p_ref_date)
            THEN 'Aktif'
        ELSE 'Demisioner'
    END;
$$;

CREATE OR REPLACE VIEW v_pengurus_efektif AS
SELECT
    p.*,
    pengurus_status_efektif(p.status, sk.status, sk.tanggal_berakhir, CURRENT_DATE) AS status_efektif,
    sk.tanggal_berakhir AS sk_tanggal_berakhir
FROM pengurus p
JOIN surat_keputusan sk ON sk.id = p.surat_keputusan_id;

COMMENT ON FUNCTION pengurus_status_efektif(TEXT, TEXT, DATE, DATE) IS
    'Status efektif pengurus: Aktif hanya bila record Aktif + SK Aktif + masa bakti belum lewat (TDD §5.4).';
COMMENT ON VIEW v_pengurus_efektif IS
    'Proyeksi pengurus + status_efektif (satu sumber kebenaran status menjabat).';
