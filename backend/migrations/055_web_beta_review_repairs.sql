-- Web Beta review repairs: feedback/report schema alignment.

ALTER TABLE reports
  ADD COLUMN IF NOT EXISTS action_taken TEXT;

ALTER TABLE feedback_tickets
  DROP CONSTRAINT IF EXISTS feedback_tickets_status_check;

ALTER TABLE feedback_tickets
  ADD CONSTRAINT feedback_tickets_status_check
  CHECK (status IN ('open', 'in_progress', 'resolved', 'closed', 'reopened'));
