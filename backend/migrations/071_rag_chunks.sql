CREATE EXTENSION IF NOT EXISTS vector;

-- Existing published rows predate the content-version invariant. Give each
-- one a full, active version-1 snapshot before the RAG projection is built.
INSERT INTO content_versions (
    content_item_id, author_id, version_number, storage_type, storage_key,
    diff_summary, status, is_latest, created_at
)
SELECT ci.id, ci.author_id, 1, 'full', ci.description,
       'migration 071 initial snapshot', 'active', TRUE, ci.created_at
FROM content_items ci
WHERE ci.status = 'published'
  AND ci.deleted_at IS NULL
  AND NOT EXISTS (
      SELECT 1 FROM content_versions cv WHERE cv.content_item_id = ci.id
  )
ON CONFLICT (content_item_id, version_number) DO NOTHING;

CREATE TABLE IF NOT EXISTS rag_chunks (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    content_id BIGINT NOT NULL REFERENCES content_items(id) ON DELETE CASCADE,
    content_version INT NOT NULL,
    chunk_index INT NOT NULL CHECK (chunk_index >= 0),
    chunk_key CHAR(64) NOT NULL,
    chunking_version INT NOT NULL CHECK (chunking_version > 0),
    heading TEXT NOT NULL DEFAULT '',
    text TEXT NOT NULL,
    source_start INT NOT NULL CHECK (source_start >= 0),
    source_end INT NOT NULL CHECK (source_end > source_start),
    zone VARCHAR(10) NOT NULL,
    content_type VARCHAR(20) NOT NULL,
    category VARCHAR(50),
    ip BIGINT REFERENCES ips(id) ON DELETE SET NULL,
    tags TEXT[] NOT NULL DEFAULT '{}',
    index_version INT NOT NULL CHECK (index_version > 0),
    -- Deduplicates deterministic chunk identity within one index generation.
    CONSTRAINT uq_rag_chunks_generation_key UNIQUE (index_version, chunk_key),
    -- Prevents duplicate ordering slots during generation staging/replay.
    CONSTRAINT uq_rag_chunks_generation_order UNIQUE (
        content_id, content_version, chunking_version, index_version, chunk_index
    )
);

ALTER TABLE rag_chunks DROP CONSTRAINT IF EXISTS fk_rag_chunks_content_version;
ALTER TABLE rag_chunks ADD CONSTRAINT fk_rag_chunks_content_version
    FOREIGN KEY (content_id, content_version)
    REFERENCES content_versions(content_item_id, version_number) ON DELETE CASCADE;

-- Supports content-version projection replacement and cascade cleanup.
CREATE INDEX IF NOT EXISTS idx_rag_chunks_content ON rag_chunks(content_id, content_version);
-- Supports generation-scoped chunk reads and rebuild bookkeeping.
CREATE INDEX IF NOT EXISTS idx_rag_chunks_generation ON rag_chunks(index_version, chunking_version);
-- Supports IP visibility filtering before retrieval ranking.
CREATE INDEX IF NOT EXISTS idx_rag_chunks_ip ON rag_chunks(ip);

CREATE TABLE IF NOT EXISTS chunk_embeddings (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    chunk_id BIGINT NOT NULL REFERENCES rag_chunks(id) ON DELETE CASCADE,
    embedding vector(1536) NOT NULL,
    embedding_model VARCHAR(100) NOT NULL,
    embedded_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- Prevents duplicate provider-model embeddings for the same chunk.
    CONSTRAINT uq_chunk_embeddings_model UNIQUE (chunk_id, embedding_model)
);

-- Supports cosine nearest-neighbor retrieval over chunk embeddings.
CREATE INDEX IF NOT EXISTS idx_chunk_embeddings_vector
    ON chunk_embeddings USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);

CREATE TABLE IF NOT EXISTS index_projection_status (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    content_id BIGINT NOT NULL REFERENCES content_items(id) ON DELETE CASCADE,
    index_version INT NOT NULL CHECK (index_version > 0),
    chunking_version INT NOT NULL CHECK (chunking_version > 0),
    embedding_model VARCHAR(100) NOT NULL,
    state VARCHAR(20) NOT NULL CHECK (state IN ('staging', 'ready', 'failed')),
    error_summary TEXT NOT NULL DEFAULT '',
    last_indexed_at TIMESTAMPTZ,
    is_current BOOLEAN NOT NULL DEFAULT FALSE,
    -- Coordinates one lifecycle row for each content index generation.
    CONSTRAINT uq_index_projection_generation UNIQUE (content_id, index_version)
);

-- Enforces one query-visible generation per content during atomic promotion.
CREATE UNIQUE INDEX IF NOT EXISTS uq_index_projection_current
    ON index_projection_status(content_id) WHERE is_current = TRUE;
-- Supports current ready-generation lookup by chunker and embedding contract.
CREATE INDEX IF NOT EXISTS idx_index_projection_ready
    ON index_projection_status(index_version, chunking_version, embedding_model)
    WHERE is_current = TRUE AND state = 'ready';
