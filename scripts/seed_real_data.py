#!/usr/bin/env python3
"""Create realistic, media-rich local data for OmniCraft UI and feature testing.

This seeder downloads real open-licensed media (images from Wikimedia Commons,
photos/videos from Pexels) into ``frontend/public/seed-media/real/`` and writes
records directly to the local Docker PostgreSQL service.  It never calls OSS or
content-moderation services.

Media sources are limited to clearly open/free licenses:
  - Wikimedia Commons images (CC0 / CC-BY / CC-BY-SA / Public domain)
  - Pexels photos and videos (Pexels License, free to use)

Commands:
  python scripts/seed_real_data.py seed
  python scripts/seed_real_data.py verify
  python scripts/seed_real_data.py cleanup
  python scripts/seed_real_data.py reset

All database records are owned by users carrying ``SEED_NAMESPACE`` in
``support_info``.  ``cleanup`` removes only those users and their dependent
records; it does not touch manually-created local data.

Default accounts (idempotent upsert):
  - admin@omnicraft.com / Admin123456   (role=admin)
  - seed-real-001@omnicraft.local / Seed123456 ... seed-real-010@omnicraft.local / Seed123456
"""

from __future__ import annotations

import argparse
import json
import os
import random
import shutil
import subprocess
import sys
import time
import urllib.error
import urllib.request
from dataclasses import dataclass
from datetime import datetime, timedelta, timezone
from pathlib import Path
from typing import Iterable, Sequence


SEED_NAMESPACE = "real-data-v1"
SEED_EMAIL_SUFFIX = "@omnicraft.local"
ADMIN_EMAIL = "admin@omnicraft.com"
ADMIN_PASSWORD_HASH = "$2a$10$wy.O0okVxIk4uPVNFieKWOqSqT2YoBsYAJ2WaH4NSrHVkiVr1TsiW"  # Admin123456
USER_PASSWORD_HASH = "$2a$10$jOQl/hiSM1udhQhg2i6ZF.3f50lQ6q7OOc2IEC7tRpZlzbF0yOcOa"  # Seed123456
SEED_IP_SLUG_PREFIX = "seed-real-"
SCRIPT_DIR = Path(__file__).resolve().parent
REPO_ROOT = SCRIPT_DIR.parent
MEDIA_ROOT = REPO_ROOT / "frontend" / "public" / "seed-media" / "real"
MEDIA_MARKER = MEDIA_ROOT / ".omnicraft-real-data-seed"
RNG = random.Random(20260801)
UA = "OmniCraft-Dev/1.0 (local dev seeding)"


# --------------------------------------------------------------------------
# Open-licensed media manifest
# --------------------------------------------------------------------------

