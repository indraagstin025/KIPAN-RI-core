-- ROLLBACK 000024_email_outbox_pengangkatan
-- Menghapus jenis 'PENGANGKATAN'. Akan gagal bila masih ada baris berjenis
-- tersebut (hapus/arsipkan lebih dahulu) — rollback destruktif best-effort.

DELETE FROM email_outbox WHERE jenis = 'PENGANGKATAN';

ALTER TABLE email_outbox DROP CONSTRAINT IF EXISTS chk_email_outbox_jenis;
ALTER TABLE email_outbox
    ADD CONSTRAINT chk_email_outbox_jenis CHECK (jenis IN (
        'STATUS_DISETUJUI', 'STATUS_DITOLAK', 'STATUS_PERBAIKAN',
        'SET_PASSWORD', 'AKUN_TERHUBUNG'
    ));
