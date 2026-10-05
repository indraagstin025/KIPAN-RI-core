-- ============================================================
-- FLAG NOTIFIKASI KEDALUWARSA MASA BAKTI
-- Version: 000031_pengurus_expiry_notif.up.sql
-- ============================================================
-- Penanda anti-duplikat notifikasi peringatan H-30/H-7 per baris pengurus.
-- Dipakai job worker pengurus-expiry-warning. Sifat: IDEMPOTEN.
-- Catatan: bila SK diperpanjang setelah flag terkirim, flag basi dan tidak
-- direset otomatis (kasus langka; admin sudah ternotifikasi).

ALTER TABLE pengurus
    ADD COLUMN IF NOT EXISTS notified_h30_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS notified_h7_at TIMESTAMPTZ;

COMMENT ON COLUMN pengurus.notified_h30_at IS
    'Waktu notifikasi peringatan H-30 terkirim (NULL = belum).';
COMMENT ON COLUMN pengurus.notified_h7_at IS
    'Waktu notifikasi peringatan H-7 terkirim (NULL = belum).';
