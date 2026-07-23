#!/usr/bin/env python3
"""Create safe, media-rich local data for OmniCraft UI and feature testing.

This seeder deliberately does not crawl the web and never calls OSS or content
moderation services.  It creates fictional content plus local SVG cover assets
under ``frontend/public/seed-media`` and writes only to the local Docker
PostgreSQL service.

Commands:
  python scripts/seed_local_rich_data.py seed
  python scripts/seed_local_rich_data.py verify
  python scripts/seed_local_rich_data.py cleanup
  python scripts/seed_local_rich_data.py reset

All database records are owned by users carrying ``SEED_NAMESPACE`` in
``support_info``.  ``cleanup`` removes only those users and their dependent
records; it does not touch manually-created local data.
"""

from __future__ import annotations

import argparse
import json
import os
import random
import shutil
import subprocess
import sys
from dataclasses import dataclass
from datetime import datetime, timedelta, timezone
from pathlib import Path
from typing import Iterable, Sequence


SEED_NAMESPACE = "local-rich-ui-v1"
SEED_EMAIL_SUFFIX = "@seed.omnicraft.local"
SEED_IP_SLUG_PREFIX = "seed-ui-rich-"
SCRIPT_DIR = Path(__file__).resolve().parent
REPO_ROOT = SCRIPT_DIR.parent
MEDIA_ROOT = REPO_ROOT / "frontend" / "public" / "seed-media"
MEDIA_MARKER = MEDIA_ROOT / ".omnicraft-local-rich-seed"
RNG = random.Random(20260723)


@dataclass(frozen=True)
class ContentPlan:
    key: str
    title: str
    description: str
    author_email: str
    zone: str
    ip_slug: str | None
    source_key: str | None
    category: str
    content_type: str
    cover_url: str
    status: str
    is_public: bool
    allow_copy: bool
    view_count: int
    download_count: int
    hot_score: float
    rating_score: float
    created_at: datetime
    tags: tuple[str, ...]


def sql(value: object | None) -> str:
    """Return a PostgreSQL literal.  Input is generated locally, never user SQL."""
    if value is None:
        return "NULL"
    if isinstance(value, bool):
        return "TRUE" if value else "FALSE"
    if isinstance(value, (int, float)):
        return str(value)
    return "'" + str(value).replace("'", "''") + "'"


def rows_sql(rows: Iterable[Sequence[object | None]]) -> str:
    return ",\n".join("(" + ", ".join(sql(value) for value in row) + ")" for row in rows)


def compose_command() -> list[str]:
    compose = os.environ.get("SEED_DOCKER_COMPOSE", "docker compose").split()
    service = os.environ.get("SEED_POSTGRES_SERVICE", "postgres")
    db_name = os.environ.get("SEED_DB_NAME", "omnicraft")
    db_user = os.environ.get("SEED_DB_USER", "omnicraft")
    return [*compose, "exec", "-T", service, "psql", "-v", "ON_ERROR_STOP=1", "-U", db_user, "-d", db_name]


def execute_sql(statement: str, *, capture: bool = False, dry_run: bool = False) -> str:
    if dry_run:
        print("[dry-run] PostgreSQL would receive a generated transaction.")
        return ""
    result = subprocess.run(
        compose_command(),
        cwd=REPO_ROOT,
        input=statement,
        text=True,
        capture_output=capture,
        check=False,
    )
    if result.returncode != 0:
        stderr = (result.stderr or "").strip() or "unknown psql error"
        raise RuntimeError(f"PostgreSQL command failed: {stderr}")
    return result.stdout