# name -> (url, license_note).  Names become local filenames under MEDIA_ROOT.
IMAGE_ASSETS = {
    # Fitness / sports
    "gym-dumbbell.jpg": ("https://upload.wikimedia.org/wikipedia/commons/thumb/1/14/Attractive_man_lifting_dumbbell_weight_for_exercise_in_fitness_gym.jpg/960px-Attractive_man_lifting_dumbbell_weight_for_exercise_in_fitness_gym.jpg", "CC BY 2.0"),
    "gym-instruments.jpg": ("https://upload.wikimedia.org/wikipedia/commons/thumb/9/9f/Gym_instruments.jpg/960px-Gym_instruments.jpg", "CC BY-SA 4.0"),
    "jogger.jpg": ("https://upload.wikimedia.org/wikipedia/commons/thumb/3/3e/Jogger_outside_Hong_Kong_Museum_of_Art.jpg/960px-Jogger_outside_Hong_Kong_Museum_of_Art.jpg", "CC BY-SA 4.0"),
    "treadmill.jpg": ("https://upload.wikimedia.org/wikipedia/commons/thumb/9/95/Exercise_Treadmill_Convey_Motion.jpg/960px-Exercise_Treadmill_Convey_Motion.jpg", "CC BY-SA 4.0"),
    # Food / coffee
    "breakfast.jpg": ("https://upload.wikimedia.org/wikipedia/commons/thumb/6/63/Healthy_Breakfast_%28Unsplash%29.jpg/960px-Healthy_Breakfast_%28Unsplash%29.jpg", "CC0"),
    "breakfast-1.jpg": ("https://upload.wikimedia.org/wikipedia/commons/thumb/c/ca/Healthy_Breakfast_1_%28Unsplash%29.jpg/960px-Healthy_Breakfast_1_%28Unsplash%29.jpg", "CC0"),
    "cooking.jpg": ("https://upload.wikimedia.org/wikipedia/commons/thumb/9/97/Cooking_a_delicious_meal_with_fresh_ingredients.jpg/960px-Cooking_a_delicious_meal_with_fresh_ingredients.jpg", "CC BY 2.0"),
    "latte-art.jpg": ("https://upload.wikimedia.org/wikipedia/commons/thumb/d/d6/Cappuccino_with_latte_art_on_Coffee_Right_in_Brno%2C_Brno-City_District.jpg/960px-Cappuccino_with_latte_art_on_Coffee_Right_in_Brno%2C_Brno-City_District.jpg", "CC BY 3.0"),
    "latte-art-2.jpg": ("https://upload.wikimedia.org/wikipedia/commons/thumb/7/76/Cup_of_coffee_with_latte_art_2016.jpg/960px-Cup_of_coffee_with_latte_art_2016.jpg", "CC BY-SA 4.0"),
    # Pets
    "kitten.jpg": ("https://upload.wikimedia.org/wikipedia/commons/thumb/c/c1/Six_weeks_old_cat_%28aka%29.jpg/960px-Six_weeks_old_cat_%28aka%29.jpg", "CC BY-SA 2.5"),
    "cat-sphynx.jpg": ("https://upload.wikimedia.org/wikipedia/commons/thumb/2/29/Cat_Sphynx._Kittens._img_11.jpg/960px-Cat_Sphynx._Kittens._img_11.jpg", "CC BY-SA 4.0"),
    # Home / office
    "home-office.jpg": ("https://upload.wikimedia.org/wikipedia/commons/thumb/7/77/Big_Desk_Home_Office_Carrollton_New_Orleans.jpg/960px-Big_Desk_Home_Office_Carrollton_New_Orleans.jpg", "CC BY 2.0"),
    # Fashion
    "fashion.jpg": ("https://upload.wikimedia.org/wikipedia/commons/thumb/3/30/Stephanie_with_a_very_cool_outfit_%28IMG_7668a%29_%285459422457%29.jpg/960px-Stephanie_with_a_very_cool_outfit_%28IMG_7668a%29_%285459422457%29.jpg", "CC BY 2.0"),
    # Travel / cycling
    "cycling.jpg": ("https://upload.wikimedia.org/wikipedia/commons/thumb/8/82/Man_riding_bicycle_on_Pieter_Calandlaan%2C_Amsterdam.jpg/960px-Man_riding_bicycle_on_Pieter_Calandlaan%2C_Amsterdam.jpg", "CC BY 4.0"),
    "mountain.jpg": ("https://upload.wikimedia.org/wikipedia/commons/thumb/6/60/Rocky_Mountain_Landscape%2C_by_Albert_Bierstadt.jpg/960px-Rocky_Mountain_Landscape%2C_by_Albert_Bierstadt.jpg", "Public domain"),
    # Literature / reading
    "manga-reader.jpg": ("https://upload.wikimedia.org/wikipedia/commons/thumb/5/53/Manga_reader_on_the_train_%28485007375%29.jpg/960px-Manga_reader_on_the_train_%28485007375%29.jpg", "CC BY-SA 2.0"),
}

# IP covers (real franchises, using freely-licensed images on Commons).
IP_IMAGE_ASSETS = {
    "ip-elden-ring.jpg": ("https://upload.wikimedia.org/wikipedia/commons/thumb/8/85/ELDEN_RING_%E2%80%93_OST_Behind_the_Scenes_with_The_Budapest_Film_Orchestra.webm/960px--ELDEN_RING_%E2%80%93_OST_Behind_the_Scenes_with_The_Budapest_Film_Orchestra.webm.jpg", "CC BY 3.0 (video still)"),
    "ip-witcher.jpg": ("https://upload.wikimedia.org/wikipedia/commons/thumb/0/06/Ciri_Cosplay_%28The_Witcher_3_Wild_Hunt%29_%E2%80%A2_2.jpg/960px-Ciri_Cosplay_%28The_Witcher_3_Wild_Hunt%29_%E2%80%A2_2.jpg", "CC BY 2.0 (cosplay photo)"),
    "ip-genshin.jpg": ("https://upload.wikimedia.org/wikipedia/commons/thumb/3/3b/Genshin_Impact_Booth%2C_BilibiliWorld_2021_20210711A.jpg/960px-Genshin_Impact_Booth%2C_BilibiliWorld_2021_20210711A.jpg", "CC BY-SA 4.0"),
    "ip-demonslayer.jpg": ("https://upload.wikimedia.org/wikipedia/commons/thumb/6/60/5_people_standing_on_the_stage_with_Demon_Slayer_cosplay_clothing_20210321g.jpg/960px-5_people_standing_on_the_stage_with_Demon_Slayer_cosplay_clothing_20210321g.jpg", "CC0 (cosplay photo)"),
    "ip-starrail.jpg": ("https://upload.wikimedia.org/wikipedia/commons/thumb/f/f8/Cosplay_of_Honkai_Star_Rail_characters.jpg/960px-Cosplay_of_Honkai_Star_Rail_characters.jpg", "CC0 (cosplay photo)"),
    "ip-cyberpunk.jpg": ("https://upload.wikimedia.org/wikipedia/commons/thumb/2/2b/PGA_2019_Cyberpunk_2077.jpg/960px-PGA_2019_Cyberpunk_2077.jpg", "CC BY 2.0"),
}

