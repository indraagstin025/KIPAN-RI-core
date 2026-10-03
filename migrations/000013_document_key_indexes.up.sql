-- ============================================================
-- INDEX KOLOM OBJECT KEY DOKUMEN (Batch 5 / T11)
-- Version: 000013_document_key_indexes.up.sql
-- ============================================================
-- documentRepo.ResolveOwner mencocokkan satu object key ke 6 kolom
-- dokumen (foto/ktp/cv/sk/surat_pernyataan/surat_sehat) di tabel
-- pendaftaran & anggota. Tanpa index, query OR tersebut melakukan seq
-- scan dan makin lambat seiring bertambahnya data. Partial index
-- (hanya baris dengan key terisi) membuat BitmapOr efisien.
-- Idempoten.
-- ============================================================

CREATE INDEX IF NOT EXISTS idx_pendaftaran_foto_key ON pendaftaran (foto_key) WHERE foto_key IS NOT NULL AND foto_key <> '';
CREATE INDEX IF NOT EXISTS idx_pendaftaran_ktp_key ON pendaftaran (ktp_key) WHERE ktp_key IS NOT NULL AND ktp_key <> '';
CREATE INDEX IF NOT EXISTS idx_pendaftaran_cv_key ON pendaftaran (cv_key) WHERE cv_key IS NOT NULL AND cv_key <> '';
CREATE INDEX IF NOT EXISTS idx_pendaftaran_sk_key ON pendaftaran (sk_key) WHERE sk_key IS NOT NULL AND sk_key <> '';
CREATE INDEX IF NOT EXISTS idx_pendaftaran_surat_pernyataan_key ON pendaftaran (surat_pernyataan_key) WHERE surat_pernyataan_key IS NOT NULL AND surat_pernyataan_key <> '';
CREATE INDEX IF NOT EXISTS idx_pendaftaran_surat_sehat_key ON pendaftaran (surat_sehat_key) WHERE surat_sehat_key IS NOT NULL AND surat_sehat_key <> '';

CREATE INDEX IF NOT EXISTS idx_anggota_foto_key ON anggota (foto_key) WHERE foto_key IS NOT NULL AND foto_key <> '';
CREATE INDEX IF NOT EXISTS idx_anggota_ktp_key ON anggota (ktp_key) WHERE ktp_key IS NOT NULL AND ktp_key <> '';
CREATE INDEX IF NOT EXISTS idx_anggota_cv_key ON anggota (cv_key) WHERE cv_key IS NOT NULL AND cv_key <> '';
CREATE INDEX IF NOT EXISTS idx_anggota_sk_key ON anggota (sk_key) WHERE sk_key IS NOT NULL AND sk_key <> '';
CREATE INDEX IF NOT EXISTS idx_anggota_surat_pernyataan_key ON anggota (surat_pernyataan_key) WHERE surat_pernyataan_key IS NOT NULL AND surat_pernyataan_key <> '';
CREATE INDEX IF NOT EXISTS idx_anggota_surat_sehat_key ON anggota (surat_sehat_key) WHERE surat_sehat_key IS NOT NULL AND surat_sehat_key <> '';
