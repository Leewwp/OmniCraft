CREATE TABLE IF NOT EXISTS content_embeddings (
    content_item_id BIGINT       PRIMARY KEY REFERENCES content_items(id) ON DELETE CASCADE,
    embedding       vector(1536) NOT NULL,
    embedded_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_content_embeddings_ivfflat
    ON content_embeddings USING ivfflat (embedding vector_cosine_ops)
    WITH (lists = 100);
