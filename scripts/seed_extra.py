"""Create missing IP and fanworks.

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
    if TOKEN:
        headers["Authorization"] = f"Bearer {TOKEN}"
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


# Login
print("Logging in...")
r = api_fatal("POST", "/auth/login",
              {"email": SEED_EMAIL, "password": SEED_PASSWORD},
              label=f"Login as {SEED_EMAIL}")
TOKEN = r["tokens"]["access_token"]
print(f"Logged in as uid={r['user']['id']}")

# Get existing IPs ID mapping
print("\nGetting IPs...")
r = api("GET", "/ips?page_size=50")
if r is None:
    print("FATAL: cannot fetch IPs from API", file=sys.stderr)
    sys.exit(1)
ips = {ip["name"]: ip["id"] for ip in r.get("ips", r.get("data", []))}
print(f"Found {len(ips)} IPs")
for n, i in ips.items():
    print(f"  [{i}] {n}")

# Create missing 明日方舟 IP (idempotent)
if "明日方舟" not in ips:
    print("\nCreating 明日方舟 IP...")
    r = api("POST", "/ips", {
        "name": "明日方舟",
        "description": "鹰角网络开发的策略塔防手游，末日废土世界观。",
        "category": "gaming",
        "tags": ["塔防","策略","末世","兽耳"]
    })
    if r:
        ip_id = r["ip"]["id"]
        print(f"  Created [{ip_id}] status={r['ip']['status']}")
        api("POST", f"/admin/ips/{ip_id}/approve")
        print(f"  Approved")
        ips["明日方舟"] = ip_id
else:
    print("\n  SKIP: 明日方舟 already exists")

# Get content IDs for source linking
r = api("GET", f"/contents?author_id={UID}&page_size=50")
if r is None:
    print("FATAL: cannot fetch contents from API", file=sys.stderr)
    sys.exit(1)
contents = {}
src_id = None
for c in r.get("contents", r.get("data", [])):
    kit = c["title"][:30]
    contents[kit] = c["id"]
    if "最后的信使" in c["title"]:
        src_id = c["id"]
print(f"\nFound {len(contents)} contents (source_original_id={src_id})")

# Fanworks to create
fanworks = [
    # (title, desc, ip_name, ctype, tags, use_source)
    ("明日方舟阿米娅升变4K壁纸级插画",
     "阿米娅升变形态是在方舟里最喜欢的设计。构图采用仰望视角，阿米娅站在甲板上，背后是移动城邦剪影。分辨率3840x2160，Clip Studio Paint EX绘制。",
     "明日方舟", "image", ["阿米娅","插画","壁纸"], False),
    ("原神角色胡桃Stable Diffusion LoRA模型分享",
     "训练了胡桃的角色LoRA。推荐参数：Model Anything V5, LoRA weight 0.75, Sampler DPM++ 2M Karras, Steps 28, CFG 7。脸部还原度高。",
     "原神", "prompt", ["LoRA","SD","胡桃","AI绘画"], False),
    ("流浪地球KSP行星发动机MOD v1.2",
     "在KSP中重现流浪地球的行星发动机！包含行星发动机零件3种规格、地壳锚固装置、地下城模块、领航员号空间站预制文件。解压到GameData/。",
     "流浪地球", "mod", ["KSP","MOD","太空"], False),
    ("鬼灭之刃灶门炭治郎之歌钢琴独奏谱",
     "扒了鬼灭第19集那首让人泪崩的插曲。中级难度英皇5-6级，降E大调，Andante J=72。左手琶音保持均匀，副歌八度跳跃需提前准备。",
     "鬼灭之刃", "sheet_music", ["乐谱","钢琴","OST"], False),
    ("银狼黑客帝国赛博朋克风格插画",
     "银狼太适合赛博朋克风格了！背景设定在星穹列车的下层甲板，霓虹灯管和全息广告。画面中隐藏了多处彩蛋。",
     "崩坏：星穹铁道", "image", ["赛博朋克","银狼","插画"], False),
    ("原神枫丹水下探索指南：全寻宝点与隐藏成就攻略",
     "枫丹水下探索是整个4.0版本最大的亮点。游泳加速消耗水之印。共38个水下宝箱加3个隐藏成就需要特定角色触发！",
     "原神-枫丹", "article", ["枫丹","攻略","原神","探索"], False),
    ("星穹同人：银河列车没有终点站",
     "## 第一章\n\n姬子关掉了通讯器。列车的窗外是一片她从未见过的星云。\n\n所有的频道都是同一个声音——一个巨大生物的心跳。\n\n前方的紫色星云中，一个巨大的轮廓正在缓缓浮现。\n\n未完待续。",
     "崩坏：星穹铁道", "article", ["同人文","科幻","星穹铁道"], True),
]

print("\n=== Creating Fanworks ===")
for title, desc, ip_name, ctype, tags, use_src in fanworks:
    if ip_name not in ips:
        print(f"  SKIP: IP {ip_name} not found")
        continue
    body = {
        "title": title, "description": desc,
        "zone": "fanwork", "ip_id": ips[ip_name],
        "content_type": ctype, "tags": tags
    }
    if use_src and src_id:
        body["source_original_id"] = src_id

    r = api("POST", "/contents", body)
    if r:
        c = r.get("content", r)
        print(f"  [{c['id']}] {title[:50]}  ({ctype})")
    else:
        print(f"  FAILED: {title[:40]}")

print("\nDone!")
