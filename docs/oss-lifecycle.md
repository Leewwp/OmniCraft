# OSS Lifecycle Strategy

This document defines the object lifecycle policy used by OmniCraft for uploaded content and temporary collaboration artifacts.

## Bucket Baseline

- Bucket ACL: private
- Access model: signed URL only
- Upload path prefix: `uploads/{user_id}/{file_type}/{yyyy}/{mm}/{dd}/...`
- Download URL validity: 1 hour
- Upload URL validity: 15 minutes

The bucket must remain private. Public read ACLs are not part of the application authorization model.

## Upload Grant Rules

- The backend issues presigned PUT URLs only together with a short-lived upload grant.
- A publish request must reference the grant ID, not only a raw OSS key.
- Grants are bound to user ID, purpose, file type, MIME type, declared file size, and OSS key.
- Grants are consumed once.
- Feedback screenshot grants and content publish grants are not interchangeable.

## Lifecycle Rules

1. Temporary PR artifacts
- Scope: objects generated for pending pull-request snapshots and temporary merge previews.
- Prefix suggestion: `uploads/pr-temp/`
- Retention: 30 days
- Action: delete object automatically when expired.

2. Deleted content assets
- Scope: assets whose related content has been deleted or permanently banned.
- Prefix suggestion: `uploads/deleted/`
- Retention: 90 days
- Action: delete object automatically when expired.

3. Permanent published assets
- Scope: files linked to active and published content.
- Prefix suggestion: `uploads/{user_id}/video/`, `uploads/{user_id}/image/`, `uploads/{user_id}/text/`, `uploads/{user_id}/mod/`, `uploads/{user_id}/sheet_music/`
- Retention: no auto-delete (manual cleanup only).

## Operational Notes

- Object keys for soft-deleted or banned content may be moved by a cleanup job to `uploads/deleted/` to activate the 90-day cleanup policy.
- PR temporary objects should always use `uploads/pr-temp/` so they are cleaned by lifecycle policy without manual jobs.
- Lifecycle policy must be configured in OSS console or IaC, not hardcoded in backend logic.
- Keep bucket private to avoid unauthorized hotlinking and enforce access expiration.
