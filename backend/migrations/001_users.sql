-- Migration 001: users table
CREATE TABLE IF NOT EXISTS users (
    id              BIGSERIAL PRIMARY KEY,
    email           VARCHAR(255) UNIQUE NOT NULL,
    password_hash   VARCHAR(255) NOT NULL,
    username        VARCHAR(64) UNIQUE NOT NULL,
    avatar_url      TEXT,
    bio             TEXT,
    reputation      INT NOT NULL DEFAULT 10,
    preferred_locale VARCHAR(10) NOT NULL DEFAULT 'zh-CN',
    role            VARCHAR(20) NOT NULL DEFAULT 'user',
    is_banned       BOOLEAN NOT NULL DEFAULT FALSE,
    ban_reason      TEXT,
    support_info    JSONB DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE INDEX IF NOT EXISTS idx_users_role ON users(role);
