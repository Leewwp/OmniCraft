# Root Python E2E

These two scripts are **manual release checks**, not CI gate tests:

- `e2e/test_search_download.py`
- `e2e/test_admin_journey.py`

They intentionally fail when fixture accounts, fixture content, or explicit auth setup are missing. They are only valid when run against a known local or staging environment with seeded users/content.

## Entry point

Run both suites:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/run-root-python-e2e.ps1
```

Run one suite:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/run-root-python-e2e.ps1 -Suite search-download
powershell -ExecutionPolicy Bypass -File scripts/run-root-python-e2e.ps1 -Suite admin-journey
```

From `frontend/`, the same manual entry point is exposed as:

```powershell
npm.cmd run test:e2e:release-manual
```

## Required fixture style

- `search/download` expects:
  - a known published content item with a downloadable attachment
  - a known verified user that can download it
  - a known unverified user fixture and a low-reputation user fixture
- `admin journey` expects:
  - a verified admin account
  - a verified non-admin user account
  - a known reportable published content id

The scripts no longer promote roles through SQL or treat missing data as success. If the fixture contract is not present, the run must fail.

## Common environment variables

Shared:

- `OMNICRAFT_API_URL`
- `OMNICRAFT_FRONTEND_URL`
- `OMNICRAFT_SCREENSHOT_DIR`

Search/download:

- `OMNICRAFT_SEARCH_QUERY`
- `OMNICRAFT_EXPECTED_RESULT_TITLE`
- `OMNICRAFT_FORBIDDEN_SEARCH_TITLES`
- `OMNICRAFT_DOWNLOAD_CONTENT_ID`
- `OMNICRAFT_DOWNLOAD_ATTACHMENT_ID`
- `OMNICRAFT_VERIFIED_USER_EMAIL`
- `OMNICRAFT_VERIFIED_USER_PASSWORD`

Optional search/download auth fixture helpers:

- `OMNICRAFT_VERIFIED_BEARER_TOKEN`
- `OMNICRAFT_UNVERIFIED_BEARER_TOKEN`
- `OMNICRAFT_LOW_REPUTATION_BEARER_TOKEN`
- `OMNICRAFT_UNVERIFIED_USER_ID`
- `OMNICRAFT_LOW_REPUTATION_USER_ID`
- `OMNICRAFT_JWT_SECRET`

Admin journey:

- `OMNICRAFT_ADMIN_EMAIL`
- `OMNICRAFT_ADMIN_PASSWORD`
- `OMNICRAFT_NORMAL_USER_EMAIL`
- `OMNICRAFT_NORMAL_USER_PASSWORD`
- `OMNICRAFT_REPORT_CONTENT_ID`

## Notes

- Screenshots are evidence only; pass/fail is driven by exact request/response and rendered-behavior assertions.
- These scripts should not be counted as automated CI gates unless the environment variables and deterministic fixtures are provisioned by the runner.