# name -> (url, license_note).  Pexels License: free to use.
VIDEO_ASSETS = {
    "fitness-1.mp4": ("https://www.pexels.com/download/video/853809/", "Pexels License"),
    "fitness-2.mp4": ("https://www.pexels.com/download/video/857097/", "Pexels License"),
    "fitness-3.mp4": ("https://www.pexels.com/download/video/4761931/", "Pexels License"),
}

# Avatars (Pexels License).
AVATAR_ASSETS = {
    f"avatar-{i}.jpg": (f"https://images.pexels.com/photos/{photo_id}/pexels-photo-{photo_id}.jpeg?auto=compress&cs=tinysrgb&w=400&h=400&fit=crop", "Pexels License")
    for i, photo_id in enumerate([220453, 774909, 1222271, 415829, 91227, 774095, 1704488, 220453, 774909, 1222271], 1)
}


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


def download(url: str, dest: Path) -> bool:
    """Download url to dest if missing.  Returns True when a file now exists."""
    if dest.exists() and dest.stat().st_size > 0:
        return True
    dest.parent.mkdir(parents=True, exist_ok=True)
    req = urllib.request.Request(url, headers={"User-Agent": UA})
    tmp = dest.with_suffix(dest.suffix + ".part")
    last_error: Exception | None = None
    for attempt in range(4):
        try:
            with urllib.request.urlopen(req, timeout=120) as resp, open(tmp, "wb") as fh:
                shutil.copyfileobj(resp, fh)
            tmp.replace(dest)
            return True
        except (urllib.error.HTTPError, OSError) as error:
            last_error = error
            # Backoff on rate limiting / transient errors, then retry.
            time.sleep(2 + attempt * 3)
    if last_error is not None:
        if tmp.exists():
            tmp.unlink()
        print(f"  WARN download failed: {dest.name}: {last_error}")
        return False
    return False


def ensure_media(*, dry_run: bool = False) -> None:
    if dry_run:
        print(f"[dry-run] Would download open-licensed media into {MEDIA_ROOT}")
        return
    MEDIA_ROOT.mkdir(parents=True, exist_ok=True)
    MEDIA_MARKER.write_text(f"namespace={SEED_NAMESPACE}\n", encoding="utf-8")
    for folder in ("covers", "ips", "videos", "avatars"):
        (MEDIA_ROOT / folder).mkdir(exist_ok=True)

    ok = fail = 0
    for name, (url, _license) in IMAGE_ASSETS.items():
        if download(url, MEDIA_ROOT / "covers" / name):
            ok += 1
        else:
            fail += 1
    for name, (url, _license) in IP_IMAGE_ASSETS.items():
        if download(url, MEDIA_ROOT / "ips" / name):
            ok += 1
        else:
            fail += 1
    for name, (url, _license) in AVATAR_ASSETS.items():
        if download(url, MEDIA_ROOT / "avatars" / name):
            ok += 1
        else:
            fail += 1
    for name, (url, _license) in VIDEO_ASSETS.items():
        if download(url, MEDIA_ROOT / "videos" / name):
            ok += 1
        else:
            fail += 1
    print(f"  media: {ok} ready, {fail} failed")


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
    """Admin + 10 regular seed users.  Admin gets the fixed admin email/password."""
    names = [
        "晨雾跑步者", "厨房实验室", "山野拾光", "橘猫观察员", "通勤造型师",
        "书房改造家", "拉花练习生", "单车通勤族", "深夜书虫", "数码开箱君",
    ]
    rows: list[tuple[object, ...]] = []
    # Admin
    rows.append((
        ADMIN_EMAIL, ADMIN_PASSWORD_HASH, "万象管理员",
        "/seed-media/real/avatars/avatar-1.jpg",
        "OmniCraft 官方测试管理员账号。", "{}", 999, "admin",
    ))
    for index, name in enumerate(names, 1):
        email = f"seed-real-{index:03d}{SEED_EMAIL_SUFFIX}"
        username = f"{name}{index:02d}"
        avatar = f"/seed-media/real/avatars/avatar-{(index % 10) + 1}.jpg"
        support_info = json.dumps({"seed_namespace": SEED_NAMESPACE}, ensure_ascii=False)
        rows.append((email, USER_PASSWORD_HASH, username, avatar, f"本地真实数据种子创作者 · {name}", support_info, 20 + index * 13, "user"))
    return rows


def make_ips() -> list[tuple[object, ...]]:
    """Real franchises with freely-licensed Commons cover images."""
    ips = [
        ("艾尔登法环", "FromSoftware 的开放世界动作 RPG，宫崎英高与乔治·R·R·马丁共创。", "gaming", "ip-elden-ring.jpg"),
        ("巫师3：狂猎", "CD Projekt RED 开发的开放世界角色扮演游戏。", "gaming", "ip-witcher.jpg"),
        ("原神", "米哈游开发的开放世界冒险游戏，提瓦特大陆。", "gaming", "ip-genshin.jpg"),
        ("鬼灭之刃", "吾峠呼世晴创作的少年漫画及其动画。", "anime", "ip-demonslayer.jpg"),
        ("崩坏：星穹铁道", "米哈游科幻回合制 RPG，星际旅行题材。", "gaming", "ip-starrail.jpg"),
        ("赛博朋克2077", "CD Projekt RED 开发的开放世界科幻游戏。", "gaming", "ip-cyberpunk.jpg"),
    ]
    rows: list[tuple[object, ...]] = []
    for index, (name, description, category, cover) in enumerate(ips, 1):
        rows.append((
            f"{SEED_IP_SLUG_PREFIX}{index:02d}", name, description,
            f"/seed-media/real/ips/{cover}", category, index,
        ))
    return rows


