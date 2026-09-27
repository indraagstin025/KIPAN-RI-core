-- ============================================================
-- ROLLBACK: 000002_seed_initial_data
-- ============================================================
-- CATATAN: migrasi ini tidak lagi menyisipkan akun admin, jadi `down`
-- TIDAK menyentuh tabel users sama sekali. Perintah lama `DELETE FROM users;`
-- bersifat destruktif (menghapus SELURUH akun, bukan hanya akun seed).
-- Penonaktifan akun seed lama ada di 000004_harden_fase1_security.up.sql.
DELETE FROM wilayah_kabupaten WHERE kode IN ('3273', '3204', '3171');
DELETE FROM wilayah_provinsi WHERE kode IN ('31', '32', '33', '35');