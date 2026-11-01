#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""demo_document_seed -- docx/csv/pdf 文档位补造数 (#928, 2026-11-01).

D 批 retro 在案缺口（2026-10-11）：A5 造数集在演示站缺文档位真实数据——
docx/csv 仅挂于 2 条模板附位（无独立文档位条目），pdf 为零（text 族附件
全部 text/plain，永不产出 text_pdf slide）。D5 文档位（可预览媒体链
document slide）与 D6 pdf 放大态（text_pdf 可放大原生 embed）的线上视觉
条验当时由 mocked e2e 覆盖，真实数据面未闭环。本脚本沿 A5 先例
（demo_showcase_seed 真实发布管线 + 幂等锚）补齐。

矩阵（三类 × 可挂载内容类型全覆盖，registry 单一事实源）：
  docx (document 族) -> template（原创——document 族唯一宿主）
  csv  (document 族) -> template
  pdf  (text 族 text_pdf，持久化 MIME 精确 application/pdf)
                     -> template（原创）/ 3d_print（原创 + 二创，全宿主全两区）

  key                   类型/区      附件族                      覆盖面
  doc-tpl-docx-01       template 原  document(docx) + 封面回填   D5 文档位·docx
  doc-tpl-csv-01        template 原  document(csv) + 封面回填    D5 文档位·csv
  doc-tpl-pdf-01        template 原  model3d + text(pdf) + 封面  D6 放大态·3D↔pdf 混排导航
  doc-tpl-suite-01      template 原  document×2 + text(pdf)+封面 D5+D6 一站式三件套
  doc-3dp-pdf-01        3d_print 原  model3d + text(pdf) + 封面  D6·3d_print 宿主
  doc-3dp-pdf-02        3d_print 二  model3d + text(pdf) + 封面  D6·二创区（IP 绑定）

凭据纪律：与 A5 相同——CORPUS_FIXTURE_PASSWORD / GREEN_UID / GREEN_SEED
只从 env 读取（演示服务器 env 注入），绝不硬编码/写文件/打印日志。

用法：
    # 离线演练（不出网：生成+校验全部资产、打印矩阵计划与限流模拟）
    python3 scripts/corpus/demo_document_seed.py --dry-run

    # 真跑（默认 localhost；演示站经 --api-base 指定 api 域）
    CORPUS_FIXTURE_PASSWORD=... python3 scripts/corpus/demo_document_seed.py \
        --execute --api-base https://<demo-api-host>/api/v1

    # 只读核验（造数结果 + 幂等预检：逐键 ds: 锚查库，报告已存在内容）
    CORPUS_FIXTURE_PASSWORD=... python3 scripts/corpus/demo_document_seed.py \
        --verify --api-base https://<demo-api-host>/api/v1

    # 粒度控制
    ... --only doc-tpl-pdf-01     # 只跑单条（断点续跑友好）
    ... --limit 3                 # 只跑前 N 条

幂等：checkpoint JSONL（复用 A5 Checkpoint 状态机）+ 内容 tag `ds:<key>`
DB 锚（键位 doc-* 前缀，与 A5 37 条 org-*/fan-* 键零碰撞，dry-run 内置
碰撞守卫断言）；重跑对已完成条目全 skip，崩溃恢复时经 ds: 锚收养既有行，
不重复发布。