def ensure_media(*, dry_run: bool = False) -> None:
    if dry_run:
        print(f"[dry-run] Would create local covers in {MEDIA_ROOT}")
        return

    MEDIA_ROOT.mkdir(parents=True, exist_ok=True)
    MEDIA_MARKER.write_text(f"namespace={SEED_NAMESPACE}\n", encoding="utf-8")
    for folder in ("covers", "ips", "avatars"):
        (MEDIA_ROOT / folder).mkdir(exist_ok=True)

    for index in range(1, 13):
        hue = (index * 31) % 360
        accent = (hue + 68) % 360
        svg = f'''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 900 1200" role="img" aria-label="OmniCraft seed cover {index}">
  <defs>
    <linearGradient id="g" x1="0" y1="0" x2="1" y2="1">
      <stop offset="0" stop-color="hsl({hue} 72% 31%)"/>
      <stop offset="1" stop-color="hsl({accent} 76% 58%)"/>
    </linearGradient>
    <filter id="blur"><feGaussianBlur stdDeviation="24"/></filter>
  </defs>
  <rect width="900" height="1200" fill="url(#g)"/>
  <circle cx="690" cy="230" r="215" fill="hsl({accent} 86% 76% / .48)" filter="url(#blur)"/>
  <path d="M-90 910 C190 675 460 1150 970 740 L970 1260 L-90 1260Z" fill="hsl({hue} 80% 12% / .36)"/>
  <rect x="68" y="796" width="520" height="6" rx="3" fill="white" opacity=".76"/>
  <rect x="68" y="828" width="382" height="6" rx="3" fill="white" opacity=".5"/>
  <text x="68" y="188" fill="white" font-size="42" font-family="Arial, sans-serif" letter-spacing="9" opacity=".84">OMNICRAFT</text>
  <text x="68" y="740" fill="white" font-size="88" font-weight="700" font-family="Arial, sans-serif">LOCAL STORY</text>
  <text x="70" y="888" fill="white" font-size="28" font-family="Arial, sans-serif" opacity=".8">fictional seed media · {index:02d}</text>
</svg>'''
        (MEDIA_ROOT / "covers" / f"cover-{index:02d}.svg").write_text(svg, encoding="utf-8")

    for index in range(1, 17):
        hue = (index * 47) % 360
        svg = f'''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1200 630" role="img" aria-label="Fictional IP cover {index}">
  <rect width="1200" height="630" fill="hsl({hue} 54% 22%)"/>
  <circle cx="920" cy="226" r="232" fill="hsl({(hue + 72) % 360} 76% 59% / .7)"/>
  <path d="M0 520 L360 270 L710 590 L1200 220 L1200 630 L0 630Z" fill="hsl({(hue + 160) % 360} 64% 28%)"/>
  <text x="64" y="90" fill="white" font-size="32" font-family="Arial, sans-serif" letter-spacing="7">FICTIONAL IP</text>
  <text x="64" y="500" fill="white" font-size="72" font-weight="700" font-family="Arial, sans-serif">OMNI ARCHIVE</text>
</svg>'''
        (MEDIA_ROOT / "ips" / f"ip-{index:02d}.svg").write_text(svg, encoding="utf-8")

    for index in range(1, 25):
        hue = (index * 29) % 360
        initials = f"{index:02d}"
        svg = f'''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 160 160" role="img" aria-label="Seed user {index}">
  <rect width="160" height="160" rx="80" fill="hsl({hue} 55% 43%)"/>
  <circle cx="80" cy="62" r="31" fill="hsl({hue} 46% 82%)"/>
  <path d="M30 150 C38 108 58 94 80 94 C102 94 122 108 130 150Z" fill="hsl({(hue + 40) % 360} 48% 76%)"/>
  <text x="80" y="152" text-anchor="middle" fill="hsl({hue} 42% 22%)" font-size="19" font-family="Arial, sans-serif" font-weight="700">{initials}</text>
</svg>'''
        (MEDIA_ROOT / "avatars" / f"avatar-{index:02d}.svg").write_text(svg, encoding="utf-8")


def remove_media(*, dry_run: bool = False) -> None:
    if not MEDIA_ROOT.exists():
        return
    if not MEDIA_MARKER.exists() or SEED_NAMESPACE not in MEDIA_MARKER.read_text(encoding="utf-8"):
        raise RuntimeError(f"Refusing to remove {MEDIA_ROOT}: seed marker is missing or does not match.")
    if dry_run:
        print(f"[dry-run] Would remove {MEDIA_ROOT}")
        return
    shutil.rmtree(MEDIA_ROOT)


def make_users() -> list[tuple[object, ...]]:
    names = [
        "星河绘本", "墨迹未干", "北岸放映机", "不熬夜的猫", "山海拾光", "小镇电台",
        "像素煮饭", "风从远方来", "纸上旅行者", "奶油云朵", "慢速快门", "夜航船",
        "柚子汽水", "雾中观鲸", "木木的工作台", "柠檬唱片", "海盐收藏家", "午后算法",
        "橘猫研究所", "纸飞机频道", "散步的云", "青苔档案", "玻璃糖纸", "宇宙便利店",
    ]
    rows: list[tuple[object, ...]] = []
    for index, name in enumerate(names, 1):
        email = f"seed-ui-{index:03d}{SEED_EMAIL_SUFFIX}"
        username = f"{name}{index:02d}"
        support_info = json.dumps({"seed_namespace": SEED_NAMESPACE, "seed_role": "ui_fixture"}, ensure_ascii=False)
        rows.append((email, username, f"/seed-media/avatars/avatar-{index:02d}.svg", f"本地界面测试样本创作者 · {name}", support_info, 8 + index % 16, "admin" if index == 1 else "user"))
    return rows


