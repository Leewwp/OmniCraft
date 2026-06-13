"""Browser smoke test for OmniCraft seeded test data.

Environment variables:
  OMNICRAFT_API_URL - Backend API URL (default: http://localhost:8080/api/v1)
  OMNICRAFT_WEB_URL - Frontend URL (default: http://localhost:3000)

Exit codes:
  0 - all pages loaded without errors
  1 - one or more errors collected
"""
from playwright.sync_api import sync_playwright
import json
import os
import sys
import urllib.request
import urllib.error

SCREENSHOTS = os.path.join(os.path.dirname(os.path.dirname(__file__)), "screenshots")
os.makedirs(SCREENSHOTS, exist_ok=True)

API_URL = os.environ.get("OMNICRAFT_API_URL", "http://localhost:8080/api/v1")
WEB_URL = os.environ.get("OMNICRAFT_WEB_URL", "http://localhost:3000")


def api_get(path):
    """GET from API, return parsed JSON or None."""
    url = f"{API_URL}{path}"
    try:
        with urllib.request.urlopen(url, timeout=10) as r:
            return json.loads(r.read())
    except Exception as e:
        print(f"  API lookup failed: {e}")
        return None


def resolve_page_urls():
    """Build page list with dynamic IDs from the API instead of hard-coded ones."""
    pages = [
        ("Original Home", f"{WEB_URL}/original"),
        ("Fanwork Home", WEB_URL),
    ]

    # Resolve IP detail URL dynamically
    ip_data = api_get("/ips?page_size=1&category=gaming")
    if ip_data:
        ips = ip_data.get("ips", ip_data.get("data", []))
        if ips:
            pages.append(("IP Detail", f"{WEB_URL}/ip/{ips[0]['id']}"))

    # Resolve original content detail URL dynamically
    orig_data = api_get("/contents?zone=original&page_size=1")
    if orig_data:
        contents = orig_data.get("contents", orig_data.get("data", []))
        if contents:
            pages.append(("Original Content Detail", f"{WEB_URL}/original/{contents[0]['id']}"))

    # Resolve fanwork content detail URL dynamically
    fw_data = api_get("/contents?zone=fanwork&page_size=1")
    if fw_data:
        contents = fw_data.get("contents", fw_data.get("data", []))
        if contents:
            pages.append(("Fanwork Content Detail", f"{WEB_URL}/content/{contents[0]['id']}"))

    return pages


PAGES = resolve_page_urls()
errors = []

with sync_playwright() as p:
    browser = p.chromium.launch(headless=True)
    context = browser.new_context(
        viewport={"width": 1440, "height": 900},
        locale="zh-CN"
    )
    page = context.new_page()

    # Capture console errors
    page.on("console", lambda msg: errors.append(f"[{msg.type}] {msg.text}") if msg.type == "error" else None)
    page.on("pageerror", lambda err: errors.append(f"[PAGE ERROR] {err}"))

    for name, url in PAGES:
        print(f"\n=== {name}: {url} ===")
        try:
            page.goto(url, timeout=15000, wait_until="networkidle")
            page.wait_for_timeout(1000)

            safe_name = name.lower().replace(" ", "_").replace("-", "_")
            path = os.path.join(SCREENSHOTS, f"{safe_name}.png")
            page.screenshot(path=path, full_page=True)
            print(f"  Screenshot: {path}")

            title = page.title()
            print(f"  Title: {title[:80]}")

            # Quick content check
            cards = page.locator("[class*='card'], [class*='Card'], [class*='masonry']").count()
            print(f"  Content elements found: {cards}")
        except Exception as e:
            print(f"  ERROR: {e}")
            errors.append(f"[NAV ERROR] {name}: {e}")

    browser.close()

print(f"\n=== Done: {len(PAGES)} pages tested, {len(errors)} errors ===")
if errors:
    print("Errors/Warnings:")
    for e in errors[:20]:
        print(f"  {e[:150]}")

# Save error log
if errors:
    with open(os.path.join(SCREENSHOTS, "errors.txt"), "w") as f:
        for e in errors:
            f.write(e + "\n")

# Exit non-zero if errors were collected (TST-029)
if errors:
    sys.exit(1)
