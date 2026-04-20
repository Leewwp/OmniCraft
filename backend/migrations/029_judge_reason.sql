ALTER TABLE judge_votes ADD COLUMN IF NOT EXISTS reason TEXT;

CREATE TABLE IF NOT EXISTS judge_reason_votes (
    id                  BIGSERIAL   PRIMARY KEY,
    reason_owner_vote_id BIGINT     NOT NULL REFERENCES judge_votes(id) ON DELETE CASCADE,
    voter_id            BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    vote_type           VARCHAR(10) NOT NULL CHECK (vote_type IN ('up','down')),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (reason_owner_vote_id, voter_id)
);

CREATE INDEX IF NOT EXISTS idx_judge_reason_votes_vote ON judge_reason_votes (reason_owner_vote_id);
