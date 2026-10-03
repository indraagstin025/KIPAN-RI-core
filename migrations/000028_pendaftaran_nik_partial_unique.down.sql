-- ROLLBACK 000028_pendaftaran_nik_partial_unique
-- Mengembalikan keunikan global. Akan GAGAL bila ada pendaftaran DITOLAK dan
-- non-DITOLAK dengan nik_hash sama (bersihkan lebih dahulu).

DROP INDEX IF EXISTS uq_pendaftaran_nik_hash;
CREATE UNIQUE INDEX IF NOT EXISTS uq_pendaftaran_nik_hash ON pendaftaran (nik_hash);
