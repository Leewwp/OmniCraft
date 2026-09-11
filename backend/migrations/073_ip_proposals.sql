-- 073: IP 共治提案域（#290 贴吧式社区枢纽）
-- 提案/投票/资料版本快照三表；与判官域完全独立（不共表不共享队列/配置）。
-- 资格：提案权=登录+信誉分≥3；投票权=关注该 IP+信誉分≥3（服务层校验）。
-- 并发：同 IP 仅允许一个 open 提案（部分唯一索引兜底 + 服务层事务校验）。
-- 软删除豁免：提案/投票/版本快照为 append-only 治理审计域，不做 deleted_at 软删除。

CREATE TABLE IF NOT EXISTS ip_proposals (
    id BIGSERIAL PRIMARY KEY,
    ip_id BIGINT NOT NULL REFERENCES ips(id) ON DELETE CASCADE,
    proposer_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status VARCHAR(16) NOT NULL DEFAULT 'open',
    description_change TEXT,
    cover_url_change TEXT,
    tags_add JSONB NOT NULL DEFAULT '[]',
    tags_remove JSONB NOT NULL DEFAULT '[]',
    moderation_state VARCHAR(16) NOT NULL DEFAULT 'approved',
    yes_votes INT NOT NULL DEFAULT 0,
    no_votes INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deadline_at TIMESTAMPTZ NOT NULL,
    closed_at TIMESTAMPTZ,
    effective_at TIMESTAMPTZ,
    CONSTRAINT ck_ip_proposals_status CHECK (status IN ('open', 'adopted', 'rejected'))
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_ip_proposals_open_per_ip
    ON ip_proposals(ip_id) WHERE status = 'open';

CREATE INDEX IF NOT EXISTS idx_ip_proposals_ip_status
    ON ip_proposals(ip_id, status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_ip_proposals_deadline
    ON ip_proposals(deadline_at) WHERE status = 'open';

CREATE TABLE IF NOT EXISTS ip_proposal_votes (
    id BIGSERIAL PRIMARY KEY,
    proposal_id BIGINT NOT NULL REFERENCES ip_proposals(id) ON DELETE CASCADE,
    voter_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    vote VARCHAR(8) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ck_ip_proposal_votes_vote CHECK (vote IN ('yes', 'no')),
    CONSTRAINT uq_ip_proposal_votes_one_per_voter UNIQUE (proposal_id, voter_id)
);

CREATE INDEX IF NOT EXISTS idx_ip_proposal_votes_proposal
    ON ip_proposal_votes(proposal_id, vote);

CREATE TABLE IF NOT EXISTS ip_profile_versions (
    id BIGSERIAL PRIMARY KEY,
    ip_id BIGINT NOT NULL REFERENCES ips(id) ON DELETE CASCADE,
    proposal_id BIGINT NOT NULL REFERENCES ip_proposals(id) ON DELETE CASCADE,
    snapshot JSONB NOT NULL,
    changes JSONB NOT NULL,
    yes_votes INT NOT NULL DEFAULT 0,
    no_votes INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ip_profile_versions_ip
    ON ip_profile_versions(ip_id, created_at DESC);
