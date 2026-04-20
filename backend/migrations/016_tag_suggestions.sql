CREATE TABLE IF NOT EXISTS tag_suggestions (
    id              BIGSERIAL PRIMARY KEY,
    content_item_id BIGINT      NOT NULL REFERENCES content_items(id) ON DELETE CASCADE,
    user_id         BIGINT      NOT NULL REFERENCES users(id),
    tag             VARCHAR(100) NOT NULL,
    action          VARCHAR(10)  NOT NULL CHECK (action IN ('add','remove')),
    status          VARCHAR(20)  NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','approved','rejected','reported')),
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE (content_item_id, user_id, tag, action)
);

CREATE INDEX IF NOT EXISTS idx_tag_suggestions_content ON tag_suggestions (content_item_id, status);
CREATE INDEX IF NOT EXISTS idx_tag_suggestions_user ON tag_suggestions (user_id);
