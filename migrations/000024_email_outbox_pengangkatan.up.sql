-- ============================================================
-- OUTBOX: JENIS PENGANGKATAN
-- Version: 000024_email_outbox_pengangkatan.up.sql
-- ============================================================
-- Menambah jenis email 'PENGANGKATAN' (notifikasi kader diangkat jadi
-- pengurus) agar pengiriman konsisten via antrian/worker. Sifat: IDEMPOTEN.

ALTER TABLE email_outbox DROP CONSTRAINT IF EXISTS chk_email_outbox_jenis;
ALTER TABLE email_outbox
    ADD CONSTRAINT chk_email_outbox_jenis CHECK (jenis IN (
        'STATUS_DISETUJUI', 'STATUS_DITOLAK', 'STATUS_PERBAIKAN',
        'SET_PASSWORD', 'AKUN_TERHUBUNG', 'PENGANGKATAN'
    ));
