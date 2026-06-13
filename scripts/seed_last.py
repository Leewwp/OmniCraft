"""Create additional fanwork for testing.

Environment variables:
  SEED_BASE_URL  - API base URL (default: http://localhost:8080/api/v1)
  SEED_EMAIL     - Login email (default: demo@omnicraft.com)
  SEED_PASSWORD  - Login password (default: demo123456)
"""
import json, os, sys, urllib.request

BASE = os.environ.get("SEED_BASE_URL", "http://localhost:8080/api/v1")
SEED_EMAIL = os.environ.get("SEED_EMAIL", "demo@omnicraft.com")
SEED_PASSWORD = os.environ.get("SEED_PASSWORD", "demo123456")
TOKEN = None


def api(method, path, data=None):
    url = f"{BASE}{path}"
    body = json.dumps(data, ensure_ascii=False).encode() if data else None
    headers = {"Content-Type": "application/json; charset=utf-8"}
    if TOKEN: headers["Authorization"] = f"Bearer {TOKEN}"
    req = urllib.request.Request(url, data=body, method=method.upper(), headers=headers)
    try:
        with urllib.request.urlopen(req, timeout=15) as r:
            return json.loads(r.read())
    except Exception as e:
        print(f"  ERR: {e}")
        return None


def api_fatal(method, path, data=None, label=""):
    """Call api() and exit on failure — used for critical operations (TST-031)."""
    result = api(method, path, data)
    if result is None:
        desc = label or f"{method} {path}"
        print(f"FATAL: {desc} failed, aborting.", file=sys.stderr)
        sys.exit(1)
    return result


r = api_fatal("POST", "/auth/login",
              {"email": SEED_EMAIL, "password": SEED_PASSWORD},
              label=f"Login as {SEED_EMAIL}")
TOKEN = r["tokens"]["access_token"]

# Get 明日方舟 IP id
r = api("GET", "/ips?page_size=50")
ips = {ip["name"]: ip["id"] for ip in r.get("ips", r.get("data", []))}
print(f"IPs: {len(ips)}")

ark_id = ips.get("明日方舟")
if ark_id:
    r = api("POST", "/contents", {
        "title": "明日方舟阿米娅升变4K壁纸级插画",
        "description": "阿米娅升变形态是在方舟里最喜欢的设计。构图采用仰望视角，阿米娅站在甲板上，背后是移动城邦剪影。分辨率3840x2160，Clip Studio Paint EX绘制，耗时16小时。博士，今天的罗德岛也照常运行着。",
        "zone": "fanwork", "ip_id": ark_id,
        "content_type": "image", "tags": ["阿米娅","插画","壁纸","明日方舟"]
    })
    if r:
        print(f"Created [{r['content']['id']}] 明日方舟阿米娅插画")
    else:
        print("Failed")
else:
    print("明日方舟 IP not found!")