def make_ips() -> list[tuple[object, ...]]:
    ips = [
        ("苍穹档案", "远行者在漂浮群岛之间寻找失落坐标的幻想世界。", "gaming"),
        ("雾港列车", "一列永远抵达不了终点的夜行列车与它的乘客们。", "film_tv"),
        ("深海回声", "潜水员在海底电台中听见来自未来的讯号。", "music"),
        ("极昼邮局", "在没有夜晚的北方小镇，信件会替人完成告别。", "literature"),
        ("银杏电台", "旧城区广播站保存着每个人没有说出口的故事。", "anime"),
        ("纸月计划", "月球基地上的纸艺工程师重建一座会呼吸的城市。", "gaming"),
        ("南风食堂", "海边食堂的菜单会随着客人的心情悄悄变化。", "film_tv"),
        ("镜面花园", "所有植物都能映出另一种人生的温室实验。", "anime"),
        ("风筝信号", "少年们用风筝在山谷间传递天气与秘密。", "literature"),
        ("零点天文台", "城市停电后的深夜，天文台重新向公众开放。", "music"),
        ("琥珀旅社", "旅行者将记忆寄存在旅社换取一晚好眠。", "film_tv"),
        ("青苔机器人", "废弃工厂中的小机器人学习照顾一座植物园。", "gaming"),
        ("白昼梦工厂", "人们把白日梦剪辑成电影放映的奇妙社区。", "anime"),
        ("盐湖观测站", "研究员在干涸盐湖边记录不可解释的光。", "literature"),
        ("落日修理铺", "修理铺可以修好坏掉的物件，也会修好关系。", "other"),
        ("候鸟书店", "每年只在迁徙季营业的移动书店。", "literature"),
    ]
    rows: list[tuple[object, ...]] = []
    for index, (name, description, category) in enumerate(ips, 1):
        rows.append((f"{SEED_IP_SLUG_PREFIX}{index:02d}", name, description, f"/seed-media/ips/ip-{index:02d}.svg", category, f"seed-ui-{(index - 1) % 24 + 1:03d}{SEED_EMAIL_SUFFIX}", 20 + index * 7.3))
    return rows


def make_contents() -> list[ContentPlan]:
    original_categories = ["film_tv", "gaming", "literature", "pet", "food", "beauty_fashion", "home", "tech_digital", "travel", "sports", "productivity"]
    content_types = ["article", "image", "video", "audio", "template", "mod", "sheet_music"]
    topics = [
        "雨天的城市散步路线", "把旧照片做成一段小电影", "一人食的暖胃菜单", "我的桌面整理实验",
        "新手也能完成的配色练习", "周末露营清单", "低成本房间改造", "听歌时的画面笔记",
        "给忙碌生活留一小时", "手账里的旅行地图", "镜头下的普通朋友", "晨间跑步的第八周",
        "从零开始的桌游夜", "深夜电台歌单", "不完美但可爱的作品集", "城市边缘的植物观察",
    ]
    tags = ["灵感", "日常", "创作记录", "教程", "摄影", "设计", "治愈", "周末", "手作", "推荐", "观察", "共创", "收藏", "轻量测试"]
    now = datetime.now(timezone.utc)
    plans: list[ContentPlan] = []
    original_keys: list[str] = []
    statuses = (["published"] * 135 + ["pending"] * 18 + ["under_review"] * 12 + ["banned"] * 9 + ["draft"] * 6)
    RNG.shuffle(statuses)

    for index in range(180):
        zone = "original" if index < 112 else "fanwork"
        key = f"content-{index + 1:03d}"
        topic = topics[index % len(topics)]
        content_type = content_types[index % len(content_types)]
        category = original_categories[index % len(original_categories)]
        author_email = f"seed-ui-{index % 24 + 1:03d}{SEED_EMAIL_SUFFIX}"
        ip_slug = None if zone == "original" else f"{SEED_IP_SLUG_PREFIX}{index % 16 + 1:02d}"
        source_key = None if zone == "original" else original_keys[(index * 3) % len(original_keys)]
        title_prefix = "原创实验" if zone == "original" else "灵感衍生"
        title = f"{title_prefix}｜{topic} {index + 1:02d}"
        description = (
            f"这是用于本地界面测试的虚构{title_prefix}样本。围绕「{topic}」，记录了构思、取舍与可复用的小技巧。\n\n"
            "- 适合列表卡片、搜索、标签与详情页排版\n"
            "- 内容与人物均为虚构，不依赖外部图片、OSS 或审核服务\n"
            "- 欢迎在本地测试评论、收藏、推荐和筛选状态"
        )
        created_at = now - timedelta(days=index % 95, hours=(index * 7) % 23, minutes=(index * 13) % 59)
        is_public = not (statuses[index] == "published" and index % 31 == 0)
        plan = ContentPlan(
            key=key,
            title=title,
            description=description,
            author_email=author_email,
            zone=zone,
            ip_slug=ip_slug,
            source_key=source_key,
            category=category,
            content_type=content_type,
            cover_url=f"/seed-media/covers/cover-{index % 12 + 1:02d}.svg",
            status=statuses[index],
            is_public=is_public,
            allow_copy=index % 9 != 0,
            view_count=80 + (index * 137) % 18000,
            download_count=(index * 11) % 170,
            hot_score=round(8 + (index * 17.3) % 350, 2),
            rating_score=round(0.36 + ((index * 13) % 64) / 100, 2),
            created_at=created_at,
            tags=tuple(tags[(index + offset * 5) % len(tags)] for offset in range(3)),
        )
        plans.append(plan)
        if zone == "original":
            original_keys.append(key)
    return plans