def make_contents() -> list[ContentPlan]:
    plans: list[ContentPlan] = []
    original_keys: list[str] = []
    now = datetime.now(timezone.utc)

    # (title, description, author_idx, zone, ip_slug, category, content_type, cover, status, tags)
    spec = [
        # --- 原创区：健身 / 运动 ---
        ("新手必看：哑铃全身训练 20 分钟完整计划",
         "## 训练前\n\n- 充分热身 5 分钟（肩部环绕 + 高抬腿）\n- 重量选择：每组最后 2 次感到吃力为宜\n\n### 训练计划\n\n1. 高脚杯深蹲 4×12\n2. 俯身划船 4×10\n3. 肩上推举 3×10\n4. 哑铃硬拉 4×12\n5. 平板支撑 3×45s\n\n> 健身没有捷径，但可以更聪明。坚持四周你会看到明显变化。",
         1, "original", None, "sports", "article", "covers/gym-dumbbell.jpg", "published", ("健身", "力量训练", "新手教程")),
        ("健身房肩部训练实录：器械 + 自由重量混合",
         "今天练肩，记录一下完整流程：坐姿推举、侧平举、反向飞鸟。每个动作 4 组，组间休息 60 秒。器械和自由重量交替做，能让三角肌得到更全面的刺激。最后用递减组收尾，泵感拉满。",
         2, "original", None, "sports", "image", "covers/gym-instruments.jpg", "published", ("健身", "肩部", "训练记录")),
        ("城市晨跑 5 公里记录：从起点到美术馆",
         "六点半的街道还很安静，沿着海边跑到美术馆正好 5 公里。配速 5'40''，心率稳定在 140 左右。跑步的好处在于，你永远不知道今天会遇到什么样的天空。",
         1, "original", None, "sports", "image", "covers/jogger.jpg", "published", ("跑步", "晨跑", "生活方式")),
        ("居家跑步机训练指南：雨天也能高效燃脂",
         "## 为什么选跑步机\n\n- 心率可控、坡度可调、膝盖友好\n- 雨天和冬天的完美替代方案\n\n### 推荐课表\n\n- 5 分钟快走热身 → 30 分钟变速跑 → 5 分钟放松\n- 每周 3 次，隔天进行",
         3, "original", None, "sports", "video", "covers/treadmill.jpg", "published", ("跑步", "跑步机", "燃脂")),
        # --- 原创区：美食 ---
        ("15 分钟营养早餐碗：燕麦 + 水果 + 坚果",
         "工作日也能吃到的营养早餐：即食燕麦打底，加酸奶、蓝莓、香蕉和一把坚果，最后淋一点蜂蜜。饱腹感能撑到中午，重点是几乎不用动火。",
         2, "original", None, "food", "image", "covers/breakfast.jpg", "published", ("早餐", "健康饮食", "快手菜")),
        ("一周工作日午餐备餐计划：5 天 5 盒",
         "周末花两小时准备好一周的午餐：藜麦打底，蛋白质（鸡胸/虾仁/豆腐）+ 当季蔬菜 + 优质脂肪。装进玻璃盒，冰箱冷藏，每天带一盒去公司，省钱又健康。",
         2, "original", None, "food", "image", "covers/breakfast-1.jpg", "published", ("备餐", "健康饮食", "职场人")),
        ("厨房新手也能做：番茄牛肉意面",
         "## 步骤\n\n1. 洋葱蒜末爆香\n2. 下牛肉末炒至变色\n3. 加番茄丁和番茄酱炖 10 分钟\n4. 拌入煮好的意面，撒帕玛森芝士\n\n全程 25 分钟，成就感满满。",
         4, "original", None, "food", "image", "covers/cooking.jpg", "published", ("意面", "家常菜", "新手")),
        ("拉花练习第 100 天：从心形到郁金香",
         "记录一下拉花学习过程：前 30 天基本只会大白点，第 60 天稳定出心形，第 90 天终于拉出第一朵郁金香。奶泡打发和融合是全部秘密。慢慢来，比较快。",
         7, "original", None, "food", "image", "covers/latte-art.jpg", "published", ("咖啡", "拉花", "练习记录")),
        ("咖啡馆打工笔记：为什么奶泡决定一杯拿铁",
         "打工第三个月，终于明白为什么同一台机器，师傅拉的花总比我的好看。融合不到位，后面拉什么都白搭。分享几个奶泡打发的要点，希望对新手有帮助。",
         7, "original", None, "food", "article", "covers/latte-art-2.jpg", "published", ("咖啡", "拉花", "笔记")),
        # --- 原创区：宠物 ---
        ("捡到流浪小猫的第 30 天：从 700g 到 1.8kg",
         "第一天：在楼下纸箱里捡到它，瘦得只有一把骨头。\n\n第七天：开始主动蹭人了。\n\n第 30 天：体重翻了快三倍，已经会满屋子跑酷。名字叫「八筒」，因为鼻子上有个八字纹。",
         5, "original", None, "pet", "image", "covers/kitten.jpg", "published", ("猫", "救助", "养猫日记")),
        ("新手养猫避坑指南：接猫前的 10 件准备",
         "1. 猫砂盆要买大的（大 1.5 倍尺寸）\n2. 猫抓板至少两个，防止拆沙发\n3. 疫苗和驱虫按时间表走\n4. 封窗！封窗！封窗！\n5. 猫粮先延续原主人家吃的，再缓慢过渡\n\n接猫一时爽，准备要做足。",
         5, "original", None, "pet", "article", "covers/cat-sphynx.jpg", "published", ("猫", "养猫", "避坑")),
        # --- 原创区：家居 ---
        ("出租屋改造：我的 12㎡ 书桌工作区",
         "用 300 元预算改造了出租屋的书桌：升降桌腿 + 木质置物架 + 暖光台灯 + 一块洞洞板。现在的桌面终于有「工作室」的感觉了，写代码的效率都高了不少。",
         6, "original", None, "home", "image", "covers/home-office.jpg", "published", ("出租屋", "书桌", "收纳")),
        ("居家办公幸福感指南：把角落变成治愈系书房",
         "## 三件提升幸福感的小事\n\n- 光线：暖色落地灯比顶灯舒服 10 倍\n- 绿植：一盆水培植物就能让心情变好\n- 收纳：所有线材藏起来，桌面留白\n\n好的环境会反过来塑造你的状态。",
         6, "original", None, "home", "article", "covers/home-office.jpg", "published", ("家居", "书房", "效率")),
        # --- 原创区：时尚 ---
        ("春季通勤穿搭：低饱和叠穿 3 套 Look",
         "LOOK 1：燕麦色西装 + 同色系阔腿裤\nLOOK 2：浅灰针织 + 白色直筒裙\nLOOK 3：牛仔衬衫 + 卡其裤叠穿\n\n春天就是叠穿的季节，低饱和度颜色怎么搭都不会错。",
         5, "original", None, "beauty_fashion", "image", "covers/fashion.jpg", "published", ("穿搭", "通勤", "春季")),
        # --- 原创区：旅行 ---
        ("骑车通勤两周体验：单程 12 公里的快乐",
         "改骑自行车通勤两周了：早上沿河边骑 12 公里，到公司神清气爽；下班再骑回来，一天的疲惫都散掉了。比地铁省 20 分钟，还能顺便锻炼。",
         8, "original", None, "travel", "image", "covers/cycling.jpg", "published", ("骑行", "通勤", "环保")),
        ("徒步小白第一座山：装备清单与安全提醒",
         "第一次徒步别贪高：选成熟路线、结伴而行、下载离线地图。必备：登山鞋、速干衣、2L 水、能量棒、头灯、雨衣。走到山顶回头看，一切都值得。",
         3, "original", None, "travel", "image", "covers/mountain.jpg", "published", ("徒步", "户外", "攻略")),
        # --- 原创区：文学 ---
        ("短篇科幻 | 《信号》",
         "## 一\n\n天文台的凌晨，所有设备同时安静了下来。\n\n## 二\n\n不是故障。是一段规律的、来自猎户座的脉冲。\n\n## 三\n\n我们花了一个月确认这不是银河系内的信号。然后，用了更长时间争论要不要回复。\n\n## 四\n\n最后一个值班的夜晚，我把食指放在发送键上。窗外，猎户座刚刚升起来。",
         9, "original", None, "literature", "article", "covers/manga-reader.jpg", "published", ("科幻", "短篇", "原创")),
        ("通勤路上的阅读：地铁上的 30 分钟",
         "每天通勤两小时，坚持阅读三周后，发现一个月能读完两本书。方法：书放背包固定位置，下车不玩手机。纸质书在地铁上的阅读体验，真的比电子屏幕好。",
         9, "original", None, "literature", "article", "covers/manga-reader.jpg", "published", ("阅读", "通勤", "自我提升")),
        # --- 原创区：数码 ---
        ("新 MacBook 开箱与迁移指南",
         "把用了四年的旧 Mac 换成新款，分享一下迁移流程：Time Machine 备份 → 新机导入 → 登录 iCloud → 重装必要软件。全程大概两小时，旧机数据一点没丢。",
         10, "original", None, "tech_digital", "article", "covers/home-office.jpg", "published", ("Mac", "开箱", "迁移")),
        # --- 二创区：各 IP ---
        ("艾尔登法环隐藏 Boss 速杀思路分享",
         "卡了大半周的 Boss 终于过了，分享一下思路：带好火焰附魔、贴身打输出窗口、注意第二阶段起跳砸地。装备堆满防火抗性可以硬抗大招。希望对各位褪色者有帮。",
         1, "fanwork", "seed-real-01", "gaming", "article", "ips/ip-elden-ring.jpg", "published", ("艾尔登法环", "攻略", "动作游戏")),
        ("巫师3 Ciri 同人插画练习：夜幕下的白狼",
         "画了一幅夜幕下的杰洛特和 Ciri，参考了游戏里的名场面。工具：Procreate，耗时 12 小时。希望大家喜欢这个版本的猎魔人父女。",
         2, "fanwork", "seed-real-02", "gaming", "image", "ips/ip-witcher.jpg", "published", ("巫师3", "同人插画", "杰洛特")),
        ("原神四周年：我最喜欢的角色合集",
         "入坑原神四年，整理一组最爱的角色截图和同人练习。蒙德的自由、璃月的烟火、稻妻的雷电，还有须弥的智慧，每一个国度都有值得记住的人。",
         3, "fanwork", "seed-real-03", "gaming", "image", "ips/ip-genshin.jpg", "published", ("原神", "角色", "同人")),
        ("鬼灭之刃 Cosplay 出片记录：游郭篇主题",
         "和小伙伴一起拍了鬼灭之刃游郭篇主题的片子。服装细节还原到发饰和羽织的纹样，灯光选了黄昏的金橙色。感谢摄影师的耐心，成片很满意！",
         4, "fanwork", "seed-real-04", "anime", "image", "ips/ip-demonslayer.jpg", "published", ("鬼灭之刃", "cosplay", "游郭篇")),
        ("崩坏：星穹铁道角色 Cosplay 群像",
         "和同好们约了一组星穹铁道角色的群像拍摄。角色的服装和道具都是大家一起手工做的，拍摄当天天气很好，原片就很好看。",
         5, "fanwork", "seed-real-05", "gaming", "image", "ips/ip-starrail.jpg", "published", ("星穹铁道", "cosplay", "群像")),
        ("赛博朋克2077 夜之城截图精选：雨夜的霓虹",
         "夜之城的雨夜永远拍不够。分享一组在游戏里随手截的图：霓虹倒映在积水里、高耸的摩天楼、还有角落里不肯睡觉的流浪汉。这个城市比传说中更有人味。",
         6, "fanwork", "seed-real-06", "gaming", "image", "ips/ip-cyberpunk.jpg", "published", ("赛博朋克2077", "截图", "夜之城")),
    ]

    for index, (title, desc, author_idx, zone, ip_slug, category, ctype, cover, status, tags) in enumerate(spec, 1):
        key = f"content-{index:03d}"
        author_email = ADMIN_EMAIL if author_idx == 0 else f"seed-real-{author_idx:03d}{SEED_EMAIL_SUFFIX}"
        created_at = now - timedelta(days=index % 60, hours=(index * 5) % 23, minutes=(index * 11) % 59)
        plans.append(ContentPlan(
            key=key,
            title=title,
            description=desc,
            author_email=author_email,
            zone=zone,
            ip_slug=ip_slug,
            source_key=None,
            category=category,
            content_type=ctype,
            cover_url=f"/seed-media/real/{cover}",
            status=status,
            is_public=True,
            allow_copy=index % 9 != 0,
            view_count=300 + (index * 937) % 20000,
            download_count=(index * 17) % 210,
            hot_score=round(20 + (index * 53.7) % 400, 2),
            rating_score=round(0.62 + ((index * 7) % 37) / 100, 2),
            created_at=created_at,
            tags=tags,
        ))
        if zone == "original":
            original_keys.append(key)
    return plans


