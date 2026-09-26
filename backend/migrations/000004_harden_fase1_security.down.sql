-- ============================================================
-- ROLLBACK: 000004_harden_fase1_security
-- ============================================================
-- ASIMETRIS DENGAN SENGAJA.
--
-- Bagian 1 (penonaktifan akun seed dengan password default publik) TIDAK
-- dibalik. Mengaktifkan kembali akun dengan kredensial yang sudah beredar
-- publik akan mengembalikan kerentanan CRITICAL (RULES #9), sehingga tidak
-- layak dijadikan operasi rollback otomatis.
--
-- Untuk mengaktifkan kembali akun tersebut dengan password BARU:
--     cd backend
--     $env:SEED_ADMIN_PASSWORD='<password-kuat>'; go run ./cmd/seed
--
-- Yang dibalik hanyalah perubahan struktur (foreign key), karena itu murni
-- perubahan skema tanpa implikasi keamanan.
-- ============================================================

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_provinsi_id_fkey;
ALTER TABLE users
    ADD CONSTRAINT users_provinsi_id_fkey
    FOREIGN KEY (provinsi_id) REFERENCES wilayah_provinsi (id) ON DELETE SET NULL;

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_kabupaten_id_fkey;
ALTER TABLE users
    ADD CONSTRAINT users_kabupaten_id_fkey
    FOREIGN KEY (kabupaten_id) REFERENCES wilayah_kabupaten (id) ON DELETE SET NULL;