def build_cleanup_sql() -> str:
    marker = sql(json.dumps({"seed_namespace": SEED_NAMESPACE}, ensure_ascii=False))
    return f"""
BEGIN;
DELETE FROM messages
WHERE sender_id IN (
  SELECT id FROM users WHERE email LIKE '%{SEED_EMAIL_SUFFIX}' AND support_info @> {marker}::jsonb
);
DELETE FROM conversations c
WHERE EXISTS (
  SELECT 1 FROM conversation_participants cp
  JOIN users u ON u.id = cp.user_id
  WHERE cp.conversation_id = c.id
    AND u.email LIKE '%{SEED_EMAIL_SUFFIX}'
    AND u.support_info @> {marker}::jsonb
);
DELETE FROM ips WHERE slug LIKE '{SEED_IP_SLUG_PREFIX}%';
DELETE FROM users
WHERE email LIKE '%{SEED_EMAIL_SUFFIX}'
  AND support_info @> {marker}::jsonb;
DELETE FROM tags WHERE category = '{SEED_NAMESPACE}';
COMMIT;
"""


def build_seed_sql() -> str:
    users = make_users()
    ips = make_ips()
    plans = make_contents()
    user_rows = rows_sql(users)
    ip_rows = rows_sql(ips)
    content_rows = rows_sql(
        (
            plan.key, plan.title, plan.description, plan.author_email, plan.zone, plan.ip_slug, plan.source_key,
            plan.category, plan.content_type, plan.cover_url, plan.status, plan.is_public, plan.allow_copy,
            plan.view_count, plan.download_count, plan.hot_score, plan.rating_score, plan.created_at.isoformat(),
        )
        for plan in plans
    )
    tag_rows = rows_sql((plan.key, tag) for plan in plans for tag in plan.tags)

    comment_bodies = [
        "这个角度很有意思，已经收藏留作以后参考。", "封面和标题搭配得很舒服。", "想看下一篇关于过程的展开！",
        "我在本地测试筛选时正好需要这样的样本。", "细节很丰富，感谢分享。", "这个思路让我想到另一种做法。",
    ]
    comment_rows = []
    for index in range(600):
        content_key = plans[index % len(plans)].key
        author_email = f"seed-ui-{(index * 5) % 24 + 1:03d}{SEED_EMAIL_SUFFIX}"
        status = "published" if index % 23 else "under_review"
        created_at = datetime.now(timezone.utc) - timedelta(days=index % 70, minutes=index * 9)
        comment_rows.append((content_key, author_email, comment_bodies[index % len(comment_bodies)], status, index % 36, created_at.isoformat()))

    reaction_rows = []
    seen_reactions: set[tuple[str, str]] = set()
    for index in range(2200):
        content_key = plans[index % len(plans)].key
        email = f"seed-ui-{(index * 7 + index // len(plans)) % 24 + 1:03d}{SEED_EMAIL_SUFFIX}"
        pair = (content_key, email)
        if pair in seen_reactions:
            continue
        seen_reactions.add(pair)
        reaction_rows.append((content_key, email, "dislike" if index % 17 == 0 else "like", (datetime.now(timezone.utc) - timedelta(days=index % 80)).isoformat()))

    favorite_rows = []
    seen_favorites: set[tuple[str, str]] = set()
    for index in range(700):
        content_key = plans[(index * 11) % len(plans)].key
        email = f"seed-ui-{(index * 3) % 24 + 1:03d}{SEED_EMAIL_SUFFIX}"
        if (content_key, email) not in seen_favorites:
            seen_favorites.add((content_key, email))
            favorite_rows.append((content_key, email, (datetime.now(timezone.utc) - timedelta(days=index % 62)).isoformat()))

    history_rows = []
    seen_history: set[tuple[str, str]] = set()
    for index in range(1400):
        content_key = plans[(index * 13) % len(plans)].key
        email = f"seed-ui-{(index * 11) % 24 + 1:03d}{SEED_EMAIL_SUFFIX}"
        if (content_key, email) not in seen_history:
            seen_history.add((content_key, email))
            history_rows.append((content_key, email, (datetime.now(timezone.utc) - timedelta(hours=index * 3)).isoformat()))

    follow_rows = []
    seen_follows: set[tuple[str, str]] = set()
    for index in range(180):
        follower = f"seed-ui-{index % 24 + 1:03d}{SEED_EMAIL_SUFFIX}"
        target = f"seed-ui-{(index * 5 + 3) % 24 + 1:03d}{SEED_EMAIL_SUFFIX}"
        if follower != target and (follower, target) not in seen_follows:
            seen_follows.add((follower, target))
            follow_rows.append((follower, target, (datetime.now(timezone.utc) - timedelta(days=index % 50)).isoformat()))

    notification_rows = []
    for index in range(240):
        recipient = f"seed-ui-{index % 24 + 1:03d}{SEED_EMAIL_SUFFIX}"
        sender = f"seed-ui-{(index * 7 + 1) % 24 + 1:03d}{SEED_EMAIL_SUFFIX}"
        target_key = plans[index % len(plans)].key
        channel = "like" if index % 3 == 0 else "reply" if index % 3 == 1 else "system"
        notice_type = "like" if channel == "like" else "comment" if channel == "reply" else "system"
        notification_rows.append((recipient, sender, target_key, notice_type, channel, "新的本地测试通知", "用于消息中心与未读状态测试。", index % 4 == 0, (datetime.now(timezone.utc) - timedelta(hours=index * 2)).isoformat()))

    collection_rows = []
    for index in range(24):
        email = f"seed-ui-{index + 1:03d}{SEED_EMAIL_SUFFIX}"
        collection_rows.append((email, "默认原创收藏", "original", True, False, 0, "本地测试默认收藏集"))
        collection_rows.append((email, "默认二创收藏", "fanwork", True, False, 0, "本地测试默认收藏集"))
        collection_rows.append((email, f"灵感整理 {index + 1:02d}", "original" if index % 2 == 0 else "fanwork", False, index % 3 == 0, 1, "用于列表、公开与私有状态测试。"))

    series_rows = []
    for index in range(10):
        owner = f"seed-ui-{index % 24 + 1:03d}{SEED_EMAIL_SUFFIX}"
        series_rows.append((f"系列灵感 {index + 1:02d}", "连续创作与排序展示的本地测试系列。", owner, "original" if index % 2 == 0 else "fanwork", plans[index * 3].key))

    return f"""
BEGIN;
CREATE TEMP TABLE seed_users (
  email TEXT, username TEXT, avatar_url TEXT, bio TEXT, support_info JSONB, reputation INT, role TEXT
) ON COMMIT DROP;
INSERT INTO seed_users VALUES\n{user_rows};
INSERT INTO users (email, password_hash, username, avatar_url, bio, support_info, reputation, role, email_verified_at)
SELECT email,
       '$2b$12$M0lmvqWJFMMjvFafrWiHrOfPAx10KALWzkKj/kGKiP2PndMgCOOcK',
       username, avatar_url, bio, support_info, reputation, role, NOW()
FROM seed_users;

CREATE TEMP TABLE seed_ips (
  slug TEXT, name TEXT, description TEXT, cover_url TEXT, category TEXT, creator_email TEXT, popularity_score DOUBLE PRECISION
) ON COMMIT DROP;
INSERT INTO seed_ips VALUES\n{ip_rows};
INSERT INTO ips (slug, name, description, cover_url, category, creator_id, status, popularity_score)
SELECT si.slug, si.name, si.description, si.cover_url, si.category, u.id, 'approved', si.popularity_score
FROM seed_ips si JOIN users u ON u.email = si.creator_email;

CREATE TEMP TABLE seed_content (
  seed_key TEXT, title TEXT, description TEXT, author_email TEXT, zone TEXT, ip_slug TEXT, source_key TEXT,
  category TEXT, content_type TEXT, cover_url TEXT, status TEXT, is_public BOOLEAN, allow_copy BOOLEAN,
  view_count BIGINT, download_count INT, hot_score DOUBLE PRECISION, rating_score DOUBLE PRECISION,
  created_at TIMESTAMPTZ, content_id BIGINT
) ON COMMIT DROP;
INSERT INTO seed_content (
  seed_key, title, description, author_email, zone, ip_slug, source_key, category, content_type, cover_url,
  status, is_public, allow_copy, view_count, download_count, hot_score, rating_score, created_at
) VALUES\n{content_rows};
INSERT INTO content_items (
  title, description, author_id, zone, ip_id, category, content_type, cover_image_url, status, is_public,
  allow_copy, view_count, download_count, hot_score, rating_score, created_at, updated_at
)
SELECT sc.title, sc.description, u.id, sc.zone, ip.id, sc.category, sc.content_type, sc.cover_url, sc.status,
       sc.is_public, sc.allow_copy, sc.view_count, sc.download_count, sc.hot_score, sc.rating_score,
       sc.created_at, sc.created_at
FROM seed_content sc
JOIN users u ON u.email = sc.author_email
LEFT JOIN ips ip ON ip.slug = sc.ip_slug;
UPDATE seed_content sc
SET content_id = ci.id
FROM content_items ci
JOIN users u ON u.id = ci.author_id
WHERE ci.title = sc.title AND u.email = sc.author_email AND ci.created_at = sc.created_at;
UPDATE content_items ci
SET source_original_id = source.content_id
FROM seed_content current_item
JOIN seed_content source ON source.seed_key = current_item.source_key
WHERE ci.id = current_item.content_id AND current_item.source_key IS NOT NULL;

CREATE TEMP TABLE seed_content_tags (seed_key TEXT, tag TEXT) ON COMMIT DROP;
INSERT INTO seed_content_tags VALUES\n{tag_rows};
INSERT INTO content_tags (content_item_id, tag)
SELECT sc.content_id, sct.tag FROM seed_content_tags sct JOIN seed_content sc ON sc.seed_key = sct.seed_key;
INSERT INTO tags (name, category, usage_count)
SELECT tag, '{SEED_NAMESPACE}', count(*)::INT FROM seed_content_tags GROUP BY tag
ON CONFLICT (name) DO NOTHING;

CREATE TEMP TABLE seed_comments (seed_key TEXT, author_email TEXT, body TEXT, status TEXT, like_count INT, created_at TIMESTAMPTZ) ON COMMIT DROP;
INSERT INTO seed_comments VALUES\n{rows_sql(comment_rows)};
INSERT INTO comments (content_item_id, author_id, body, content, target_type, target_id, status, like_count, created_at, updated_at)
SELECT sc.content_id, u.id, cm.body, cm.body, 'content', sc.content_id, cm.status, cm.like_count, cm.created_at, cm.created_at
FROM seed_comments cm JOIN seed_content sc ON sc.seed_key = cm.seed_key JOIN users u ON u.email = cm.author_email;

CREATE TEMP TABLE seed_reactions (seed_key TEXT, user_email TEXT, reaction TEXT, created_at TIMESTAMPTZ) ON COMMIT DROP;
INSERT INTO seed_reactions VALUES\n{rows_sql(reaction_rows)};
INSERT INTO reactions (user_id, target_type, target_id, reaction, created_at)
SELECT u.id, 'content', sc.content_id, sr.reaction, sr.created_at
FROM seed_reactions sr JOIN seed_content sc ON sc.seed_key = sr.seed_key JOIN users u ON u.email = sr.user_email;
UPDATE content_items ci
SET like_count = COALESCE(agg.likes, 0), dislike_count = COALESCE(agg.dislikes, 0)
FROM (
  SELECT target_id, count(*) FILTER (WHERE reaction = 'like')::INT AS likes,
         count(*) FILTER (WHERE reaction = 'dislike')::INT AS dislikes
  FROM reactions WHERE target_type = 'content' GROUP BY target_id
) agg WHERE ci.id = agg.target_id;

CREATE TEMP TABLE seed_favorites (seed_key TEXT, user_email TEXT, created_at TIMESTAMPTZ) ON COMMIT DROP;
INSERT INTO seed_favorites VALUES\n{rows_sql(favorite_rows)};
INSERT INTO favorites (user_id, content_item_id, created_at)
SELECT u.id, sc.content_id, sf.created_at
FROM seed_favorites sf JOIN seed_content sc ON sc.seed_key = sf.seed_key JOIN users u ON u.email = sf.user_email;

CREATE TEMP TABLE seed_history (seed_key TEXT, user_email TEXT, viewed_at TIMESTAMPTZ) ON COMMIT DROP;
INSERT INTO seed_history VALUES\n{rows_sql(history_rows)};
INSERT INTO browse_history (user_id, content_item_id, viewed_at)
SELECT u.id, sc.content_id, sh.viewed_at
FROM seed_history sh JOIN seed_content sc ON sc.seed_key = sh.seed_key JOIN users u ON u.email = sh.user_email;

CREATE TEMP TABLE seed_follows (follower_email TEXT, target_email TEXT, created_at TIMESTAMPTZ) ON COMMIT DROP;
INSERT INTO seed_follows VALUES\n{rows_sql(follow_rows)};
INSERT INTO follows (follower_id, target_type, target_id, created_at)
SELECT follower.id, 'user', target.id, sf.created_at
FROM seed_follows sf JOIN users follower ON follower.email = sf.follower_email JOIN users target ON target.email = sf.target_email;

CREATE TEMP TABLE seed_notifications (
  recipient_email TEXT, sender_email TEXT, seed_key TEXT, type TEXT, channel TEXT, title TEXT, body TEXT, is_read BOOLEAN, created_at TIMESTAMPTZ
) ON COMMIT DROP;
INSERT INTO seed_notifications VALUES\n{rows_sql(notification_rows)};
INSERT INTO notifications (user_id, type, channel, title, body, target_type, target_id, sender_id, is_read, created_at)
SELECT recipient.id, sn.type, sn.channel, sn.title, sn.body, 'content', sc.content_id, sender.id, sn.is_read, sn.created_at
FROM seed_notifications sn
JOIN users recipient ON recipient.email = sn.recipient_email
JOIN users sender ON sender.email = sn.sender_email
JOIN seed_content sc ON sc.seed_key = sn.seed_key;

CREATE TEMP TABLE seed_collections (
  user_email TEXT, title TEXT, zone TEXT, is_default BOOLEAN, is_public BOOLEAN, sort_order INT, description TEXT
) ON COMMIT DROP;
INSERT INTO seed_collections VALUES\n{rows_sql(collection_rows)};
INSERT INTO collections (user_id, title, description, zone, is_default, is_public, sort_order)
SELECT u.id, sc.title, sc.description, sc.zone, sc.is_default, sc.is_public, sc.sort_order
FROM seed_collections sc JOIN users u ON u.email = sc.user_email
ON CONFLICT (user_id, zone) WHERE is_default = TRUE DO UPDATE SET title = EXCLUDED.title, description = EXCLUDED.description;
INSERT INTO collection_items (collection_id, content_item_id, note, added_at)
SELECT c.id, sc.content_id, '用于收藏集卡片与排序测试。', NOW() - (row_number() OVER () || ' hours')::interval
FROM collections c
JOIN users u ON u.id = c.user_id
JOIN seed_content sc ON sc.zone = c.zone
WHERE u.email LIKE '%{SEED_EMAIL_SUFFIX}' AND c.is_default
  AND (sc.content_id + u.id) % 7 = 0
ON CONFLICT (collection_id, content_item_id) DO NOTHING;

CREATE TEMP TABLE seed_series (title TEXT, description TEXT, owner_email TEXT, zone TEXT, cover_key TEXT) ON COMMIT DROP;
INSERT INTO seed_series VALUES\n{rows_sql(series_rows)};
INSERT INTO content_series (title, description, owner_id, zone, cover_content_id)
SELECT ss.title, ss.description, u.id, ss.zone, sc.content_id
FROM seed_series ss JOIN users u ON u.email = ss.owner_email JOIN seed_content sc ON sc.seed_key = ss.cover_key;
INSERT INTO content_series_items (series_id, content_item_id, sort_order)
SELECT cs.id, sc.content_id, row_number() OVER (PARTITION BY cs.id ORDER BY sc.created_at)::INT
FROM content_series cs
JOIN users u ON u.id = cs.owner_id
JOIN seed_content sc ON sc.zone = cs.zone
WHERE u.email LIKE '%{SEED_EMAIL_SUFFIX}' AND (cs.id + sc.content_id) % 9 = 0
ON CONFLICT (series_id, content_item_id) DO NOTHING;
COMMIT;
"""


