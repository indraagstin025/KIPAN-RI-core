-- ============================================================
-- 9. NOTIFIKASI IN-APP MINIMAL (PENYESUAIAN FASE 1-2)
-- Version: 000009_notifications.up.sql
-- ============================================================
-- Pengganti aman notifyAdmins proyek lama: fan-out dilakukan server-side
-- (INSERT...SELECT berdasar role + wilayah admin aktif), bukan dari
-- parameter klien. Baca dibatasi milik sendiri (WHERE user_id = klaim JWT)
-- sehingga celah IDOR ?userId= tidak terulang.
-- ============================================================

CREATE TABLE IF NOT EXISTS notifications (
    id BIGSERIAL PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(150) NOT NULL,
    message TEXT NOT NULL,
    type VARCHAR(30) NOT NULL,
    link VARCHAR(255),
    is_read BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_notifications_user
    ON notifications(user_id, created_at DESC);
