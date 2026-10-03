-- ============================================================
-- MONITORING: pg_stat_statements
-- Version: 000022_pg_stat_statements.up.sql
-- ============================================================
-- Menyediakan statistik query untuk tuning (slow query, query terberat).
-- Untuk data lengkap, tambahkan `pg_stat_statements` ke shared_preload_libraries
-- di postgresql.conf lalu restart server. CREATE EXTENSION tetap aman tanpa itu.
-- Sifat: IDEMPOTEN.

CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
