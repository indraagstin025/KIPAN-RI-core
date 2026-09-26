-- ============================================================
-- SIM-KIPAN CORE DATABASE SCHEMA (FOKUS: AUTH & AUTHORIZATION)
-- PostgreSQL 16 Migration
-- Version: 000001_init_schema.up.sql
-- ============================================================

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ============================================================
-- 1. MASTER WILAYAH REFERENSI ROLE ADMIN (BPS)
-- ============================================================

CREATE TABLE IF NOT EXISTS wilayah_provinsi (
    id SERIAL PRIMARY KEY,
    kode VARCHAR(10) NOT NULL UNIQUE,
    nama VARCHAR(100) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS wilayah_kabupaten (
    id SERIAL PRIMARY KEY,
    provinsi_id INT NOT NULL REFERENCES wilayah_provinsi(id) ON DELETE RESTRICT,
    kode VARCHAR(10) NOT NULL UNIQUE,
    nama VARCHAR(100) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- ============================================================
-- 2. USERS (ADMIN SYSTEM)
-- Soft Delete: deleted_at + Partial Unique Index pada email
-- ============================================================

CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    name VARCHAR(150) NOT NULL,
    role VARCHAR(30) NOT NULL, -- SUPER_ADMIN, ADMIN_NASIONAL, ADMIN_PROVINSI, ADMIN_KABUPATEN
    status VARCHAR(20) NOT NULL DEFAULT 'Aktif', -- Aktif, Nonaktif, Suspended
    avatar_url VARCHAR(255),
    provinsi_id INT REFERENCES wilayah_provinsi(id) ON DELETE SET NULL,
    kabupaten_id INT REFERENCES wilayah_kabupaten(id) ON DELETE SET NULL,
    last_login_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_users_email_active ON users (email) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_users_role ON users (role);
CREATE INDEX IF NOT EXISTS idx_users_wilayah ON users (provinsi_id, kabupaten_id);

-- ============================================================
-- 3. REFRESH TOKENS (REFRESH TOKEN ROTATION & REUSE DETECTION)
-- ============================================================

CREATE TABLE IF NOT EXISTS user_refresh_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash CHAR(64) NOT NULL,
    family_id UUID NOT NULL,
    is_revoked BOOLEAN NOT NULL DEFAULT FALSE,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_refresh_token_hash ON user_refresh_tokens (token_hash);
CREATE INDEX IF NOT EXISTS idx_refresh_family ON user_refresh_tokens (family_id);
