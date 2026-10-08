#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""demo_showcase_seed -- 全类型演示造数脚本 (#848 / A5, 2026-10-08).

演示站 1400+ 条内容全为直插 DB 空壳行（零真实附件）；本脚本按 #848 造数
矩阵经**真实 API 管线**补齐全类型带真实附件的内容：

    login -> POST /contents/oss-token (presign + grant)
          -> PUT presigned OSS URL (真实二进制)
          -> POST /contents (媒体集/附件族/海报契约全走后端校验)
          -> 等待 Green 审核放行 (published)；滞留 pending 时
             POST /internal/ai-callback 旁路 (SHA256 checksum)
          -> 同作者 3 连发 x2 组挂 POST /series (+ /series/:id/items)

矩阵（类型归属单一事实源 backend/config.yaml content_registry）：
  原创区 8 类：image/article/video/audio/template/sheet_music/3d_print/other
  二创区 9 类：上述除 template 外 + mod/prompt (mod 仅二创、template 仅原创)
  每格 2~3 条 + 特殊形态（9 图满画廊 x1、3 视频含 poster x1、附件族全覆盖、
  fanwork 全挂 corpus IP、原创带封面图文）+ 2 组 series（激活 SeriesNav）。

凭据纪律：fixture 密码只从 env CORPUS_FIXTURE_PASSWORD 读取（演示服务器
env 注入，涉密协议），绝不硬编码/写入文件/打印到日志；GREEN_UID /
GREEN_SEED 仅审核旁路需要，同样只走 env。

用法：
    # 离线演练（不出网：生成+校验全部资产、打印全矩阵计划与限流模拟）
    python3 scripts/corpus/demo_showcase_seed.py --dry-run

    # 真跑（默认 localhost；演示站经 --api-base 指定 api 域）
    CORPUS_FIXTURE_PASSWORD=... python3 scripts/corpus/demo_showcase_seed.py \
        --execute --api-base https://<demo-api-host>/api/v1 \
        --account a01 --account a02

    # 粒度控制
    ... --only org-image-03        # 只跑单条（断点续跑友好）
    ... --limit 5                  # 只跑前 N 条

依赖：Python 3.9+ 标准库（requests 不需要）；可选 ffmpeg（音频走 MP3 合成，
缺失时自动退化为纯 Python WAV，两者文件头 magic 均匹配扩展名）。断点续跑：
checkpoint JSONL（复用 corpus_lib.ItemState 模式）+ 内容 tag `ds:<key>`
幂等锚（重跑不重复发布）。

