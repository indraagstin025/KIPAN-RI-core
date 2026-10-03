-- ============================================================
-- FK ACTOR PADA RIWAYAT PENDAFTARAN (Batch 5 / T8)
-- Version: 000012_riwayat_actor_fk.up.sql
-- ============================================================
-- Kolom pendaftaran_riwayat.actor_id sebelumnya UUID polos (tanpa FK),
-- sehingga bisa merujuk user yang tidak ada. Ditambahkan FK ke users(id)
-- dengan ON DELETE SET NULL (audit trail tetap utuh saat user dihapus).
-- Baris yatim dibersihkan (actor_id -> NULL) sebelum constraint dipasang.
-- Idempoten.
-- ============================================================

UPDATE pendaftaran_riwayat r
SET actor_id = NULL
WHERE r.actor_id IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM users u WHERE u.id = r.actor_id);

ALTER TABLE pendaftaran_riwayat DROP CONSTRAINT IF EXISTS fk_riwayat_actor;
ALTER TABLE pendaftaran_riwayat
    ADD CONSTRAINT fk_riwayat_actor
    FOREIGN KEY (actor_id) REFERENCES users (id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_riwayat_actor ON pendaftaran_riwayat (actor_id);
