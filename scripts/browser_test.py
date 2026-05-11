"""Browser test for OmniCraft seeded test data."""
from playwright.sync_api import sync_playwright
import os

SCREENSHOTS = os.path.join(os.path.dirname(os.path.dirname(__file__)), "screenshots")
os.makedirs(SCREENSHOTS, exist_ok=True)

PAGES = [
    ("Original Home", "http://localhost:3000/original"),
    ("Fanwork Home", "http://localhost:3000"),
    ("IP Detail - Genshin", "http://localhost:3000/ip/27"),
    ("Original Content Detail", "http://localhost:3000/original/63"),
    ("Fanwork Content Detail", "http://localhost:3000/content/77"),
]

errors = []

with sync_playwright() as p:
    browser = p.chromium.launch(headless=True)
    context = browser.new_context(
        viewport={"width": 1440, "height": 900},
        locale="zh-CN"
    )
    page = context.new_page()

    # Capture console errors
    page.on("console", lambda msg: errors.append(f"[{msg.type}] {msg.text}") if msg.type in ("error", "warning") else None)
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
            content_el = page.locator("text=原神, text=流浪地球, text=黑神话, text=饭团, text=橘猫, text=枫丹").first
            has_content = content_el.is_visible() if content_el else False
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
