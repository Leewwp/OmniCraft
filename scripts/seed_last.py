import json, urllib.request

BASE = "http://localhost:8080/api/v1"
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

r = api("POST", "/auth/login", {"email": "demo@omnicraft.com", "password": "demo123456"})
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
