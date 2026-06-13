"""
Manual cross-stack search and download validation.

This script is intentionally strict:
- fixture/env gaps are failures
- every protected-path assertion checks the exact contract
- screenshots are evidence only, never the pass condition
"""

from __future__ import annotations

import base64
import hashlib
import hmac
import json
import os
import sys
import time
from pathlib import Path
from typing import Any
from urllib.parse import parse_qs, urlparse

import requests
from playwright.sync_api import TimeoutError as PlaywrightTimeoutError
from playwright.sync_api import sync_playwright


BASE_URL_API = os.getenv("OMNICRAFT_API_URL", "http://localhost:8080").rstrip("/")
BASE_URL_FRONT = os.getenv("OMNICRAFT_FRONTEND_URL", "http://localhost:3000").rstrip("/")
SCREENSHOT_DIR = Path(
    os.getenv(
        "OMNICRAFT_SCREENSHOT_DIR",
        r"c:\Users\16278\Desktop\file\code\project\OmniCraft\screenshots\review-web-beta\09-cross-stack-e2e",
    )
)

SEARCH_QUERY = os.getenv("OMNICRAFT_SEARCH_QUERY", "").strip()
EXPECTED_RESULT_TITLE = os.getenv("OMNICRAFT_EXPECTED_RESULT_TITLE", "").strip()
FORBIDDEN_SEARCH_TITLES = [
    item.strip()
    for item in os.getenv("OMNICRAFT_FORBIDDEN_SEARCH_TITLES", "").split(",")
    if item.strip()
]

DOWNLOAD_CONTENT_ID = os.getenv("OMNICRAFT_DOWNLOAD_CONTENT_ID", "").strip()
DOWNLOAD_ATTACHMENT_ID = os.getenv("OMNICRAFT_DOWNLOAD_ATTACHMENT_ID", "").strip()
MISSING_CONTENT_ID = int(os.getenv("OMNICRAFT_MISSING_CONTENT_ID", "999999999"))

VERIFIED_EMAIL = os.getenv("OMNICRAFT_VERIFIED_USER_EMAIL", "").strip()
VERIFIED_PASSWORD = os.getenv("OMNICRAFT_VERIFIED_USER_PASSWORD", "").strip()
VERIFIED_BEARER_TOKEN = os.getenv("OMNICRAFT_VERIFIED_BEARER_TOKEN", "").strip()

UNVERIFIED_BEARER_TOKEN = os.getenv("OMNICRAFT_UNVERIFIED_BEARER_TOKEN", "").strip()
LOW_REPUTATION_BEARER_TOKEN = os.getenv("OMNICRAFT_LOW_REPUTATION_BEARER_TOKEN", "").strip()
JWT_SECRET = os.getenv("OMNICRAFT_JWT_SECRET", "").strip()

UNVERIFIED_USER_ID = os.getenv("OMNICRAFT_UNVERIFIED_USER_ID", "").strip()
LOW_REPUTATION_USER_ID = os.getenv("OMNICRAFT_LOW_REPUTATION_USER_ID", "").strip()

EXPECTED_NOAUTH_STATUS = int(os.getenv("OMNICRAFT_EXPECTED_NOAUTH_STATUS", "401"))
EXPECTED_NOAUTH_CODE = os.getenv("OMNICRAFT_EXPECTED_NOAUTH_CODE", "UNAUTHORIZED").strip()
EXPECTED_UNVERIFIED_STATUS = int(os.getenv("OMNICRAFT_EXPECTED_UNVERIFIED_STATUS", "403"))
EXPECTED_UNVERIFIED_CODE = os.getenv("OMNICRAFT_EXPECTED_UNVERIFIED_CODE", "EMAIL_NOT_VERIFIED").strip()
EXPECTED_LOW_REPUTATION_STATUS = int(os.getenv("OMNICRAFT_EXPECTED_LOW_REPUTATION_STATUS", "403"))
EXPECTED_LOW_REPUTATION_CODE = os.getenv(
    "OMNICRAFT_EXPECTED_LOW_REPUTATION_CODE", "INSUFFICIENT_REPUTATION"
).strip()
EXPECTED_MISSING_STATUS = int(os.getenv("OMNICRAFT_EXPECTED_MISSING_STATUS", "404"))
EXPECTED_MISSING_CODE = os.getenv("OMNICRAFT_EXPECTED_MISSING_CODE", "NOT_FOUND").strip()

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


