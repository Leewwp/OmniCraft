CREATE TABLE IF NOT EXISTS llm_configs (
    id           BIGSERIAL    PRIMARY KEY,
    config_name  VARCHAR(100) NOT NULL,
    provider_type VARCHAR(50) NOT NULL,
    api_base     VARCHAR(500),
    model        VARCHAR(100) NOT NULL,
    api_key_enc  TEXT,
    is_active    BOOLEAN      NOT NULL DEFAULT FALSE,
    extra_params JSONB        NOT NULL DEFAULT '{}',
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_llm_configs_active
    ON llm_configs (is_active) WHERE is_active = TRUE;
