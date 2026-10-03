-- ============================================================
-- RIWAYAT BACKUP DATABASE
-- Version: 000030_backups.up.sql
-- ============================================================
-- Menyimpan riwayat backup (pg_dump) yang dibuat server-side & diunggah ke
-- bucket private. Sifat: IDEMPOTEN.

CREATE TABLE IF NOT EXISTS backups (
    id SERIAL PRIMARY KEY,
    filename VARCHAR(255) NOT NULL,
    object_key VARCHAR(500) NOT NULL,
    size_bytes BIGINT NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'ready',
    error TEXT,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_backup_status CHECK (status IN ('ready', 'failed'))
);

CREATE INDEX IF NOT EXISTS idx_backups_created ON backups (created_at DESC);
