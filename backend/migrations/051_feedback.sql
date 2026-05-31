-- Feedback tickets, replies and attachments (V-05)

CREATE TABLE feedback_tickets (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT REFERENCES users(id),
  contact_email VARCHAR(255),
  category VARCHAR(32) NOT NULL,
  title VARCHAR(160) NOT NULL,
  description TEXT NOT NULL,
  diagnostic_summary JSONB NOT NULL DEFAULT '{}',
  status VARCHAR(24) NOT NULL DEFAULT 'open',
  priority VARCHAR(24) NOT NULL DEFAULT 'normal',
  assignee_admin_id BIGINT REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  resolved_at TIMESTAMPTZ,
  CHECK (category IN ('web_bug', 'desktop_deploy', 'content_or_community', 'account_or_security', 'agent_quality', 'feature_request', 'other')),
  CHECK (status IN ('open', 'in_progress', 'resolved', 'closed')),
  CHECK (priority IN ('low', 'normal', 'high', 'urgent'))
);

CREATE TABLE feedback_replies (
  id BIGSERIAL PRIMARY KEY,
  ticket_id BIGINT NOT NULL REFERENCES feedback_tickets(id) ON DELETE CASCADE,
  author_user_id BIGINT REFERENCES users(id),
  author_admin_id BIGINT REFERENCES users(id),
  body TEXT NOT NULL,
  is_internal_note BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CHECK (author_user_id IS NOT NULL OR author_admin_id IS NOT NULL),
  CHECK (NOT (author_user_id IS NOT NULL AND author_admin_id IS NOT NULL))
);

CREATE TABLE feedback_attachments (
  id BIGSERIAL PRIMARY KEY,
  ticket_id BIGINT NOT NULL REFERENCES feedback_tickets(id) ON DELETE CASCADE,
  oss_key TEXT NOT NULL,
  file_type VARCHAR(32) NOT NULL,
  mime_type VARCHAR(100) NOT NULL,
  size_bytes BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_feedback_tickets_user_id ON feedback_tickets(user_id);
CREATE INDEX idx_feedback_tickets_status ON feedback_tickets(status);
CREATE INDEX idx_feedback_replies_ticket_id ON feedback_replies(ticket_id);
CREATE INDEX idx_feedback_attachments_ticket_id ON feedback_attachments(ticket_id);
