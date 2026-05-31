-- Admin audit logs (A-01)

CREATE TABLE admin_audit_logs (
  id BIGSERIAL PRIMARY KEY,
  admin_user_id BIGINT NOT NULL REFERENCES users(id),
  action VARCHAR(96) NOT NULL,
  target_type VARCHAR(48) NOT NULL,
  target_id VARCHAR(96),
  trace_id VARCHAR(96),
  metadata JSONB NOT NULL DEFAULT '{}',
  result VARCHAR(24) NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_admin_audit_logs_created_at ON admin_audit_logs(created_at DESC);
CREATE INDEX idx_admin_audit_logs_action ON admin_audit_logs(action);