def verify_sql() -> str:
    marker = sql(json.dumps({"seed_namespace": SEED_NAMESPACE}, ensure_ascii=False))
    return f"""
\pset tuples_only on
SELECT 'seed_users=' || count(*) FROM users WHERE support_info @> {marker}::jsonb;
SELECT 'seed_ips=' || count(*) FROM ips WHERE slug LIKE '{SEED_IP_SLUG_PREFIX}%';
SELECT 'seed_content=' || count(*) FROM content_items ci JOIN users u ON u.id = ci.author_id WHERE u.support_info @> {marker}::jsonb;
SELECT 'seed_published_public=' || count(*) FROM content_items ci JOIN users u ON u.id = ci.author_id WHERE u.support_info @> {marker}::jsonb AND ci.status = 'published' AND ci.is_public;
SELECT 'seed_local_covers=' || count(*) FROM content_items ci JOIN users u ON u.id = ci.author_id WHERE u.support_info @> {marker}::jsonb AND ci.cover_image_url LIKE '/seed-media/%';
SELECT 'seed_comments=' || count(*) FROM comments c JOIN users u ON u.id = c.author_id WHERE u.support_info @> {marker}::jsonb;
SELECT 'seed_reactions=' || count(*) FROM reactions r JOIN users u ON u.id = r.user_id WHERE u.support_info @> {marker}::jsonb;
SELECT 'seed_favorites=' || count(*) FROM favorites f JOIN users u ON u.id = f.user_id WHERE u.support_info @> {marker}::jsonb;
SELECT 'seed_history=' || count(*) FROM browse_history bh JOIN users u ON u.id = bh.user_id WHERE u.support_info @> {marker}::jsonb;
SELECT 'seed_notifications=' || count(*) FROM notifications n JOIN users u ON u.id = n.user_id WHERE u.support_info @> {marker}::jsonb;
SELECT 'seed_collections=' || count(*) FROM collections c JOIN users u ON u.id = c.user_id WHERE u.support_info @> {marker}::jsonb;
SELECT 'seed_series=' || count(*) FROM content_series cs JOIN users u ON u.id = cs.owner_id WHERE u.support_info @> {marker}::jsonb;
"""