限流预算：/contents/oss-token 与 /contents 各耗 1 次/调用，每账号 200/时
（config rate_limit.upload_per_hour）；两枚 corpus 种子作者号轮换，脚本内置
逐时窗口账本，超窗自动切号/等待。凭证端点 5 次/分/IP，登录节流内建。
"""
from __future__ import annotations

import argparse
import http.cookiejar
import json
import os
import re
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
from typing import Any, Dict, List, Optional, Tuple

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import corpus_lib as lib  # noqa: E402  (RateLimiter / CircuitBreaker / callback helpers)
import demo_showcase_assets as assets  # noqa: E402

REPO_ROOT = assets.REPO_ROOT
CORPUS_EMAIL_SUFFIX = lib.CORPUS_EMAIL_SUFFIX

DEFAULT_API_BASE = os.environ.get("API_BASE", "http://localhost:8080/api/v1")
DEFAULT_WORKDIR = os.environ.get(
    "DEMO_SEED_WORKDIR", os.path.join(REPO_ROOT, "artifacts", "demo-showcase-seed")
)
UPLOAD_PER_HOUR_DEFAULT = 200          # backend/config.yaml rate_limit.upload_per_hour
VIDEO_MAX_SEC = 180                    # backend/config.yaml limits.video_max_sec
ACCESS_TTL_MARGIN = 110                # access_token_ttl=120s, refresh margin (same as injector)
MAX_LOGINS_PER_MIN = 3                 # credential endpoint allows 5/min per IP
PUBLISH_WAIT_TIMEOUT_DEFAULT = 90      # Green 正常内容秒级放行；90s 兜底
IDEMPOTENCY_TAG_PREFIX = "ds:"         # content-tag idempotency anchor (c2: pattern)
DEMO_TAG = "演示造数"

# 11 类目（config.yaml ip_categories allowlist；dry-run 兜底常量，运行时优先
# 读仓库 config.yaml 保持单一事实源）。
IP_CATEGORIES_FALLBACK = [
    "game", "film_tv", "anime", "manga", "novel", "literature",
    "music", "variety", "short_drama", "vtuber", "other",
]

# fanwork IP 兜底名单（corpus-v2 ip_roster）；execute 时以 GET /ips 的
# approved 列表为准并轮换。
CORPUS_IP_ROSTER_FALLBACK = lib.IP_CATEGORY.keys()


def log(msg: str) -> None:
    print("[%s] %s" % (time.strftime("%H:%M:%S"), msg), flush=True)


# ---------------------------------------------------------------- repo config

def load_repo_config_values() -> Dict[str, Any]:
    """Sniff the few scalars we mirror from backend/config.yaml (single source).

    Mini targeted parse (stdlib, no yaml dep): ip_categories list block +
    upload_per_hour. Missing file/keys fall back to the shipped defaults.
    """
    values: Dict[str, Any] = {
        "ip_categories": list(IP_CATEGORIES_FALLBACK),
        "upload_per_hour": UPLOAD_PER_HOUR_DEFAULT,
    }
    path = os.path.join(REPO_ROOT, "backend", "config.yaml")
    try:
        with open(path, "r", encoding="utf-8") as fh:
            text = fh.read()
    except OSError:
        return values
    block = re.search(r"^ip_categories:\n((?:\s+-\s+\w+\n)+)", text, re.M)
    if block:
        cats = re.findall(r"-\s+(\w+)", block.group(1))
        if cats:
            values["ip_categories"] = cats
    hour = re.search(r"^\s+upload_per_hour:\s*(\d+)\s*$", text, re.M)
    if hour:
        values["upload_per_hour"] = int(hour.group(1))
    return values


# ---------------------------------------------------------------- plan model

TYPE_LABEL = {
    "image": "图片", "article": "文章", "video": "视频", "audio": "音频",
    "template": "模板", "sheet_music": "乐谱", "3d_print": "3D打印",
    "other": "其他", "mod": "模组", "prompt": "提示词",
}
ZONE_LABEL = {"original": "原创", "fanwork": "二创"}

ARTICLE_BODY = (
    "这是一篇用于演示验收的示例正文。本条内容由造数脚本通过真实发布管线写入："
    "表单校验、审核门与版本快照全部走线上路径，因此它与人工发布的作品行为一致。\n\n"
    "正文用于验收文章类查看器：段落间距、行高、中文排版与长文本滚动。"
    "段落之间保留空行，以覆盖 Markdown 段落解析的展示路径。"
)
PROMPT_BODY = (
    "【角色】你是一位资深场景概念设计师。\n"
    "【任务】基于以下关键词产出一段 200 字以内的场景描述：晨光、旧书店、木质书梯、灰尘。\n"
    "【约束】不出现真实人名；结尾附一句可用于封面的短标题。\n"
    "【输出】一段场景描述 + 一个短标题。"
)
OTHER_BODY = (
    "本条为 other 品类演示：无附件、无固定形态，正文即内容。\n"
    "用于验收文本类内容在列表与详情页的最小展示路径。"
)


def _img(lib_images: List[Dict[str, Any]], start: int, count: int) -> List[Dict[str, Any]]:
    """Pick `count` images starting at `start` with wraparound."""
    n = len(lib_images)
    return [lib_images[(start + i) % n] for i in range(count)]


def build_plan(library: Dict[str, List[Dict[str, Any]]], accounts: List[str]) -> Tuple[List[Dict[str, Any]], List[Dict[str, Any]]]:
    """Deterministic full-matrix plan. Returns (items, series_specs).

    Series members must share one author (AddItem ownership rule), so series
    groups are pinned to the account their first member drew; every other
    item round-robins the two fixture accounts.
    """
    img = library["image"]
    vid = library["video"]
    items: List[Dict[str, Any]] = []

    def add(
        zone: str, ctype: str, num: int, *, account: str,
        attachments: Optional[List[Dict[str, Any]]] = None,
        poster: Optional[Dict[str, Any]] = None,
        cover: Optional[Dict[str, Any]] = None,
        body: Optional[str] = None, category: Optional[str] = None,
        special: Optional[str] = None, series: Optional[str] = None,
        ip_hint: Optional[str] = None,
    ) -> Dict[str, Any]:
        zone_label = ZONE_LABEL[zone]
        type_label = TYPE_LABEL[ctype]
        title = "演示·%s·%s·%02d" % (zone_label, type_label, num)
        if special:
            title += "·" + special
        key = "%s-%s-%02d" % ("org" if zone == "original" else "fan", ctype, num)
        item = {
            "key": key, "zone": zone, "content_type": ctype, "title": title,
            "description": body or ("%s 演示条目（%s/%s）。由 #848 A5 造数脚本生成。" % (type_label, zone_label, type_label)),
            "account": account, "attachments": attachments or [], "poster": poster,
            "cover": cover, "category": category, "special": special,
            "series": series, "ip_hint": ip_hint,
            "tags": [DEMO_TAG, IDEMPOTENCY_TAG_PREFIX + key],
        }
        items.append(item)
        return item

    # -------------------------------------------------- original (8 types, 18)
    add("original", "image", 1, account=accounts[0], attachments=_img(img, 0, 3), category="other")
    add("original", "image", 2, account=accounts[1 % len(accounts)], attachments=_img(img, 3, 2), category="other")
    add("original", "image", 3, account=accounts[0], attachments=_img(img, 5, 9), category="other", special="九图满画廊")
    add("original", "article", 1, account=accounts[1 % len(accounts)], body=ARTICLE_BODY, cover=img[14], category="literature", special="带封面图文", series="S1")
    add("original", "article", 2, account=accounts[0], body=ARTICLE_BODY, category="literature", series="S1")
    add("original", "article", 3, account=accounts[1 % len(accounts)], body=ARTICLE_BODY, category="literature", series="S1")
    add("original", "video", 1, account=accounts[0], attachments=[vid[1]], poster=img[15], category="film_tv")
    add("original", "video", 2, account=accounts[1 % len(accounts)], attachments=[vid[0], vid[1]], poster=img[13], category="film_tv")
    add("original", "audio", 1, account=accounts[0], attachments=[library["audio"][0]], category="music")
    add("original", "audio", 2, account=accounts[1 % len(accounts)], attachments=library["audio"], category="music")
    add("original", "template", 1, account=accounts[0], attachments=[library["document"][0], library["document"][1]], category="other")
    add("original", "template", 2, account=accounts[1 % len(accounts)], attachments=[library["document"][0]], category="other")
    add("original", "sheet_music", 1, account=accounts[0], attachments=[library["sheet_music"][0]], category="music")
    add("original", "sheet_music", 2, account=accounts[1 % len(accounts)], attachments=[library["sheet_music"][0]], category="music")
    add("original", "3d_print", 1, account=accounts[0], attachments=[library["model3d"][0], library["text"][0]], category="other", special="STL加文本")
    add("original", "3d_print", 2, account=accounts[1 % len(accounts)], attachments=[library["model3d"][0]], category="other")
    add("original", "other", 1, account=accounts[0], body=OTHER_BODY, cover=img[12], category="other")
    add("original", "other", 2, account=accounts[1 % len(accounts)], body=OTHER_BODY, category="other")

    # -------------------------------------------------- fanwork (9 types, 19)
    fan_accounts = list(reversed(accounts))  # opposite parity start
    add("fanwork", "image", 1, account=fan_accounts[0], attachments=_img(img, 8, 3), ip_hint="原神")
    add("fanwork", "image", 2, account=fan_accounts[1 % len(fan_accounts)], attachments=_img(img, 11, 2), ip_hint="魔道祖师")
    add("fanwork", "article", 1, account=fan_accounts[0], body=ARTICLE_BODY, ip_hint="诡秘之主", series="S2")
    add("fanwork", "article", 2, account=fan_accounts[1 % len(fan_accounts)], body=ARTICLE_BODY, ip_hint="全职高手", series="S2")
    add("fanwork", "article", 3, account=fan_accounts[0], body=ARTICLE_BODY, ip_hint="盗墓笔记", series="S2")
    add("fanwork", "video", 1, account=fan_accounts[1 % len(fan_accounts)], attachments=[vid[0]], poster=img[10], ip_hint="双城之战")
    add("fanwork", "video", 2, account=fan_accounts[0], attachments=[vid[0], vid[1], vid[2]], poster=img[15], ip_hint="双城之战", special="三视频连播")
    add("fanwork", "audio", 1, account=fan_accounts[1 % len(fan_accounts)], attachments=[library["audio"][1]], ip_hint="天官赐福")
    add("fanwork", "audio", 2, account=fan_accounts[0], attachments=[library["audio"][0]], ip_hint="哈利·波特")
    add("fanwork", "mod", 1, account=fan_accounts[1 % len(fan_accounts)], attachments=[library["mod"][0]], ip_hint="王者荣耀")
    add("fanwork", "mod", 2, account=fan_accounts[0], attachments=[library["mod"][0]], ip_hint="宝可梦")
    add("fanwork", "prompt", 1, account=fan_accounts[1 % len(fan_accounts)], body=PROMPT_BODY, ip_hint="罗小黑战记")
    add("fanwork", "prompt", 2, account=fan_accounts[0], body=PROMPT_BODY, ip_hint="火影忍者")
    add("fanwork", "sheet_music", 1, account=fan_accounts[1 % len(fan_accounts)], attachments=[library["sheet_music"][0]], ip_hint="海贼王")
    add("fanwork", "sheet_music", 2, account=fan_accounts[0], attachments=[library["sheet_music"][0]], ip_hint="哪吒/封神宇宙")
    add("fanwork", "3d_print", 1, account=fan_accounts[1 % len(fan_accounts)], attachments=[library["model3d"][0], library["text"][0]], ip_hint="崩坏：星穹铁道", special="STL加文本")
    add("fanwork", "3d_print", 2, account=fan_accounts[0], attachments=[library["model3d"][0]], ip_hint="西游记（孙悟空）")
    add("fanwork", "other", 1, account=fan_accounts[1 % len(fan_accounts)], body=OTHER_BODY, ip_hint="宝可梦")
    add("fanwork", "other", 2, account=fan_accounts[0], body=OTHER_BODY, ip_hint="原神")

    # series specs: members resolved after items exist. Series membership is
    # ownership-bound (AddItem rejects content not authored by the series
    # owner), so each group is pinned to ONE fixture account -- S1 draws the
    # first account, S2 the last, keeping the rotation balanced.
    series_specs = [
        {
            "key": "S1", "zone": "original", "pin_account": accounts[0],
            "title": "演示·合集·原创创作手册",
            "description": "同作者三连发（原创区）：选题、结构、复盘。",
        },
        {
            "key": "S2", "zone": "fanwork", "pin_account": accounts[-1],
            "title": "演示·合集·二创连载",
            "description": "同作者三连发（二创区）：连载三章。",
        },
    ]
    for spec in series_specs:
        spec["members"] = [it["key"] for it in items if it.get("series") == spec["key"]]
        for it in items:
            if it.get("series") == spec["key"]:
                it["account"] = spec["pin_account"]
        spec["account"] = spec["pin_account"]
    return items, series_specs


def item_quota_cost(item: Dict[str, Any]) -> int:
    """Upload-quota units: 1 per presign + 1 per publish (cover counts too)."""
    return 1 + len(item["attachments"]) + (1 if item.get("poster") else 0) + (1 if item.get("cover") else 0)


def simulate_quota(items: List[Dict[str, Any]], limit_per_hour: int) -> List[Dict[str, Any]]:
    """Client-side ledger projection: per-account calendar-hour windows.

    The projection spreads items deterministically over consecutive windows
    starting now; the execute path keys the same ledger on real wall-clock
    windows and waits for the boundary instead of switching accounts (upload
    grants are bound to the publishing author, so an item cannot migrate).
    """
    ledger: Dict[str, Dict[int, int]] = {}
    for idx, item in enumerate(items):
        account = item["account"]
        window = (int(time.time()) + idx) // 3600
        ledger.setdefault(account, {})
        ledger[account][window] = ledger[account].get(window, 0) + item_quota_cost(item)
    rows = []
    for account in sorted(ledger):
        for window in sorted(ledger[account]):
            used = ledger[account][window]
            flag = "OK" if used <= limit_per_hour else "OVER"
            rows.append({"account": account, "window": window, "units": used, "flag": flag})
    return rows


# ---------------------------------------------------------------- checkpoint

class DemoItemState:
    """Checkpoint state machine for one demo item (corpus_lib.ItemState pattern)."""

    def __init__(self, key: str) -> None:
        self.key = key
        self.content_id: Optional[int] = None
        self.published: bool = False
        self.series_done: bool = False
        self.error: Optional[str] = None

    @property
    def done(self) -> bool:
        return self.content_id is not None and self.published and self.series_done

    def to_json(self) -> str:
        return json.dumps(
            {"key": self.key, "content_id": self.content_id, "published": self.published,
             "series_done": self.series_done, "error": self.error},
            ensure_ascii=False, separators=(",", ":"),
        )

    @classmethod
    def from_json(cls, line: str) -> "DemoItemState":
        raw = json.loads(line)
        state = cls(raw["key"])
        state.content_id = raw.get("content_id")
        state.published = bool(raw.get("published"))
        state.series_done = bool(raw.get("series_done"))
        state.error = raw.get("error")
        return state


class Checkpoint:
    def __init__(self, workdir: str) -> None:
        os.makedirs(workdir, exist_ok=True)
        self.path = os.path.join(workdir, "checkpoint.jsonl")
        self.states: Dict[str, DemoItemState] = {}
        if os.path.exists(self.path):
            with open(self.path, "r", encoding="utf-8") as fh:
                for line in fh:
                    if line.strip():
                        state = DemoItemState.from_json(line)
                        self.states[state.key] = state
        self._fh = open(self.path, "a", encoding="utf-8")

    def get(self, key: str) -> DemoItemState:
        if key not in self.states:
            self.states[key] = DemoItemState(key)
        return self.states[key]

    def save(self, state: DemoItemState) -> None:
        self.states[state.key] = state
        self._fh.write(state.to_json() + "\n")
        self._fh.flush()


# ---------------------------------------------------------------- API client

class Api:
    """REST client mirroring corpus_injector's shape (CSRF + login throttling)."""

    def __init__(self, api_base: str, rps: float = 2.0) -> None:
        self.api_base = api_base.rstrip("/")
        self.password = lib.FIXTURE_PASSWORD
        self.tokens: Dict[str, Tuple[str, float]] = {}
        self.login_times: List[float] = []
        self.limiter = lib.RateLimiter(rps)
        self.jar = http.cookiejar.CookieJar()
        self.opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(self.jar))
        self.quota_used: Dict[str, Dict[int, int]] = {}  # account -> hour window -> units

    # -- raw ---------------------------------------------------------------

    def _csrf_token(self) -> str:
        # CSRF cookie name is mode-aware (middleware/csrf.go): release uses the
        # __Host- prefix, debug uses the plain name. Match either so the script
        # runs against both local dev and the release-mode demo.
        for cookie in self.jar:
            if cookie.name in ("__Host-csrf", "csrf-token"):
                return cookie.value
        return ""

    def request(
        self,
        method: str,
        path: str,
        body: Optional[Dict[str, Any]] = None,
        token: Optional[str] = None,
        form: Optional[Dict[str, str]] = None,
        timeout: int = 60,
    ) -> Tuple[int, Dict[str, Any]]:
        url = self.api_base + path
        data = None
        headers = {"Accept": "application/json"}
        if form is not None:
            data = urllib.parse.urlencode(form).encode("utf-8")
            headers["Content-Type"] = "application/x-www-form-urlencoded"
        elif body is not None:
            data = json.dumps(body, ensure_ascii=False).encode("utf-8")
            headers["Content-Type"] = "application/json"
        if token:
            headers["Authorization"] = "Bearer " + token
        if method in ("POST", "PATCH", "PUT", "DELETE") and "/internal/" not in path:
            if not self._csrf_token():
                self.opener.open(
                    urllib.request.Request(self.api_base + "/auth/csrf", method="GET"), timeout=30
                ).read()
            csrf = self._csrf_token()
            if csrf:
                headers["X-CSRF-Token"] = csrf
        req = urllib.request.Request(url, data=data, headers=headers, method=method)
        self.limiter.wait()
        try:
            with self.opener.open(req, timeout=timeout) as resp:
                payload = resp.read().decode("utf-8")
                status = resp.status
        except urllib.error.HTTPError as err:
            payload = err.read().decode("utf-8", "replace")
            status = err.code
        parsed: Dict[str, Any] = {}
        if payload:
            try:
                parsed = json.loads(payload)
            except ValueError:
                parsed = {"_raw": payload[:500]}
        return status, parsed

    # -- auth ----------------------------------------------------------------

    def login(self, account: str) -> str:
        if not self.password:
            raise RuntimeError("CORPUS_FIXTURE_PASSWORD is not set (demo server env only)")
        cached = self.tokens.get(account)
        if cached and cached[1] > time.time() + 5:
            return cached[0]
        now = time.time()
        self.login_times = [t for t in self.login_times if now - t < 60]
        if len(self.login_times) >= MAX_LOGINS_PER_MIN:
            sleep = 60 - (now - self.login_times[0]) + 1
            log("login throttle: sleeping %.1fs" % sleep)
            time.sleep(max(sleep, 1.0))
        email = account + CORPUS_EMAIL_SUFFIX
        status, payload = self.request("POST", "/auth/login", {"email": email, "password": self.password})
        if status == 429:
            time.sleep(90)
            status, payload = self.request("POST", "/auth/login", {"email": email, "password": self.password})
        self.login_times.append(time.time())
        if status != 200 or "tokens" not in payload:
            raise RuntimeError("login failed for %s: %d %s" % (account, status, str(payload)[:200]))
        access = payload["tokens"]["access_token"]
        self.tokens[account] = (access, time.time() + ACCESS_TTL_MARGIN)
        return access

    # -- upload quota ledger ---------------------------------------------------

    def quota_window(self) -> int:
        return int(time.time()) // 3600

    def quota_reserve(self, account: str, units: int = 1) -> bool:
        window = self.quota_window()
        bucket = self.quota_used.setdefault(account, {})
        if bucket.get(window, 0) + units > UPLOAD_PER_HOUR_DEFAULT:
            return False
        bucket[window] = bucket.get(window, 0) + units
        return True

    # -- pipeline steps ----------------------------------------------------------

    def presign(self, token: str, asset: Dict[str, Any]) -> Dict[str, Any]:
        body: Dict[str, Any] = {
            "file_name": asset["name"],
            "file_type": asset["file_type"],
            "mime_type": asset["mime_type"],
            "file_size": asset["size"],
        }
        if asset.get("duration_sec") is not None:
            body["duration_sec"] = int(asset["duration_sec"])
        status, payload = self.request("POST", "/contents/oss-token", body, token=token)
        if status != 200 or "upload_url" not in payload:
            raise RuntimeError("presign failed for %s (%s): %d %s" % (
                asset["name"], asset["file_type"], status, str(payload)[:300]))
        return payload

    def put_object(self, upload_url: str, asset: Dict[str, Any]) -> None:
        with open(asset["path"], "rb") as fh:
            data = fh.read()
        if len(data) != asset["size"]:
            raise RuntimeError("asset %s size drifted before upload" % asset["name"])
        req = urllib.request.Request(
            upload_url, data=data, method="PUT",
            headers={"Content-Type": asset["mime_type"]},
        )
        self.limiter.wait()
        try:
            with self.opener.open(req, timeout=600) as resp:
                resp.read()
        except urllib.error.HTTPError as err:
            raise RuntimeError("OSS PUT failed for %s: %d %s" % (
                asset["name"], err.code, err.read().decode("utf-8", "replace")[:200]))

    def find_by_key(self, viewer_token: str, key: str) -> Optional[int]:
        """Idempotency lookup: the ds:<key> tag is the DB-side anchor."""
        query = urllib.parse.quote(IDEMPOTENCY_TAG_PREFIX + key)
        status, payload = self.request("GET", "/contents?tags=" + query + "&page=1&page_size=5", token=viewer_token)
        if status != 200:
            return None
        rows = payload.get("contents") or payload.get("items") or []
        for row in rows:
            if int(row.get("id", 0) or 0) > 0:
                return int(row["id"])
        return None

    def publish(self, token: str, item: Dict[str, Any], grants: List[Dict[str, Any]], poster_grant: Optional[Dict[str, Any]], cover_url: str) -> int:
        media_gallery = item["content_type"] in ("image", "video")
        attachments = []
        for idx, (asset, grant) in enumerate(zip(item["attachments"], grants)):
            entry: Dict[str, Any] = {
                "grant_id": grant["grant_id"],
                "file_type": asset["file_type"],
                "oss_key": grant["oss_key"],
                "file_size": asset["size"],
                "mime_type": asset["mime_type"],
                "file_name": asset["name"],
            }
            if asset.get("duration_sec") is not None:
                entry["duration_sec"] = int(asset["duration_sec"])
            if media_gallery:
                entry["width"] = asset["width"]
                entry["height"] = asset["height"]
                entry["sort_order"] = idx
            attachments.append(entry)
        body: Dict[str, Any] = {
            "title": item["title"],
            "description": item["description"],
            "zone": item["zone"],
            "content_type": item["content_type"],
            "is_public": True,
            "allow_copy": True,
            "tags": item["tags"],
            "attachments": attachments,
        }
        if item["zone"] == "original":
            body["category"] = item["category"]
        if item.get("ip_id"):
            body["ip_id"] = item["ip_id"]
        if item["content_type"] == "video":
            body["poster_grant_id"] = poster_grant["grant_id"]
        if item["content_type"] not in ("image", "video") and cover_url:
            body["cover_image_url"] = cover_url
        status, payload = self.request("POST", "/contents", body, token=token)
        if status != 201:
            raise RuntimeError("publish failed for %s (%d): %s" % (item["key"], status, str(payload)[:400]))
        return int(payload["content"]["id"])

    def content_status(self, token: str, content_id: int) -> Optional[str]:
        status, payload = self.request("GET", "/contents/%d" % content_id, token=token)
        if status != 200:
            return None
        return payload.get("content", {}).get("status")

    def wait_published(self, token: str, content_id: int, timeout: float) -> bool:
        deadline = time.time() + timeout
        while time.time() < deadline:
            if self.content_status(token, content_id) == "published":
                return True
            time.sleep(3.0)
        return False

    def ai_callback_pass(self, content_id: int, key: str, green_uid: str, green_seed: str) -> None:
        content_json, _ = lib.build_ai_callback_payload(content_id, "demoseed-%s" % key)
        checksum = lib.ai_callback_checksum(green_uid, green_seed, content_json)
        status, payload = self.request(
            "POST", "/internal/ai-callback",
            form={"content": content_json, "checksum": checksum},
        )
        if status != 200:
            raise RuntimeError("ai-callback failed for content %d (%d): %s" % (
                content_id, status, str(payload)[:200]))

    # -- lookups -------------------------------------------------------------

    def fetch_public_config(self) -> Dict[str, Any]:
        status, payload = self.request("GET", "/config/public")
        if status != 200:
            raise RuntimeError("GET /config/public failed: %d" % status)
        return payload

    def fetch_approved_ips(self, token: str) -> List[Dict[str, Any]]:
        status, payload = self.request("GET", "/ips?limit=100&page=1", token=token)
        if status != 200:
            raise RuntimeError("GET /ips failed: %d" % status)
        rows = payload.get("ips") or payload.get("items") or []
        approved = [row for row in rows if str(row.get("status", "approved")) == "approved"]
        if not approved:
            raise RuntimeError("no approved IPs visible via GET /ips")
        return approved

    def find_series_by_title(self, token: str, zone: str, title: str) -> Optional[int]:
        status, payload = self.request("GET", "/series?zone=" + zone, token=token)
        if status != 200:
            return None
        for row in payload.get("items") or []:
            if row.get("title") == title:
                return int(row["id"])
        return None

    def create_series(self, token: str, spec: Dict[str, Any]) -> int:
        existing = self.find_series_by_title(token, spec["zone"], spec["title"])
        if existing:
            return existing
        status, payload = self.request("POST", "/series", {
            "title": spec["title"], "description": spec["description"], "zone": spec["zone"],
        }, token=token)
        if status != 201:
            raise RuntimeError("create series %s failed (%d): %s" % (spec["key"], status, str(payload)[:300]))
        return int(payload["series"]["id"])

    def add_series_item(self, token: str, series_id: int, content_id: int) -> None:
        status, payload = self.request(
            "POST", "/series/%d/items" % series_id, {"content_item_id": content_id}, token=token)
        if status == 201:
            return
        # duplicate member (resume path) is success
        if status == 409 or "duplicate" in str(payload).lower():
            return
        raise RuntimeError("add series item content=%d failed (%d): %s" % (content_id, status, str(payload)[:300]))