def build_cleanup_sql() -> str:
    marker = sql(json.dumps({"seed_namespace": SEED_NAMESPACE}, ensure_ascii=False))
    return f"""
BEGIN;
DELETE FROM notifications
WHERE user_id IN (SELECT id FROM users WHERE support_info @> {marker}::jsonb)
   OR sender_id IN (SELECT id FROM users WHERE support_info @> {marker}::jsonb);
DELETE FROM messages WHERE sender_id IN (
  SELECT id FROM users WHERE support_info @> {marker}::jsonb
);
DELETE FROM conversations c WHERE EXISTS (
  SELECT 1 FROM conversation_participants cp
  JOIN users u ON u.id = cp.user_id
  WHERE cp.conversation_id = c.id AND u.support_info @> {marker}::jsonb
);
DELETE FROM ips WHERE slug LIKE '{SEED_IP_SLUG_PREFIX}%';
DELETE FROM users WHERE support_info @> {marker}::jsonb OR email = '{ADMIN_EMAIL}';
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
            plan.key, plan.title, plan.description, plan.author_email, plan.zone, plan.ip_slug,
            plan.category, plan.content_type, plan.cover_url, plan.status, plan.is_public,
            plan.allow_copy, plan.view_count, plan.download_count, plan.hot_score, plan.rating_score,
            plan.created_at.isoformat(),
        )
        for plan in plans
    )
    tag_rows = rows_sql((plan.key, tag) for plan in plans for tag in plan.tags)

    comment_bodies = [
        "太实用了，正好最近需要，收藏了！", "细节满满，看得出是用心写的。", "学到了，感谢分享！",
        "同感，之前一直没找到合适的方案。", "封面很好看，内容也很扎实。", "期待下一篇！",
        "实践一周后来反馈：真的有效！", "请问能出一期进阶版吗？",
    ]
    comment_rows = []
    for index in range(240):
        content_key = plans[index % len(plans)].key
        author_email = f"seed-real-{(index * 5) % 10 + 1:03d}{SEED_EMAIL_SUFFIX}"
        status = "published" if index % 23 else "under_review"
        created_at = datetime.now(timezone.utc) - timedelta(days=index % 45, minutes=index * 7)
        comment_rows.append((content_key, author_email, comment_bodies[index % len(comment_bodies)], status, index % 32, created_at.isoformat()))

    reaction_rows = []
    seen_reactions: set[tuple[str, str]] = set()
    for index in range(900):
        content_key = plans[index % len(plans)].key
        email = f"seed-real-{(index * 7 + index // len(plans)) % 10 + 1:03d}{SEED_EMAIL_SUFFIX}"
        pair = (content_key, email)
        if pair in seen_reactions:
            continue
        seen_reactions.add(pair)
        reaction_rows.append((content_key, email, "dislike" if index % 17 == 0 else "like", (datetime.now(timezone.utc) - timedelta(days=index % 40)).isoformat()))

    favorite_rows = []
    seen_favorites: set[tuple[str, str]] = set()
    for index in range(260):
        content_key = plans[(index * 11) % len(plans)].key
        email = f"seed-real-{(index * 3) % 10 + 1:03d}{SEED_EMAIL_SUFFIX}"
        if (content_key, email) not in seen_favorites:
            seen_favorites.add((content_key, email))
            favorite_rows.append((content_key, email, (datetime.now(timezone.utc) - timedelta(days=index % 30)).isoformat()))

    history_rows = []
    seen_history: set[tuple[str, str]] = set()
    for index in range(600):
        content_key = plans[(index * 13) % len(plans)].key
        email = f"seed-real-{(index * 11) % 10 + 1:03d}{SEED_EMAIL_SUFFIX}"
        if (content_key, email) not in seen_history:
            seen_history.add((content_key, email))
            history_rows.append((content_key, email, (datetime.now(timezone.utc) - timedelta(hours=index * 3)).isoformat()))

    follow_rows = []
    seen_follows: set[tuple[str, str]] = set()
    for index in range(70):
        follower = f"seed-real-{index % 10 + 1:03d}{SEED_EMAIL_SUFFIX}"
        target = f"seed-real-{(index * 5 + 3) % 10 + 1:03d}{SEED_EMAIL_SUFFIX}"
        if follower != target and (follower, target) not in seen_follows:
            seen_follows.add((follower, target))
            follow_rows.append((follower, target, (datetime.now(timezone.utc) - timedelta(days=index % 25)).isoformat()))

    notification_rows = []
    for index in range(90):
        recipient = f"seed-real-{index % 10 + 1:03d}{SEED_EMAIL_SUFFIX}"
        sender = f"seed-real-{(index * 7 + 1) % 10 + 1:03d}{SEED_EMAIL_SUFFIX}"
        target_key = plans[index % len(plans)].key
        channel = "like" if index % 3 == 0 else "reply" if index % 3 == 1 else "system"
        notice_type = "like" if channel == "like" else "comment" if channel == "reply" else "system"
        notification_rows.append((recipient, sender, target_key, notice_type, channel, "新的本地测试通知", "用于消息中心与未读状态测试。", index % 4 == 0, (datetime.now(timezone.utc) - timedelta(hours=index * 2)).isoformat()))

    collection_rows = []
    for index in range(10):
        email = f"seed-real-{index + 1:03d}{SEED_EMAIL_SUFFIX}"
        collection_rows.append((email, "默认原创收藏", "original", True, False, 0, "本地测试默认收藏集"))
        collection_rows.append((email, "默认二创收藏", "fanwork", True, False, 0, "本地测试默认收藏集"))
        collection_rows.append((email, f"灵感整理 {index + 1:02d}", "original" if index % 2 == 0 else "fanwork", False, index % 3 == 0, 1, "用于列表、公开与私有状态测试。"))

    series_rows = []
    for index in range(6):
        owner = f"seed-real-{index % 10 + 1:03d}{SEED_EMAIL_SUFFIX}"
        series_rows.append((f"系列灵感 {index + 1:02d}", "连续创作与排序展示的本地测试系列。", owner, "original" if index % 2 == 0 else "fanwork", plans[index * 3].key))

    return f"""
