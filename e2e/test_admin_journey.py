"""
Manual cross-stack admin journey validation.

This script is manual-only release evidence. It must run against explicit,
known fixtures and fails when setup assumptions are missing or drifted.
"""

from __future__ import annotations

import json
import os
import re
import sys
import time
from pathlib import Path
from typing import Any

import requests
from playwright.sync_api import sync_playwright


BASE_API = os.getenv("OMNICRAFT_API_URL", "http://localhost:8080").rstrip("/")
BASE_WEB = os.getenv("OMNICRAFT_FRONTEND_URL", "http://localhost:3000").rstrip("/")
SCREENSHOT_DIR = Path(
    os.getenv(
        "OMNICRAFT_SCREENSHOT_DIR",
        r"c:\Users\16278\Desktop\file\code\project\OmniCraft\screenshots\review-web-beta\09-cross-stack-e2e",
    )
)

ADMIN_EMAIL = os.getenv("OMNICRAFT_ADMIN_EMAIL", "").strip()
ADMIN_PASSWORD = os.getenv("OMNICRAFT_ADMIN_PASSWORD", "").strip()
NORMAL_EMAIL = os.getenv("OMNICRAFT_NORMAL_USER_EMAIL", "").strip()
NORMAL_PASSWORD = os.getenv("OMNICRAFT_NORMAL_USER_PASSWORD", "").strip()
REPORT_CONTENT_ID = os.getenv("OMNICRAFT_REPORT_CONTENT_ID", "").strip()

ANONYMOUS_FEEDBACK_EMAIL = os.getenv("OMNICRAFT_FEEDBACK_CONTACT_EMAIL", "qa-feedback@example.com").strip()
REPORT_REASON = os.getenv("OMNICRAFT_REPORT_REASON", "spam").strip()
EXPECTED_REPORT_ACTION = os.getenv("OMNICRAFT_REPORT_ACTION", "resolved").strip()
EXPECTED_FEEDBACK_STATUS = os.getenv("OMNICRAFT_FEEDBACK_CLOSE_STATUS", "closed").strip()

results: dict[str, dict[str, str]] = {}


def record(step: str, passed: bool, detail: str = "") -> None:
    status = "PASS" if passed else "FAIL"
    results[step] = {"status": status, "detail": detail}
    print(f"  [{status}] {step}: {detail}")


def fail(message: str) -> None:
    raise RuntimeError(message)


def require_value(name: str, value: str) -> str:
    if value:
        return value
    fail(f"missing required environment variable: {name}")
    return ""


def require_int(name: str, value: str) -> int:
    raw = require_value(name, value)
    try:
        return int(raw)
    except ValueError as exc:
        raise RuntimeError(f"{name} must be an integer, got {raw!r}") from exc


def ensure_dir(path: Path) -> None:
    path.mkdir(parents=True, exist_ok=True)


class APIClient:
    def __init__(self, base_url: str):
        self.base_url = base_url
        self.session = requests.Session()
        self.csrf_token = ""

    def fetch_csrf(self) -> str:
        resp = self.session.get(f"{self.base_url}/api/v1/auth/csrf", timeout=15)
        if resp.status_code != 200:
            fail(f"GET /api/v1/auth/csrf returned {resp.status_code}: {resp.text[:200]}")
        data = resp.json()
        token = data.get("csrf_token")
        if not token:
            fail("/api/v1/auth/csrf response is missing csrf_token")
        self.csrf_token = token
        return token

    def request(
        self,
        method: str,
        path: str,
        *,
        token: str | None = None,
        json_data: dict[str, Any] | None = None,
        params: dict[str, Any] | None = None,
    ) -> requests.Response:
        headers: dict[str, str] = {}
        if token:
            headers["Authorization"] = f"Bearer {token}"
        if method.upper() in {"POST", "PATCH", "PUT", "DELETE"}:
            headers["X-CSRF-Token"] = self.csrf_token or self.fetch_csrf()
        return self.session.request(
            method,
            f"{self.base_url}{path}",
            headers=headers,
            json=json_data,
            params=params,
            timeout=15,
        )

    def login(self, email: str, password: str) -> tuple[str, dict[str, Any]]:
        self.fetch_csrf()
        resp = self.request("POST", "/api/v1/auth/login", json_data={"email": email, "password": password})
        if resp.status_code != 200:
            fail(f"login failed for {email}: {resp.status_code} {resp.text[:200]}")
        data = resp.json()
        token = (data.get("tokens") or {}).get("access_token")
        if not token:
            fail(f"login response is missing tokens.access_token for {email}")
        me_resp = self.request("GET", "/api/v1/auth/me", token=token)
        if me_resp.status_code != 200:
            fail(f"/api/v1/auth/me failed for {email}: {me_resp.status_code} {me_resp.text[:200]}")
        me_data = me_resp.json()
        user = me_data.get("user")
        if not isinstance(user, dict):
            fail(f"/api/v1/auth/me returned unexpected payload for {email}: {me_data!r}")
        return token, user