依赖：Python 3.9+ 标准库（复用 demo_showcase_assets / demo_showcase_seed /
corpus_lib，不引第三方包）。
"""
from __future__ import annotations

import argparse
import os
import sys
import time
from typing import Any, Dict, List, Optional, Tuple

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import corpus_lib as lib  # noqa: E402
import demo_showcase_assets as assets  # noqa: E402
import demo_showcase_seed as a5  # noqa: E402  (Api / Checkpoint / execute / plan helpers)

REPO_ROOT = assets.REPO_ROOT

DEFAULT_API_BASE = a5.DEFAULT_API_BASE
DEFAULT_WORKDIR = os.environ.get(
    "DEMO_DOC_SEED_WORKDIR", os.path.join(REPO_ROOT, "artifacts", "demo-doc-seed")
)
IDEMPOTENCY_TAG_PREFIX = a5.IDEMPOTENCY_TAG_PREFIX  # "ds:"
DEMO_TAG = a5.DEMO_TAG                          # "演示造数"
KEY_PREFIX = "doc-"                             # #928 键位命名空间

ZONE_LABEL = a5.ZONE_LABEL
TYPE_LABEL = a5.TYPE_LABEL

DOCX_BODY = (
    "本文档由 #928 演示造数脚本经标准库构造、走真实发布管线上传，用于模板类"
    "文档位（可预览媒体链 document slide）与下载链路的线上验收。"
)
PDF_BODY = [
    "OmniCraft demo datasheet (seed corpus #928).",
    "Purpose: document slot preview (D5) and pdf enlarge state (D6).",
    "Uploaded through the real presign + publish pipeline.",
    "Persisted MIME is exactly application/pdf (text family, text_pdf slide).",
]

# D5/D6 验收面映射（dry-run 打印 + verify 口径注释用）
FEATURE_BY_KEY = {
    "doc-tpl-docx-01": "D5 文档位·docx",
    "doc-tpl-csv-01": "D5 文档位·csv",
    "doc-tpl-pdf-01": "D6 放大态·3D+pdf 混排",
    "doc-tpl-suite-01": "D5+D6 三件套混排",
    "doc-3dp-pdf-01": "D6 放大态·3d_print 宿主",
    "doc-3dp-pdf-02": "D6 放大态·二创区",
}


def log(msg: str) -> None:
    print("[%s] %s" % (time.strftime("%H:%M:%S"), msg), flush=True)


# ---------------------------------------------------------------- doc assets

def ensure_document_assets(workdir: str) -> Dict[str, Dict[str, Any]]:
    """Synthesize + validate the #928 document assets (docx/csv/pdf).

    Reuses the A5 asset builders; pdf comes from assets.generate_pdf (text
    family, persisted MIME exactly application/pdf). Idempotent: re-running
    rewrites the same deterministic bytes.
    """
    assets_dir = os.path.join(workdir, "assets")
    os.makedirs(assets_dir, exist_ok=True)
    library: Dict[str, Dict[str, Any]] = {
        "docx": assets.generate_docx(
            os.path.join(assets_dir, "demo-doc-manual.docx"),
            "万象工坊演示文档：模板手册（#928）",
            [
                DOCX_BODY,
                "一、文档位：本附件在详情浮层可预览媒体链中占据 document 轮播位。",
                "二、放大态：pdf 类附件（text 族）可经右上放大钮进入全屏 embed。",
                "三、下载链路：右栏附件行提供带签名的下载入口。",
            ],
        ),
        "csv": assets.generate_csv(os.path.join(assets_dir, "demo-doc-sheet.csv")),
        "pdf": assets.generate_pdf(
            os.path.join(assets_dir, "demo-doc-datasheet.pdf"),
            "OmniCraft Demo Datasheet (#928)",
            PDF_BODY,
            pages=2,
        ),
    }
    failures: List[str] = []
    for kind, asset in library.items():
        for issue in assets.validate_asset(asset):
            failures.append("%s/%s: %s" % (kind, asset["name"], issue))
    if failures:
        raise RuntimeError("document asset validation failed:\n  " + "\n  ".join(failures))
    return library


# ---------------------------------------------------------------- plan model

def build_doc_plan(
    media_library: Dict[str, List[Dict[str, Any]]],
    doc_library: Dict[str, Dict[str, Any]],
    accounts: List[str],
) -> List[Dict[str, Any]]:
    """Deterministic #928 matrix (6 items; see module docstring table)."""
    img = media_library["image"]
    stl = media_library["model3d"][0]
    items: List[Dict[str, Any]] = []

    def add(
        key: str, zone: str, ctype: str, *, account: str,
        attachments: List[Dict[str, Any]], cover: Dict[str, Any],
        category: Optional[str] = None, ip_hint: Optional[str] = None,
        title: str = "", description: str = "",
    ) -> Dict[str, Any]:
        zone_label = ZONE_LABEL[zone]
        type_label = TYPE_LABEL[ctype]
        item = {
            "key": key, "zone": zone, "content_type": ctype,
            "title": title or "演示·文档·%s·%s" % (zone_label, type_label),
            "description": description or (
                "%s 演示条目（#928 文档位补造数，%s）。" % (type_label, zone_label)
            ),
            "account": account, "attachments": attachments, "poster": None,
            "cover": cover, "category": category, "special": None,
            "series": None, "ip_hint": ip_hint,
            "tags": [DEMO_TAG, IDEMPOTENCY_TAG_PREFIX + key],
        }
        items.append(item)
        return item

    add(
        KEY_PREFIX + "tpl-docx-01", "original", "template",
        account=accounts[0], attachments=[doc_library["docx"]], cover=img[2],
        category="other", title="演示·文档·模板·docx 手册",
        description="模板附 docx 文档：详情浮层可预览媒体链的 document 轮播位（D5 文档位）演示条目。",
    )
    add(
        KEY_PREFIX + "tpl-csv-01", "original", "template",
        account=accounts[1 % len(accounts)], attachments=[doc_library["csv"]], cover=img[4],
        category="other", title="演示·文档·模板·csv 数据表",
        description="模板附 csv 数据表：可预览媒体链 document 位（表格网格渲染，D5 文档位）演示条目。",
    )
    add(
        KEY_PREFIX + "tpl-pdf-01", "original", "template",
        account=accounts[0], attachments=[stl, doc_library["pdf"]], cover=img[6],
        category="other", title="演示·文档·模板·pdf 图纸（3D+PDF 混排）",
        description="模板附 3D 模型与 pdf 图纸：可放大序列含 model3d 与 text_pdf 两类，验证 D6 放大态跨项导航。",
    )
    add(
        KEY_PREFIX + "tpl-suite-01", "original", "template",
        account=accounts[1 % len(accounts)],
        attachments=[doc_library["docx"], doc_library["csv"], doc_library["pdf"]],
        cover=img[8], category="other", title="演示·文档·模板·三件套（docx+csv+pdf）",
        description="一站式混排：docx 与 csv 占据 document 文档位（D5），pdf 占据 text_pdf 位并可放大（D6）。",
    )
    add(
        KEY_PREFIX + "3dp-pdf-01", "original", "3d_print",
        account=accounts[0], attachments=[stl, doc_library["pdf"]], cover=img[9],
        category="other", title="演示·文档·3D打印·pdf 说明书",
        description="3D 打印附 pdf 打印说明书：3d_print 宿主上的 text_pdf 放大态（D6）演示条目。",
    )
    add(
        KEY_PREFIX + "3dp-pdf-02", "fanwork", "3d_print",
        account=accounts[1 % len(accounts)], attachments=[stl, doc_library["pdf"]],
        cover=img[11], ip_hint="原神", title="演示·文档·二创·3D打印·pdf 设定集",
        description="二创区 3D 打印附 pdf 设定集：二创区 text_pdf 放大态（D6）与 IP 来源关联演示条目。",
    )
    return items


