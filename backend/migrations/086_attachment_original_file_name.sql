-- Migration 086: content_attachments.original_file_name (#688 v2.2).
-- The presign request already carries file_name but it was dropped; the
-- publish payload's file_name is untrusted. The server normalizes and locks
-- the name into the grant at presign time and persists it here so multi-
-- attachment detail views can distinguish same-type files and downloads get
-- a proper Content-Disposition. Legacy rows keep NULL and the frontend falls
-- back to the type label (today's rendering).
ALTER TABLE content_attachments
    ADD COLUMN IF NOT EXISTS original_file_name VARCHAR(255) NULL;