def expect_schema(data: dict[str, Any], *, step: str) -> list[dict[str, Any]]:
    required_keys = ("items", "total", "page", "page_size")
    missing = [key for key in required_keys if key not in data]
    if missing:
        fail(f"{step} response is missing keys {missing}: {data!r}")
    items = data["items"]
    if not isinstance(items, list):
        fail(f"{step} items must be a list, got {type(items).__name__}")
    for key in ("total", "page", "page_size"):
        if not isinstance(data[key], int):
            fail(f"{step} {key} must be an integer, got {data[key]!r}")
    return items


def browser_login(page, email: str, password: str) -> None:
    page.goto(f"{BASE_WEB}/login", wait_until="networkidle", timeout=20000)
    page.locator("input#email").fill(email)
    page.locator("input#password").fill(password)
    page.locator('button[type="submit"]').click()
    page.wait_for_load_state("networkidle", timeout=20000)
    if "/login" in page.url:
        alert = ""
        error = page.locator('[role="alert"], .text-destructive').first
        if error.count() > 0:
            alert = error.inner_text()
        fail(f"browser login failed for {email}: url={page.url} alert={alert[:120]}")


def create_report_fixture(
    admin_client: APIClient,
    admin_token: str,
    normal_client: APIClient,
    normal_token: str,
    normal_user: dict[str, Any],
    content_id: int,
) -> dict[str, Any]:
    detail = f"manual admin journey fixture {int(time.time())}"
    resp = normal_client.request(
        "POST",
        f"/api/v1/contents/{content_id}/report",
        token=normal_token,
        json_data={"reason": REPORT_REASON, "detail": detail},
    )
    if resp.status_code not in (201, 409):
        fail(f"report fixture creation failed: {resp.status_code} {resp.text[:200]}")

    reports_resp = admin_client.request(
        "GET",
        "/api/v1/admin/reports",
        token=admin_token,
        params={"status": "pending", "page": 1, "page_size": 100},
    )
    if reports_resp.status_code != 200:
        fail(f"listing pending reports failed: {reports_resp.status_code} {reports_resp.text[:200]}")
    payload = reports_resp.json()
    reports = payload.get("reports")
    if not isinstance(reports, list):
        fail(f"/api/v1/admin/reports returned unexpected payload: {payload!r}")

    normal_user_id = normal_user.get("id")
    for report in reports:
        if not isinstance(report, dict):
            continue
        if report.get("target_type") != "content":
            continue
        if report.get("target_id") != content_id:
            continue
        if report.get("reporter_id") != normal_user_id:
            continue
        if report.get("status") != "pending":
            continue
        return report

    fail("report fixture was not present in pending admin reports after creation attempt")
    return {}


def create_feedback_fixture(public_client: APIClient) -> dict[str, Any]:
    unique = int(time.time())
    public_client.fetch_csrf()
    resp = public_client.request(
        "POST",
        "/api/v1/feedback",
        json_data={
            "contact_email": ANONYMOUS_FEEDBACK_EMAIL,
            "category": "other",
            "title": f"manual admin journey feedback {unique}",
            "description": f"manual feedback close fixture {unique}",
            "diagnostic_summary": {"route": "/admin/feedback"},
            "captcha_token": "bypass",
            "attachment_grants": [],
        },
    )
    if resp.status_code != 201:
        fail(f"feedback fixture creation failed: {resp.status_code} {resp.text[:200]}")
    ticket = resp.json()
    if not isinstance(ticket, dict) or not ticket.get("id"):
        fail(f"feedback fixture response is missing ticket id: {ticket!r}")
    return ticket


