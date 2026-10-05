-- ROLLBACK 000031_pengurus_expiry_notif

ALTER TABLE pengurus
    DROP COLUMN IF EXISTS notified_h30_at,
    DROP COLUMN IF EXISTS notified_h7_at;
