-- ============================================================
-- ROLLBACK 000009_notifications
-- ============================================================

DROP INDEX IF EXISTS idx_notifications_user;
DROP TABLE IF EXISTS notifications;
