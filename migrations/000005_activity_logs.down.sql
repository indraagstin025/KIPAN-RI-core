-- ============================================================
-- ROLLBACK 000005_activity_logs
-- ============================================================
-- DROP TABLE adalah DDL dan tidak memicu row trigger append-only
-- (trigger hanya menolak UPDATE/DELETE DML), sehingga rollback aman.
-- ============================================================

DROP TRIGGER IF EXISTS trg_activity_logs_no_update ON activity_logs;
DROP FUNCTION IF EXISTS prevent_activity_logs_mutation();
DROP TABLE IF EXISTS activity_logs;