def assert_audit_entry(
    admin_client: APIClient,
    admin_token: str,
    *,
    action: str,
    target_id: int,
    metadata_expectations: dict[str, Any],
    step: str,
) -> None:
    resp = admin_client.request(
        "GET",
        "/api/v1/admin/audit-logs",
        token=admin_token,
        params={"action": action, "page": 1, "page_size": 100},
    )
    if resp.status_code != 200:
        fail(f"{step} request failed: {resp.status_code} {resp.text[:200]}")
    data = resp.json()
    items = expect_schema(data, step=step)
    match = None
    for item in items:
        if not isinstance(item, dict):
            continue
        if item.get("action") != action:
            continue
        if str(item.get("target_id")) != str(target_id):
            continue
        if item.get("result") != "success":
            continue
        metadata = item.get("metadata")
        if not isinstance(metadata, dict):
            continue
        if all(metadata.get(key) == value for key, value in metadata_expectations.items()):
            match = item
            break
    if match is None:
        fail(f"{step} did not find expected audit entry for action={action}, target_id={target_id}")
    required_item_keys = ("id", "admin_user_id", "action", "target_type", "target_id", "metadata", "result", "created_at")
    missing = [key for key in required_item_keys if key not in match]
    if missing:
        fail(f"{step} matched entry is missing keys {missing}: {match!r}")