def verify(*, dry_run: bool = False) -> None:
    output = execute_sql(verify_sql(), capture=True, dry_run=dry_run)
    if dry_run:
        return
    metrics = {}
    for raw_line in output.splitlines():
        line = raw_line.strip()
        if "=" in line:
            key, value = line.split("=", 1)
            metrics[key] = int(value)
    for key in sorted(metrics):
        print(f"{key}: {metrics[key]}")
    minimums = {"seed_users": 24, "seed_ips": 16, "seed_content": 180, "seed_published_public": 120, "seed_local_covers": 180, "seed_comments": 500}
    failures = [f"{key}={metrics.get(key, 0)} < {minimum}" for key, minimum in minimums.items() if metrics.get(key, 0) < minimum]
    if failures:
        raise RuntimeError("Verification failed: " + ", ".join(failures))


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=("seed", "verify", "cleanup", "reset"))
    parser.add_argument("--dry-run", action="store_true", help="show planned action without writing files or database rows")
    args = parser.parse_args()
    try:
        if args.command == "cleanup":
            execute_sql(build_cleanup_sql(), dry_run=args.dry_run)
            remove_media(dry_run=args.dry_run)
            print("Removed only local-rich seed data and seed media.")
        elif args.command == "verify":
            verify(dry_run=args.dry_run)
        else:
            # Both seed and reset deliberately replace only this seed namespace, keeping the result deterministic.
            execute_sql(build_cleanup_sql(), dry_run=args.dry_run)
            ensure_media(dry_run=args.dry_run)
            execute_sql(build_seed_sql(), dry_run=args.dry_run)
            verify(dry_run=args.dry_run)
            print("Local rich seed data is ready. No OSS, moderation, or network crawling was used.")
    except (OSError, RuntimeError, subprocess.SubprocessError) as error:
        print(f"ERROR: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