def doc_kinds(item: Dict[str, Any]) -> List[str]:
    """Attachment extension kinds in plan order (docx/csv/pdf presence audit)."""
    return [a.get("extension") for a in item["attachments"]]


# ---------------------------------------------------------------- dry run

def dry_run(
    media_library: Dict[str, List[Dict[str, Any]]],
    doc_library: Dict[str, Dict[str, Any]],
    items: List[Dict[str, Any]],
    config_values: Dict[str, Any],
    workdir: str,
    accounts: List[str],
) -> bool:
    ok = True
    print("=" * 72)
    print("DRY RUN -- offline #928 document-slot matrix rehearsal (no network)")
    print("=" * 72)
    print("workdir: %s" % workdir)

    print("\n-- document assets (validated on build) --")
    for kind in sorted(doc_library):
        asset = doc_library[kind]
        print("  %-5s %s -> file_type=%s mime=%s (%d bytes)" % (
            kind, asset["name"], asset["file_type"], asset["mime_type"], asset["size"]))
    if doc_library["pdf"]["mime_type"] != "application/pdf":
        print("  !! pdf mime must be exactly application/pdf (persisted-mime contract)")
        ok = False
    for kind, asset in doc_library.items():
        for issue in assets.validate_asset(asset):
            print("  !! %s/%s: %s" % (kind, asset["name"], issue))
            ok = False

    print("\n-- plan: %d items --" % len(items))
    for item in items:
        kinds = ",".join(k or "?" for k in doc_kinds(item))
        print("  %-16s %-8s/%-9s acct=%-4s quota=%2d cover=%s attach=[%s] %s%s" % (
            item["key"], item["zone"], item["content_type"], item["account"],
            a5.item_quota_cost(item), item["cover"]["name"], kinds,
            "category=%s " % item["category"] if item["category"] else "",
            "ip_hint=%s" % item["ip_hint"] if item.get("ip_hint") else "",
        ))
        print("      feature: %s | title: %s" % (
            FEATURE_BY_KEY.get(item["key"], "?"), item["title"]))

    # -- per-item contract self-checks (mirror backend gates) ---------------
    print("\n-- contract self-checks --")
    for item in items:
        key = item["key"]
        attachments = item["attachments"]
        fams = {a["file_type"] for a in attachments}
        allowed = a5.ALLOWED_FAMILIES.get(item["content_type"])
        if allowed is None:
            print("  !! %s: unknown content type %r" % (key, item["content_type"])); ok = False
        elif not fams.issubset(allowed):
            print("  !! %s: attachment families %s outside %s registry" % (
                key, fams, allowed)); ok = False
        if item["zone"] == "original" and item["category"] not in config_values["ip_categories"]:
            print("  !! %s: category %r not in 11-category allowlist" % (key, item["category"])); ok = False
        if item["zone"] == "fanwork" and not item.get("ip_hint"):
            print("  !! %s: fanwork missing IP/source attribution" % key); ok = False
        if item["content_type"] == "3d_print" and not any(
                a["file_type"] == "model3d" for a in attachments):
            print("  !! %s: 3d_print missing model3d attachment (attachment_policy)" % key); ok = False
        if not item.get("cover"):
            print("  !! %s: missing cover asset (cover backfill contract)" % key); ok = False
        if not str(item["key"]).startswith(KEY_PREFIX):
            print("  !! %s: key outside %s namespace" % (key, KEY_PREFIX)); ok = False
        if IDEMPOTENCY_TAG_PREFIX + key not in item["tags"]:
            print("  !! %s: missing ds: idempotency anchor tag" % key); ok = False

    # -- matrix coverage -----------------------------------------------------
    print("\n-- matrix coverage (docx/csv/pdf x hosting content types) --")
    hosting: Dict[Tuple[str, str, str], int] = {}
    for item in items:
        for kind in (k for k in doc_kinds(item) if k in (".docx", ".csv", ".pdf")):
            hosting[(item["zone"], item["content_type"], kind)] = (
                hosting.get((item["zone"], item["content_type"], kind), 0) + 1)
    for (zone, ctype, kind), count in sorted(hosting.items()):
        print("  %-9s %-9s %-6s x%d" % (zone, ctype, kind, count))
    for kind, hosts in (
        (".docx", {("original", "template")}),
        (".csv", {("original", "template")}),
        (".pdf", {("original", "template"), ("original", "3d_print"), ("fanwork", "3d_print")}),
    ):
        got = {(z, c) for (z, c, k) in hosting if k == kind}
        missing = hosts - got
        if missing:
            print("  !! %s hosting coverage incomplete, missing %s" % (kind, sorted(missing))); ok = False
        else:
            print("  %s hosting coverage complete: %s" % (kind, sorted(hosts)))

    # -- key namespace collision guard (A5 keys are org-*/fan-*) ------------
    a5_items, _ = a5.build_plan(media_library, accounts)
    a5_keys = {it["key"] for it in a5_items}
    overlap = a5_keys & {it["key"] for it in items}
    if overlap:
        print("  !! key collision with A5 plan: %s" % sorted(overlap)); ok = False
    else:
        print("  key namespace clean: %d #928 keys disjoint from %d A5 keys" % (
            len(items), len(a5_keys)))

    print("\n-- quota simulation (upload units: presign + publish each count) --")
    rows = a5.simulate_quota(items, config_values["upload_per_hour"])
    for row in rows:
        print("  account=%s hour-window=%d units=%d/%d %s" % (
            row["account"], row["window"], row["units"],
            config_values["upload_per_hour"], row["flag"]))
    if any(r["flag"] != "OK" for r in rows):
        print("  !! quota overflow in projection"); ok = False

    print("\nDRY RUN RESULT: %s" % ("PASS" if ok else "FAIL"))
    return ok


