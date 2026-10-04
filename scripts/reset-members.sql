-- ============================================================
-- RESET DATA KEANGGOTAAN & KEPENGURUSAN (DEV/STAGING)
-- ============================================================
-- Menghapus SELURUH anggota, pengurus, pendaftaran (+riwayat), SK, akun
-- USER anggota, dan antrian email; lalu mereset penomoran (NIA & nomor
-- registrasi) agar mulai dari 001 lagi.
-- TIDAK menghapus: akun admin (role non-USER), activity_logs (audit),
-- master wilayah/jabatan, organisasi_profile, backups.
-- DESTRUKTIF: jalankan setelah backup. Sifat: transaksional.
-- ============================================================
BEGIN;

-- 1) Kepengurusan (pengurus.anggota_id & pengurus.surat_keputusan_id RESTRICT)
DELETE FROM pengurus;

-- 2) Anggota
DELETE FROM anggota;

-- 3) Pendaftaran (riwayat dulu karena pendaftaran_riwayat.pendaftaran_id RESTRICT)
DELETE FROM pendaftaran_riwayat;
DELETE FROM pendaftaran;

-- 4) Surat Keputusan (setelah pengurus)
DELETE FROM surat_keputusan;

-- 5) Antrian email
DELETE FROM email_outbox;

-- 6) Akun USER anggota (JANGAN sentuh akun admin).
--    FK activity_logs.actor_id ON DELETE SET NULL akan meng-UPDATE activity_logs,
--    sedangkan tabel itu append-only (trigger trg_activity_logs_no_update).
--    Di DEV, trigger append-only dinonaktifkan SEMENTARA agar penghapusan akun
--    (dan SET NULL pada audit) dapat berjalan; setelahnya diaktifkan kembali.
ALTER TABLE activity_logs DISABLE TRIGGER USER;
DELETE FROM users WHERE role = 'USER';
ALTER TABLE activity_logs ENABLE TRIGGER USER;

-- 7) Reset penomoran
TRUNCATE TABLE anggota_nia_sequence;
TRUNCATE TABLE pendaftaran_nomor_sequence;

COMMIT;