def main() -> int:
    ensure_dir(SCREENSHOT_DIR)
    try:
        admin_email = require_value("OMNICRAFT_ADMIN_EMAIL", ADMIN_EMAIL)
        admin_password = require_value("OMNICRAFT_ADMIN_PASSWORD", ADMIN_PASSWORD)
        normal_email = require_value("OMNICRAFT_NORMAL_USER_EMAIL", NORMAL_EMAIL)
        normal_password = require_value("OMNICRAFT_NORMAL_USER_PASSWORD", NORMAL_PASSWORD)
        report_content_id = require_int("OMNICRAFT_REPORT_CONTENT_ID", REPORT_CONTENT_ID)

        admin_client = APIClient(BASE_API)
        admin_token, admin_user = admin_client.login(admin_email, admin_password)
        if admin_user.get("role") != "admin":
            fail(f"admin fixture user {admin_email} has role={admin_user.get('role')!r}, expected 'admin'")
        record("setup-admin-role", True, f"{admin_email} confirmed as admin")

        normal_client = APIClient(BASE_API)
        normal_token, normal_user = normal_client.login(normal_email, normal_password)
        if normal_user.get("role") == "admin":
            fail(f"normal fixture user {normal_email} unexpectedly has admin role")
        record("setup-normal-role", True, f"{normal_email} confirmed as non-admin")

        report_fixture = create_report_fixture(
            admin_client, admin_token, normal_client, normal_token, normal_user, report_content_id
        )
        record("setup-report-fixture", True, f"report_id={report_fixture['id']}")

        public_client = APIClient(BASE_API)
        feedback_fixture = create_feedback_fixture(public_client)
        record("setup-feedback-fixture", True, f"ticket_id={feedback_fixture['id']}")

        with sync_playwright() as playwright:
            browser = playwright.chromium.launch(headless=True)
            try:
                normal_page = browser.new_page(viewport={"width": 1280, "height": 900})
                browser_login(normal_page, normal_email, normal_password)
                normal_page.goto(f"{BASE_WEB}/admin", wait_until="networkidle", timeout=20000)
                normal_page.screenshot(path=str(SCREENSHOT_DIR / "22-normal-user-admin-denied.png"), full_page=True)
                page_content = normal_page.content().lower()
                denied = "access denied" in page_content or "accessdenied" in page_content
                redirected = "/admin" not in normal_page.url
                if not (denied or redirected):
                    fail(f"normal user reached admin area: url={normal_page.url}")
                record("A1", True, f"normal user blocked from /admin; redirected={redirected}")

                admin_page = browser.new_page(viewport={"width": 1280, "height": 900})
                browser_login(admin_page, admin_email, admin_password)
                admin_pages = [
                    ("/admin/dashboard", "23-admin-dashboard.png"),
                    ("/admin/reports", "24-admin-reports.png"),
                    ("/admin/feedback", "25-admin-feedback.png"),
                    ("/admin/queue", "26-admin-queue.png"),
                    ("/admin/audit-logs", "27-admin-audit-logs.png"),
                    ("/admin/config", "28-admin-config.png"),
                ]
                for path, filename in admin_pages:
                    admin_page.goto(f"{BASE_WEB}{path}", wait_until="networkidle", timeout=20000)
                    admin_page.screenshot(path=str(SCREENSHOT_DIR / filename), full_page=True)
                    page_content = admin_page.content().lower()
                    if "/admin" not in admin_page.url:
                        fail(f"admin browser flow left admin area while visiting {path}: {admin_page.url}")
                    if "access denied" in page_content or "accessdenied" in page_content:
                        fail(f"admin browser flow rendered access denied while visiting {path}")
                    record(f"B-{path.split('/')[-1]}", True, f"admin loaded {path}")

                admin_page.goto(f"{BASE_WEB}/admin/queue", wait_until="networkidle", timeout=20000)
                button_texts = []
                for index in range(admin_page.locator("button").count()):
                    text = admin_page.locator("button").nth(index).inner_text().strip().lower()
                    if text:
                        button_texts.append(text)
                forbidden_controls = [text for text in button_texts if text in {"replay", "retry", "delete"}]
                if forbidden_controls:
                    fail(f"queue page exposed forbidden controls: {forbidden_controls}")
                record("D1", True, "queue page exposes no replay/retry/delete controls")
            finally:
                browser.close()

        report_id = int(report_fixture["id"])
        report_action_taken = f"manual admin journey resolve {int(time.time())}"
        resolve_resp = admin_client.request(
            "PATCH",
            f"/api/v1/admin/reports/{report_id}",
            token=admin_token,
            json_data={"status": EXPECTED_REPORT_ACTION, "action_taken": report_action_taken},
        )
        if resolve_resp.status_code != 200:
            fail(f"report resolve API failed: {resolve_resp.status_code} {resolve_resp.text[:200]}")
        record("C1", True, f"resolved report {report_id} via API")

        assert_audit_entry(
            admin_client,
            admin_token,
            action="report_resolve",
            target_id=report_id,
            metadata_expectations={"report_id": report_id, "decision": EXPECTED_REPORT_ACTION, "reason": report_action_taken},
            step="C2",
        )
        record("C2", True, f"audit captured report_resolve for report_id={report_id}")

        ticket_id = int(feedback_fixture["id"])
        close_resp = admin_client.request(
            "PATCH",
            f"/api/v1/admin/feedback/{ticket_id}",
            token=admin_token,
            json_data={"status": EXPECTED_FEEDBACK_STATUS},
        )
        if close_resp.status_code != 200:
            fail(f"feedback close API failed: {close_resp.status_code} {close_resp.text[:200]}")
        ticket = close_resp.json()
        if ticket.get("id") != ticket_id or ticket.get("status") != EXPECTED_FEEDBACK_STATUS:
            fail(f"feedback close returned unexpected payload: {ticket!r}")
        record("C3", True, f"closed feedback ticket {ticket_id} via API")

        assert_audit_entry(
            admin_client,
            admin_token,
            action="feedback_close",
            target_id=ticket_id,
            metadata_expectations={"ticket_id": ticket_id, "status": EXPECTED_FEEDBACK_STATUS},
            step="C4",
        )
        record("C4", True, f"audit captured feedback_close for ticket_id={ticket_id}")

        config_resp = admin_client.request("GET", "/api/v1/admin/config", token=admin_token)
        if config_resp.status_code != 200:
            fail(f"GET /api/v1/admin/config failed: {config_resp.status_code} {config_resp.text[:200]}")
        config_payload = config_resp.json()
        secrets_status = config_payload.get("secrets_status")
        if not isinstance(secrets_status, dict) or not all(isinstance(value, bool) for value in secrets_status.values()):
            fail(f"admin config secrets_status must be a boolean map: {config_payload!r}")
        config_str = json.dumps(config_payload.get("config", {}))
        leaked = []
        for pattern in (
            r'"secret"\s*:\s*"(?!\*\*\*REDACTED\*\*\*)[^"]+"',
            r'"access_key_secret"\s*:\s*"(?!\*\*\*REDACTED\*\*\*)[^"]+"',
            r'"api_key"\s*:\s*"(?!\*\*\*REDACTED\*\*\*)[^"]+"',
            r'"hmac_secret"\s*:\s*"(?!\*\*\*REDACTED\*\*\*)[^"]+"',
        ):
            leaked.extend(re.findall(pattern, config_str, flags=re.IGNORECASE))
        if leaked:
            fail(f"admin config leaked plaintext secrets: {leaked}")
        record("E1", True, "config API exposes only redaction status for secrets")
    except Exception as exc:
        record("setup-or-run", False, str(exc))

    print("\n" + "=" * 60)
    print("TEST SUMMARY")
    print("=" * 60)
    all_passed = True
    for test_id, result in sorted(results.items()):
        status = result["status"]
        print(f"  [{status}] {test_id}: {result['detail']}")
        if status != "PASS":
            all_passed = False
    print("=" * 60)
    if all_passed:
        print("ALL TESTS PASSED")
    else:
        failed = [test_id for test_id, result in results.items() if result["status"] != "PASS"]
        print(f"SOME TESTS FAILED: {', '.join(failed)}")
    print("=" * 60)
    return 0 if all_passed else 1


if __name__ == "__main__":
    sys.exit(main())