BEGIN;
CREATE TEMP TABLE seed_users (
  email TEXT, password_hash TEXT, username TEXT, avatar_url TEXT, bio TEXT, support_info JSONB, reputation INT, role TEXT
) ON COMMIT DROP;
INSERT INTO seed_users VALUES\n{user_rows};
INSERT INTO users (email, password_hash, username, avatar_url, bio, support_info, reputation, role, email_verified_at, preferred_locale)
SELECT email, password_hash, username, avatar_url, bio, support_info, reputation, role, NOW(), 'zh-CN'
FROM seed_users su
WHERE NOT EXISTS (SELECT 1 FROM users u WHERE u.email = su.email)
ON CONFLICT (email) DO NOTHING;

CREATE TEMP TABLE seed_ips (
  slug TEXT, name TEXT, description TEXT, cover_url TEXT, category TEXT, creator_idx INT
) ON COMMIT DROP;
INSERT INTO seed_ips VALUES\n{ip_rows};
INSERT INTO ips (slug, name, description, cover_url, category, creator_id, status)
SELECT si.slug, si.name, si.description, si.cover_url, si.category, u.id, 'approved'
FROM seed_ips si
JOIN users u ON u.email = CONCAT('seed-real-', LPAD(si.creator_idx::TEXT, 3, '0'), '{SEED_EMAIL_SUFFIX}')
ON CONFLICT (slug) DO NOTHING;