def base64url_encode(data: bytes) -> str:
    return base64.urlsafe_b64encode(data).rstrip(b"=").decode("ascii")


def mint_access_token(user_id: int, role: str) -> str:
    if not JWT_SECRET:
        fail("missing OMNICRAFT_JWT_SECRET for local token minting")
    header = {"alg": "HS256", "typ": "JWT"}
    now = int(time.time())
    payload = {"user_id": user_id, "role": role, "sub": "access", "iat": now, "exp": now + 3600}
    signing_input = ".".join(
        [
            base64url_encode(json.dumps(header, separators=(",", ":")).encode("utf-8")),
            base64url_encode(json.dumps(payload, separators=(",", ":")).encode("utf-8")),
        ]
    )
    signature = hmac.new(JWT_SECRET.encode("utf-8"), signing_input.encode("ascii"), hashlib.sha256).digest()
    return signing_input + "." + base64url_encode(signature)


def get_fixture_token(token_env: str, user_id_env: str, label: str, role: str = "user") -> str:
    direct = os.getenv(token_env, "").strip()
    if direct:
        return direct
    user_id_raw = os.getenv(user_id_env, "").strip()
    if user_id_raw:
        return mint_access_token(require_int(user_id_env, user_id_raw), role)
    fail(f"{label} fixture requires {token_env} or {user_id_env} + OMNICRAFT_JWT_SECRET")
    return ""


def ensure_dir(path: Path) -> None:
    path.mkdir(parents=True, exist_ok=True)


def fetch_csrf(session: requests.Session) -> str:
    resp = session.get(f"{BASE_URL_API}/api/v1/auth/csrf", timeout=15)
    if resp.status_code != 200:
        fail(f"GET /api/v1/auth/csrf returned {resp.status_code}: {resp.text[:200]}")
    data = resp.json()
    token = data.get("csrf_token")
    if not token:
        fail("/api/v1/auth/csrf response is missing csrf_token")
    return token


def login_verified_user() -> tuple[requests.Session, str]:
    if VERIFIED_BEARER_TOKEN:
        session = requests.Session()
        csrf_token = fetch_csrf(session)
        session.headers.update({"X-CSRF-Token": csrf_token})
        record("1-Verified-Login", True, "using explicit bearer token fixture")
        return session, VERIFIED_BEARER_TOKEN

    email = require_value("OMNICRAFT_VERIFIED_USER_EMAIL", VERIFIED_EMAIL)
    password = require_value("OMNICRAFT_VERIFIED_USER_PASSWORD", VERIFIED_PASSWORD)
    session = requests.Session()
    csrf_token = fetch_csrf(session)
    resp = session.post(
        f"{BASE_URL_API}/api/v1/auth/login",
        json={"email": email, "password": password},
        headers={"X-CSRF-Token": csrf_token},
        timeout=15,
    )
    if resp.status_code != 200:
        fail(f"verified user login failed: {resp.status_code} {resp.text[:200]}")
    data = resp.json()
    token = (data.get("tokens") or {}).get("access_token")
    if not token:
        fail("verified user login response is missing tokens.access_token")
    record("1-Verified-Login", True, f"logged in as {email}")
    return session, token


def request_json(
    session: requests.Session,
    method: str,
    path: str,
    *,
    token: str | None = None,
    params: dict[str, Any] | None = None,
    json_body: dict[str, Any] | None = None,
) -> requests.Response:
    headers: dict[str, str] = {}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    if method.upper() in {"POST", "PATCH", "PUT", "DELETE"}:
        csrf = session.headers.get("X-CSRF-Token")
        if not csrf:
            csrf = fetch_csrf(session)
            session.headers.update({"X-CSRF-Token": csrf})
        headers["X-CSRF-Token"] = csrf
    return session.request(
        method,
        f"{BASE_URL_API}{path}",
        params=params,
        json=json_body,
        headers=headers,
        timeout=15,
    )