# ---------------------------------------------------------------- executor

def reserve_or_wait(api: Api, account: str, units: int = 1) -> None:
    """Reserve upload-quota units, sleeping to the next calendar hour if full.

    Grants are bound to the publishing author (Consume checks the user id),
    so an in-flight item cannot migrate to the peer account; waiting for the
    window boundary is the only correct backpressure.
    """
    while not api.quota_reserve(account, units):
        window = api.quota_window()
        wait = (window + 1) * 3600 - int(time.time()) + 2
        log("upload quota window full for %s; sleeping %ds to next window" % (account, wait))
        time.sleep(max(wait, 5))


def upload_asset(api: Api, token: str, account: str, asset: Dict[str, Any]) -> Dict[str, Any]:
    """presign + PUT one asset, honoring the client-side hourly ledger."""
    reserve_or_wait(api, account, 1)
    grant = api.presign(token, asset)
    api.put_object(grant["upload_url"], asset)
    return grant


def execute(api: Api, items: List[Dict[str, Any]], series_specs: List[Dict[str, Any]],
            checkpoint: Checkpoint, green_uid: str, green_seed: str,
            publish_timeout: float) -> None:
    public_config = api.fetch_public_config()
    oss_domain = str(public_config.get("oss_domain", "")).rstrip("/")
    ip_pool: List[Dict[str, Any]] = []

    def next_ip_id(item: Dict[str, Any]) -> Optional[int]:
        nonlocal ip_pool
        if not ip_pool:
            ip_pool = api.fetch_approved_ips(api.login(items[0]["account"]))
        hint = item.get("ip_hint") or ""
        for row in ip_pool:
            if hint and hint in str(row.get("name", "")):
                return int(row["id"])
        idx = items.index(item)
        return int(ip_pool[idx % len(ip_pool)]["id"])

    breaker = lib.CircuitBreaker(max_consecutive=4, min_samples=10, max_error_ratio=0.15)
    stats = {"done": 0, "skip": 0, "fail": 0}

    for item in items:
        state = checkpoint.get(item["key"])
        if state.done:
            stats["skip"] += 1
            continue
        account = item["account"]
        try:
            token = api.login(account)

            # 0. idempotency anchor (crash between publish and checkpoint)
            if state.content_id is None:
                found = api.find_by_key(token, item["key"])
                if found:
                    state.content_id = found
                    checkpoint.save(state)
                    log("  %s: adopted existing content %d (ds: tag anchor)" % (item["key"], found))

            # 1. fanwork IP binding
            if item["zone"] == "fanwork" and not item.get("ip_id"):
                ip_id = next_ip_id(item)
                if ip_id:
                    item["ip_id"] = ip_id

            # 2. uploads + publish
            if state.content_id is None:
                grants = [upload_asset(api, token, account, asset) for asset in item["attachments"]]
                poster_grant = None
                if item.get("poster"):
                    poster_grant = upload_asset(api, token, account, item["poster"])
                cover_url = ""
                if item.get("cover"):
                    cover_grant = upload_asset(api, token, account, item["cover"])
                    cover_url = "%s/%s" % (oss_domain, cover_grant["oss_key"]) if oss_domain else ""
                    if not cover_url:
                        log("  %s: oss_domain empty; publishing without cover URL" % item["key"])
                reserve_or_wait(api, account, 1)
                state.content_id = api.publish(token, item, grants, poster_grant, cover_url)
                checkpoint.save(state)
                log("  %s: published as content %d (%s/%s)" % (
                    item["key"], state.content_id, item["zone"], item["content_type"]))

            # 3. moderation: Green pass -> published; pending -> ai-callback bypass
            if not state.published:
                if api.wait_published(token, state.content_id, publish_timeout):
                    state.published = True
                elif green_uid and green_seed:
                    api.ai_callback_pass(state.content_id, item["key"], green_uid, green_seed)
                    if api.wait_published(token, state.content_id, 30.0):
                        state.published = True
                        log("  %s: published via ai-callback bypass" % item["key"])
                if not state.published:
                    raise RuntimeError("content %d still not published after wait+bypass" % state.content_id)
                checkpoint.save(state)

            # 4. series membership (no upload quota consumed)
            if item.get("series") and not state.series_done:
                spec = next(s for s in series_specs if s["key"] == item["series"])
                owner_token = api.login(spec["account"])
                series_id = api.create_series(owner_token, spec)
                api.add_series_item(owner_token, series_id, state.content_id)
                state.series_done = True
                checkpoint.save(state)
                log("  %s: mounted into series %s (#%d)" % (item["key"], spec["key"], series_id))
            elif not item.get("series"):
                state.series_done = True  # nothing to mount
                checkpoint.save(state)

            state.error = None
            checkpoint.save(state)
            stats["done"] += 1
            breaker.record(True)
        except Exception as err:  # noqa: BLE001 - record and continue
            state.error = str(err)[:500]
            checkpoint.save(state)
            stats["fail"] += 1
            breaker.record(False)
            log("  FAIL %s: %s" % (item["key"], state.error))
            if breaker.tripped:
                log("CIRCUIT BREAKER TRIPPED: %s -- stopping run" % breaker.reason)
                break
    log("execute summary: %s breaker=%s" % (stats, breaker.reason))


