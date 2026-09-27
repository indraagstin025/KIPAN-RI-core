-- ============================================================
-- 5. ACTIVITY LOGS (AUDIT TRAIL FORENSIK, APPEND-ONLY)
-- Version: 000005_activity_logs.up.sql
-- ============================================================
-- Menutup temuan audit M-4 (RULES 16, 21):
-- struct domain.ActivityLog sudah ada, tetapi tabel dan writer-nya
-- belum ada sehingga mutasi sensitif tanpa jejak forensik.
--
-- Sifat: APPEND-ONLY. UPDATE/DELETE ditolak trigger di bawah, sehingga
-- berlaku untuk semua role DB (bukan sekadar konvensi aplikasi).
-- PII (NIK utuh, token, password) TIDAK BOLEH ditulis ke metadata —
-- service wajib memasking sebelum insert.
-- ============================================================

CREATE TABLE IF NOT EXISTS activity_logs (
    id BIGSERIAL PRIMARY KEY,
    actor_id UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    actor_name VARCHAR(150) NOT NULL,
    actor_role VARCHAR(50) NOT NULL,
    ip_address VARCHAR(45) NOT NULL,
    user_agent TEXT NULL,
    entity_name VARCHAR(50) NOT NULL,
    entity_id VARCHAR(100) NOT NULL,
    action VARCHAR(50) NOT NULL,
    metadata JSONB NULL,
    request_id VARCHAR(100) NOT NULL DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_activity_actor
    ON activity_logs(actor_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_activity_entity
    ON activity_logs(entity_name, entity_id);
CREATE INDEX IF NOT EXISTS idx_activity_created
    ON activity_logs(created_at DESC);

-- Penjaga append-only: tolak UPDATE dan DELETE dalam kondisi apapun.
CREATE OR REPLACE FUNCTION prevent_activity_logs_mutation()
RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'activity_logs bersifat append-only: % tidak diizinkan', TG_OP;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_activity_logs_no_update ON activity_logs;
CREATE TRIGGER trg_activity_logs_no_update
BEFORE UPDATE OR DELETE ON activity_logs
FOR EACH ROW EXECUTE FUNCTION prevent_activity_logs_mutation();