# ---------------------------------------------------------------- verify

def verify(api: "a5.Api", items: List[Dict[str, Any]]) -> bool:
    """Read-only post-seed check: ds:<key> anchor lookup + attachment audit.

    Doubles as the idempotency pre-check -- every key reported EXISTING here
    is adopted and skipped by --execute on re-run.
    """
    ok = True
    print("=" * 72)
    print("VERIFY -- ds: anchor lookup + attachment audit (read-only)")
    print("=" * 72)
    viewer_token = api.login(items[0]["account"])
    for item in items:
        key = item["key"]
        found = api.find_by_key(viewer_token, key)
        if not found:
            print("  %-16s MISSING (execute will publish)" % key)
            ok = False
            continue
        status, payload = api.request("GET", "/contents/%d" % found, token=viewer_token)
        content = (payload.get("content") or {}) if status == 200 else {}
        print("  %-16s content=%d status=%-9s title=%s" % (
            key, found, content.get("status", "?"), content.get("title", "?")))
        for att in (payload.get("attachments") or []):
            signed = "signed" if att.get("oss_url") else "UNSIGNED"
            print("      %-9s %-12s %s scan=%s %s" % (
                att.get("file_type"), att.get("mime_type"), signed,
                att.get("scan_status"), att.get("original_file_name")))
        if content.get("status") != "published":
            print("      !! not published")
            ok = False
    print("VERIFY RESULT: %s" % ("PASS" if ok else "INCOMPLETE"))
    return ok