# ---------------------------------------------------------------- dry run

def dry_run(library: Dict[str, List[Dict[str, Any]]], items: List[Dict[str, Any]],
            series_specs: List[Dict[str, Any]], config_values: Dict[str, Any],
            workdir: str, ffmpeg_bin: Optional[str]) -> bool:
    ok = True
    print("=" * 72)
    print("DRY RUN -- offline full-matrix rehearsal (no network calls)")
    print("=" * 72)
    print("workdir: %s" % workdir)
    print("audio path: %s" % ("ffmpeg MP3 (%s)" % ffmpeg_bin if ffmpeg_bin else "pure-Python WAV (ffmpeg absent)"))
    print("repo config sniff: ip_categories=%s upload_per_hour=%d" % (
        ",".join(config_values["ip_categories"]), config_values["upload_per_hour"]))

    print("\n-- asset inventory (all validated on build) --")
    for kind in sorted(library):
        entries = library[kind]
        detail = []
        for asset in entries:
            extra = ""
            if asset.get("width"):
                extra = " %dx%d" % (asset["width"], asset["height"])
            if asset.get("duration_sec") is not None:
                extra += " %ds" % asset["duration_sec"]
            detail.append("%s(%dKB%s)" % (asset["name"], asset["size"] // 1024, extra))
        print("  %-12s %d -> %s" % (kind, len(entries), ", ".join(detail)))

    print("\n-- plan: %d items --" % len(items))
    matrix: Dict[Tuple[str, str], int] = {}
    for item in items:
        matrix[(item["zone"], item["content_type"])] = matrix.get((item["zone"], item["content_type"]), 0) + 1
        cover_note = ""
        if item.get("cover"):
            cover_note = " cover=%s" % item["cover"]["name"]
        poster_note = " poster=%s" % item["poster"]["name"] if item.get("poster") else ""
        attach_note = " attach=[%s]" % ",".join(a["name"] for a in item["attachments"]) if item["attachments"] else ""
        series_note = " series=%s" % item["series"] if item.get("series") else ""
        ip_note = " ip_hint=%s" % item["ip_hint"] if item.get("ip_hint") else ""
        cat_note = " category=%s" % item["category"] if item.get("category") else ""
        print("  %-16s %-11s acct=%-4s quota=%2d%s%s%s%s%s%s" % (
            item["key"], item["content_type"], item["account"], item_quota_cost(item),
            cat_note, ip_note, series_note, attach_note, poster_note, cover_note))
        # per-item contract self-checks (mirror backend gates)
        ctype = item["content_type"]
        attachments = item["attachments"]
        if ctype == "image":
            if not (2 <= len(attachments) <= 9):
                print("    !! image gallery size %d violates 2..9" % len(attachments)); ok = False
            if any(a["file_type"] != "image" for a in attachments):
                print("    !! image content carries non-image attachment"); ok = False
        if ctype == "video":
            if not (1 <= len(attachments) <= 3):
                print("    !! video count %d violates 1..3" % len(attachments)); ok = False
            if not item.get("poster"):
                print("    !! video missing poster grant"); ok = False
            if any(a["duration_sec"] > VIDEO_MAX_SEC for a in attachments):
                print("    !! video duration exceeds %ds" % VIDEO_MAX_SEC); ok = False
        if ctype == "3d_print" and not any(a["file_type"] == "model3d" for a in attachments):
            print("    !! 3d_print missing model3d attachment (attachment_policy)"); ok = False
        if ctype == "mod" and any(a["file_type"] != "mod" for a in attachments):
            print("    !! mod carries non-mod attachment"); ok = False
        if item["zone"] == "original" and item["category"] not in config_values["ip_categories"]:
            print("    !! category %r not in 11-category allowlist" % item["category"]); ok = False
        if item["zone"] == "fanwork" and not item.get("ip_hint"):
            print("    !! fanwork missing IP/source attribution"); ok = False
        if item["attachments"]:
            fams = {a["file_type"] for a in attachments}
            allowed = ALLOWED_FAMILIES.get(ctype)
            if allowed is None:
                print("    !! unknown content type %r" % ctype); ok = False
            elif not fams.issubset(allowed):
                print("    !! attachment families %s outside %s registry" % (fams, allowed)); ok = False

    print("\n-- matrix coverage (zone/type -> count) --")
    original_types = ["image", "article", "video", "audio", "template", "sheet_music", "3d_print", "other"]
    fanwork_types = ["image", "article", "video", "audio", "mod", "prompt", "sheet_music", "3d_print", "other"]
    for zone, types in (("original", original_types), ("fanwork", fanwork_types)):
        counts = [matrix.get((zone, t), 0) for t in types]
        print("  %-9s %s" % (zone, " ".join("%s=%d" % (t, c) for t, c in zip(types, counts))))
        if any(c < 2 or c > 3 for c in counts):
            print("    !! cell count outside 2..3"); ok = False
    total_types = {t for (z, t) in matrix}
    if total_types != set(original_types) | set(fanwork_types):
        print("    !! full 10-type coverage broken"); ok = False

    specials = [it for it in items if it.get("special")]
    print("\n-- special forms --")
    for it in specials:
        print("  %-16s %s (%d attachments)" % (it["key"], it["special"], len(it["attachments"])))
    nine = [it for it in items if it["content_type"] == "image" and len(it["attachments"]) == 9]
    three_video = [it for it in items if it["content_type"] == "video" and len(it["attachments"]) == 3]
    if len(nine) != 1:
        print("  !! expected exactly one 9-image gallery, got %d" % len(nine)); ok = False
    if len(three_video) != 1:
        print("  !! expected exactly one 3-video item, got %d" % len(three_video)); ok = False

    print("\n-- series plan (%d) --" % len(series_specs))
    for spec in series_specs:
        members = [it for it in items if it.get("series") == spec["key"]]
        accounts = {m["account"] for m in members}
        print("  %s zone=%s owner=%s members=%s single-author=%s" % (
            spec["title"], spec["zone"], spec["account"], [m["key"] for m in members], len(accounts) == 1))
        if len(members) != 3 or len(accounts) != 1:
            print("    !! series must have 3 members by one author"); ok = False

    print("\n-- quota simulation (upload units: presign + publish each count) --")
    rows = simulate_quota(items, config_values["upload_per_hour"])
    for row in rows:
        print("  account=%s hour-window=%d units=%d/%d %s" % (
            row["account"], row["window"], row["units"], config_values["upload_per_hour"], row["flag"]))
    if any(r["flag"] != "OK" for r in rows):
        print("  (execute path rotates accounts / waits for the next hour window)")
    per_account: Dict[str, int] = {}
    for item in items:
        per_account[item["account"]] = per_account.get(item["account"], 0) + item_quota_cost(item)
    print("  totals: %s (rotation across %d accounts)" % (
        per_account, len(per_account)))
    if not per_account or min(per_account.values()) == 0:
        print("  !! accounts not rotated"); ok = False

    print("\nDRY RUN RESULT: %s" % ("PASS" if ok else "FAIL"))
    return ok


# attachment families per content type (mirrors backend/config.yaml
# content_registry upload_file_types; the registry is the single source)
ALLOWED_FAMILIES = {
    "image": {"image"},
    "article": set(),
    "video": {"video"},
    "audio": {"audio"},
    "mod": {"mod"},
    "prompt": set(),
    "template": {"text", "document", "model3d"},
    "sheet_music": {"sheet_music"},
    "3d_print": {"model3d", "text"},
    "other": set(),
}


# ---------------------------------------------------------------- main

def main() -> None:
    parser = argparse.ArgumentParser(
        description="demo showcase seeder (#848 A5)",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=__doc__.split("用法：")[-1] if __doc__ else None,
    )
    mode = parser.add_mutually_exclusive_group(required=True)
    mode.add_argument("--dry-run", action="store_true", help="offline rehearsal: assets + plan, no network")
    mode.add_argument("--execute", action="store_true", help="run the real pipeline against --api-base")
    parser.add_argument("--api-base", default=DEFAULT_API_BASE, help="API base (default %(default)s)")
    parser.add_argument("--account", action="append", default=None,
                        help="fixture author id (repeatable; default a01 a02 corpus 种子作者)")
    parser.add_argument("--workdir", default=DEFAULT_WORKDIR, help="checkpoint + generated assets dir")
    parser.add_argument("--limit", type=int, default=None, help="execute only first N planned items")
    parser.add_argument("--only", default=None, help="execute a single item key (e.g. org-image-03)")
    parser.add_argument("--publish-timeout", type=float, default=PUBLISH_WAIT_TIMEOUT_DEFAULT,
                        help="seconds to wait for Green moderation before the ai-callback bypass")
    parser.add_argument("--no-ffmpeg", action="store_true", help="force pure-Python WAV audio even when ffmpeg exists")
    parser.add_argument("--rps", type=float, default=2.0, help="request pacing (default 2)")
    args = parser.parse_args()

    accounts = args.account or ["a01", "a02"]
    if len(accounts) < 2:
        raise SystemExit("at least two --account values are required for rotation")

    config_values = load_repo_config_values()
    library = assets.ensure_demo_assets(args.workdir, use_ffmpeg=not args.no_ffmpeg)
    items, series_specs = build_plan(library, accounts)

    if args.dry_run:
        ok = dry_run(library, items, series_specs, config_values, args.workdir,
                     assets.detect_ffmpeg() if not args.no_ffmpeg else None)
        sys.exit(0 if ok else 1)

    # --execute
    work = items
    if args.only:
        work = [it for it in items if it["key"] == args.only]
        if not work:
            raise SystemExit("no plan item matches --only %s" % args.only)
    if args.limit is not None:
        work = work[: args.limit]
    if not lib.FIXTURE_PASSWORD:
        raise SystemExit(
            "CORPUS_FIXTURE_PASSWORD is not set -- fixture credential lives in the "
            "demo server env (涉密协议), never in this repo")
    green_uid = os.environ.get("GREEN_UID", "")
    green_seed = os.environ.get("GREEN_SEED", "")
    if not (green_uid and green_seed):
        log("GREEN_UID/GREEN_SEED not set: pending items will fail instead of using the ai-callback bypass")

    checkpoint = Checkpoint(args.workdir)
    api = Api(args.api_base, rps=args.rps)
    log("execute: %d items against %s (accounts: %s)" % (len(work), args.api_base, ",".join(accounts)))
    execute(api, work, series_specs, checkpoint, green_uid, green_seed, args.publish_timeout)


if __name__ == "__main__":
    main()