CREATE TEMP TABLE seed_content (
  seed_key TEXT, title TEXT, description TEXT, author_email TEXT, zone TEXT, ip_slug TEXT,
  category TEXT, content_type TEXT, cover_url TEXT, status TEXT, is_public BOOLEAN, allow_copy BOOLEAN,
  view_count BIGINT, download_count INT, hot_score DOUBLE PRECISION, rating_score DOUBLE PRECISION,
  created_at TIMESTAMPTZ, content_id BIGINT
) ON COMMIT DROP;
INSERT INTO seed_content (
  seed_key, title, description, author_email, zone, ip_slug, category, content_type, cover_url,
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
\\pset tuples_only on
SELECT 'users=' || count(*) FROM users WHERE support_info @> {marker}::jsonb OR email = '{ADMIN_EMAIL}';
SELECT 'admin=' || count(*) FROM users WHERE email = '{ADMIN_EMAIL}' AND role = 'admin';
SELECT 'ips=' || count(*) FROM ips WHERE slug LIKE '{SEED_IP_SLUG_PREFIX}%';
SELECT 'content=' || count(*) FROM content_items ci JOIN users u ON u.id = ci.author_id WHERE u.support_info @> {marker}::jsonb;
SELECT 'published_public=' || count(*) FROM content_items ci JOIN users u ON u.id = ci.author_id WHERE u.support_info @> {marker}::jsonb AND ci.status = 'published' AND ci.is_public;
SELECT 'covers_local=' || count(*) FROM content_items ci JOIN users u ON u.id = ci.author_id WHERE u.support_info @> {marker}::jsonb AND ci.cover_image_url LIKE '/seed-media/real/%';
SELECT 'comments=' || count(*) FROM comments c JOIN users u ON u.id = c.author_id WHERE u.support_info @> {marker}::jsonb;
SELECT 'reactions=' || count(*) FROM reactions r JOIN users u ON u.id = r.user_id WHERE u.support_info @> {marker}::jsonb;
SELECT 'favorites=' || count(*) FROM favorites f JOIN users u ON u.id = f.user_id WHERE u.support_info @> {marker}::jsonb;
SELECT 'history=' || count(*) FROM browse_history bh JOIN users u ON u.id = bh.user_id WHERE u.support_info @> {marker}::jsonb;
SELECT 'notifications=' || count(*) FROM notifications n JOIN users u ON u.id = n.user_id WHERE u.support_info @> {marker}::jsonb;
SELECT 'collections=' || count(*) FROM collections c JOIN users u ON u.id = c.user_id WHERE u.support_info @> {marker}::jsonb;
SELECT 'series=' || count(*) FROM content_series cs JOIN users u ON u.id = cs.owner_id WHERE u.support_info @> {marker}::jsonb;
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
        print(f"  {key}: {metrics[key]}")
    minimums = {"users": 11, "admin": 1, "ips": 6, "content": 25, "published_public": 25, "covers_local": 25, "comments": 200}
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
            print("Removed only real-data seed users and seed media.")
        elif args.command == "verify":
            verify(dry_run=args.dry_run)
        else:
            execute_sql(build_cleanup_sql(), dry_run=args.dry_run)
            ensure_media(dry_run=args.dry_run)
            execute_sql(build_seed_sql(), dry_run=args.dry_run)
            verify(dry_run=args.dry_run)
            print("Real open-licensed seed data is ready. No OSS or moderation services were used.")
            print("Accounts: admin@omnicraft.com / Admin123456 · seed-real-001@omnicraft.local / Seed123456 ...")
    except (OSError, RuntimeError, subprocess.SubprocessError) as error:
        print(f"ERROR: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