def expect_error_contract(
    resp: requests.Response, *, expected_status: int, expected_code: str, label: str
) -> None:
    if resp.status_code != expected_status:
        fail(f"{label} returned {resp.status_code}, expected {expected_status}: {resp.text[:200]}")
    try:
        body = resp.json()
    except json.JSONDecodeError as exc:
        raise RuntimeError(f"{label} returned non-JSON body: {resp.text[:200]}") from exc
    code = body.get("code")
    if code != expected_code:
        fail(f"{label} returned code={code!r}, expected {expected_code!r}")


def build_download_path(content_id: int, attachment_id: int) -> str:
    return f"/api/v1/contents/{content_id}/download?attachment_id={attachment_id}"


def run_search_page_validation(query: str, expected_title: str, forbidden_titles: list[str]) -> None:
    ensure_dir(SCREENSHOT_DIR)
    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        page = browser.new_page(viewport={"width": 1280, "height": 900})
        try:
            page.goto(f"{BASE_URL_FRONT}/search", wait_until="networkidle", timeout=30000)
            search_input = None
            selectors = [
                'input[type="search"]',
                'input[name="q"]',
                'input[role="searchbox"]',
                'input[placeholder*="搜索"]',
                'input[placeholder*="Search"]',
            ]
            for selector in selectors:
                locator = page.locator(selector).first
                if locator.count() > 0 and locator.is_visible(timeout=2000):
                    search_input = locator
                    break
            if search_input is None:
                fail("search page did not expose a visible search input")

            record("2-Search-Page-Landmarks", True, "search input is visible before interaction")

            with page.expect_response(
                lambda resp: "/api/v1/contents/search" in resp.url
                and parse_qs(urlparse(resp.url).query).get("q", []) == [query],
                timeout=15000,
            ) as response_info:
                search_input.fill(query)
                search_input.press("Enter")

            response = response_info.value
            if response.status != 200:
                fail(f"search request returned HTTP {response.status}")
            response_body = response.json()
            items = response_body.get("items")
            if not isinstance(items, list):
                fail("search response is missing an items list")
            if not any(isinstance(item, dict) and item.get("title") == expected_title for item in items):
                fail(f"search response did not contain expected title {expected_title!r}")

            parsed_query = parse_qs(urlparse(response.url).query).get("q", [])
            if parsed_query != [query]:
                fail(f"browser search request used q={parsed_query!r}, expected {[query]!r}")

            page.wait_for_timeout(1200)
            try:
                page.wait_for_selector(f"text={expected_title}", timeout=10000)
            except PlaywrightTimeoutError as exc:
                raise RuntimeError(f"search UI did not render expected title {expected_title!r}") from exc

            body_text = page.inner_text("body")
            for forbidden_title in forbidden_titles:
                if forbidden_title and forbidden_title in body_text:
                    fail(f"search UI rendered forbidden title {forbidden_title!r}")

            page.screenshot(path=str(SCREENSHOT_DIR / "12-search-page.png"), full_page=True)
            record(
                "4-Browser-Search",
                True,
                f"captured q={query!r} request and rendered {expected_title!r}",
            )
        finally:
            browser.close()


def run_api_search_validation(session: requests.Session, query: str, expected_title: str, forbidden_titles: list[str]) -> None:
    resp = request_json(session, "GET", "/api/v1/contents/search", params={"q": query})
    if resp.status_code != 200:
        fail(f"API search returned {resp.status_code}: {resp.text[:200]}")
    body = resp.json()
    items = body.get("items")
    total = body.get("total")
    if not isinstance(items, list):
        fail("API search response is missing items list")
    if not isinstance(total, int):
        fail("API search response is missing integer total")
    if not any(isinstance(item, dict) and item.get("title") == expected_title for item in items):
        fail(f"API search did not return expected title {expected_title!r}")
    titles = [item.get("title") for item in items if isinstance(item, dict)]
    for forbidden_title in forbidden_titles:
        if forbidden_title in titles:
            fail(f"API search returned forbidden title {forbidden_title!r}")
    record("5-API-Search-Visibility", True, f"total={total}, expected title present, forbidden titles absent")


def run_download_negative(session: requests.Session, path: str) -> None:
    resp = session.get(f"{BASE_URL_API}{path}", timeout=15)
    expect_error_contract(
        resp,
        expected_status=EXPECTED_NOAUTH_STATUS,
        expected_code=EXPECTED_NOAUTH_CODE,
        label="unauthenticated download",
    )
    record("6-Download-NoAuth", True, f"status={EXPECTED_NOAUTH_STATUS}, code={EXPECTED_NOAUTH_CODE}")