# ---------------------------------------------------------------- main

def main() -> None:
    parser = argparse.ArgumentParser(
        description="demo document-slot seeder (#928: docx/csv/pdf)",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=__doc__.split("用法：")[-1] if __doc__ else None,
    )
    mode = parser.add_mutually_exclusive_group(required=True)
    mode.add_argument("--dry-run", action="store_true", help="offline rehearsal: assets + plan, no network")
    mode.add_argument("--execute", action="store_true", help="run the real pipeline against --api-base")
    mode.add_argument("--verify", action="store_true", help="read-only anchor lookup + attachment audit")
    parser.add_argument("--api-base", default=DEFAULT_API_BASE, help="API base (default %(default)s)")
    parser.add_argument("--account", action="append", default=None,
                        help="fixture author id (repeatable; default a01 a02 corpus 种子作者)")
    parser.add_argument("--workdir", default=DEFAULT_WORKDIR, help="checkpoint + generated assets dir")
    parser.add_argument("--limit", type=int, default=None, help="execute only first N planned items")
    parser.add_argument("--only", default=None, help="execute a single item key (e.g. doc-tpl-pdf-01)")
    parser.add_argument("--publish-timeout", type=float, default=a5.PUBLISH_WAIT_TIMEOUT_DEFAULT,
                        help="seconds to wait for Green moderation before the ai-callback bypass")
    parser.add_argument("--no-ffmpeg", action="store_true", help="force pure-Python WAV audio in base library build")
    parser.add_argument("--rps", type=float, default=2.0, help="request pacing (default 2)")
    args = parser.parse_args()

    accounts = args.account or ["a01", "a02"]
    if len(accounts) < 2:
        raise SystemExit("at least two --account values are required for rotation")

    config_values = a5.load_repo_config_values()
    media_library = assets.ensure_demo_assets(args.workdir, use_ffmpeg=not args.no_ffmpeg)
    doc_library = ensure_document_assets(args.workdir)
    items = build_doc_plan(media_library, doc_library, accounts)

    if args.dry_run:
        ok = dry_run(media_library, doc_library, items, config_values, args.workdir, accounts)
        sys.exit(0 if ok else 1)

    if not lib.FIXTURE_PASSWORD:
        raise SystemExit(
            "CORPUS_FIXTURE_PASSWORD is not set -- fixture credential lives in the "
            "demo server env (涉密协议), never in this repo")

    if args.verify:
        api = a5.Api(args.api_base, rps=args.rps)
        sys.exit(0 if verify(api, items) else 1)

    # --execute
    work = items
    if args.only:
        work = [it for it in items if it["key"] == args.only]
        if not work:
            raise SystemExit("no plan item matches --only %s" % args.only)
    if args.limit is not None:
        work = work[: args.limit]
    green_uid = os.environ.get("GREEN_UID", "")
    green_seed = os.environ.get("GREEN_SEED", "")
    if not (green_uid and green_seed):
        log("GREEN_UID/GREEN_SEED not set: pending items will fail instead of using the ai-callback bypass")

    checkpoint = a5.Checkpoint(args.workdir)
    api = a5.Api(args.api_base, rps=args.rps)
    log("execute: %d items against %s (accounts: %s)" % (
        len(work), args.api_base, ",".join(accounts)))
    a5.execute(api, work, [], checkpoint, green_uid, green_seed, args.publish_timeout)


if __name__ == "__main__":
    main()
