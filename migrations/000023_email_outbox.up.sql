-- ============================================================
-- EMAIL OUTBOX (ANTRIAN PENGIRIMAN EMAIL STATUS & KREDENSIAL)
-- Version: 000023_email_outbox.up.sql
-- ============================================================
-- Antrian persisten pengiriman email (transactional outbox). Baris ditulis
-- pada transaksi yang sama dengan perubahan status; worker mengirim pending
-- lalu menandai sent/failed. Token set-password TIDAK disimpan di sini.
-- Sifat: IDEMPOTEN.

CREATE TABLE IF NOT EXISTS email_outbox (
    id              BIGSERIAL PRIMARY KEY,
    jenis           VARCHAR(40) NOT NULL,
    pendaftaran_id  INTEGER,
    user_id         UUID,
    provinsi_id     INTEGER,
    kabupaten_id    INTEGER,
    to_email        VARCHAR(255) NOT NULL,
    subject         VARCHAR(255) NOT NULL,
    text_body       TEXT NOT NULL,
    html_body       TEXT,
    status          VARCHAR(20) NOT NULL DEFAULT 'pending',
    attempts        INTEGER NOT NULL DEFAULT 0,
    next_retry_at   TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    sent_at         TIMESTAMP WITH TIME ZONE,
    last_error      TEXT,
    created_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT chk_email_outbox_jenis CHECK (jenis IN (
        'STATUS_DISETUJUI', 'STATUS_DITOLAK', 'STATUS_PERBAIKAN',
        'SET_PASSWORD', 'AKUN_TERHUBUNG'
    )),
    CONSTRAINT chk_email_outbox_status CHECK (status IN ('pending', 'sent', 'failed'))
);

CREATE INDEX IF NOT EXISTS idx_email_outbox_pending
    ON email_outbox (status, next_retry_at)
    WHERE status = 'pending';

CREATE INDEX IF NOT EXISTS idx_email_outbox_scope
    ON email_outbox (provinsi_id, kabupaten_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_email_outbox_pendaftaran
    ON email_outbox (pendaftaran_id);