def run_download_success(session: requests.Session, token: str, path: str) -> None:
    resp = request_json(session, "GET", path, token=token)
    if resp.status_code != 200:
        fail(f"verified download returned {resp.status_code}: {resp.text[:200]}")
    if resp.headers.get("Location"):
        fail("verified download must return JSON, not a redirect")
    try:
        body = resp.json()
    except json.JSONDecodeError as exc:
        raise RuntimeError(f"verified download returned non-JSON body: {resp.text[:200]}") from exc
    download_url = body.get("download_url")
    expires_in = body.get("expires_in")
    if not isinstance(download_url, str) or not download_url:
        fail(f"download response is missing download_url: {body!r}")
    if not isinstance(expires_in, int) or expires_in <= 0:
        fail(f"download response is missing positive expires_in: {body!r}")
    record("7-Download-Verified", True, f"received signed url with expires_in={expires_in}")


def run_download_rejection(session: requests.Session, path: str, token: str, *, step: str, status: int, code: str) -> None:
    resp = request_json(session, "GET", path, token=token)
    expect_error_contract(resp, expected_status=status, expected_code=code, label=step)
    record(step, True, f"status={status}, code={code}")


def main() -> int:
    try:
        query = require_value("OMNICRAFT_SEARCH_QUERY", SEARCH_QUERY)
        expected_title = require_value("OMNICRAFT_EXPECTED_RESULT_TITLE", EXPECTED_RESULT_TITLE)
        content_id = require_int("OMNICRAFT_DOWNLOAD_CONTENT_ID", DOWNLOAD_CONTENT_ID)
        attachment_id = require_int("OMNICRAFT_DOWNLOAD_ATTACHMENT_ID", DOWNLOAD_ATTACHMENT_ID)
        download_path = build_download_path(content_id, attachment_id)

        verified_session, verified_token = login_verified_user()
        run_search_page_validation(query, expected_title, FORBIDDEN_SEARCH_TITLES)
        run_api_search_validation(verified_session, query, expected_title, FORBIDDEN_SEARCH_TITLES)
        run_download_negative(requests.Session(), download_path)
        run_download_success(verified_session, verified_token, download_path)

        unverified_token = get_fixture_token(
            "OMNICRAFT_UNVERIFIED_BEARER_TOKEN",
            "OMNICRAFT_UNVERIFIED_USER_ID",
            "unverified download",
        )
        low_rep_token = get_fixture_token(
            "OMNICRAFT_LOW_REPUTATION_BEARER_TOKEN",
            "OMNICRAFT_LOW_REPUTATION_USER_ID",
            "low reputation download",
        )

        run_download_rejection(
            verified_session,
            download_path,
            unverified_token,
            step="8-Download-Unverified",
            status=EXPECTED_UNVERIFIED_STATUS,
            code=EXPECTED_UNVERIFIED_CODE,
        )
        run_download_rejection(
            verified_session,
            download_path,
            low_rep_token,
            step="9-Download-LowReputation",
            status=EXPECTED_LOW_REPUTATION_STATUS,
            code=EXPECTED_LOW_REPUTATION_CODE,
        )
        missing_path = build_download_path(MISSING_CONTENT_ID, attachment_id)
        run_download_rejection(
            verified_session,
            missing_path,
            verified_token,
            step="10-Download-MissingContent",
            status=EXPECTED_MISSING_STATUS,
            code=EXPECTED_MISSING_CODE,
        )
    except Exception as exc:
        record("setup-or-run", False, str(exc))

    print("\n" + "=" * 60)
    print("TEST SUMMARY")
    print("=" * 60)
    total = len(results)
    passed = sum(1 for value in results.values() if value["status"] == "PASS")
    failed = total - passed
    for step, info in results.items():
        print(f"  [{info['status']}] {step}: {info['detail']}")
    print(f"\nTotal: {total} | Passed: {passed} | Failed: {failed}")
    print("=" * 60)
    return 0 if failed == 0 else 1


if __name__ == "__main__":
    sys.exit(main())
