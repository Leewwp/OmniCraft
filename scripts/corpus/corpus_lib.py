#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Shared pure logic for the corpus-v2 injection pipeline (#291 CORPUS-01).

Design notes (see artifacts/lane1-status.md for the full contract survey):

- Content publication goes through the REAL pipeline: REST form validation ->
  moderation gate (pending + content.review queue) -> operator callback via the
  production /internal/ai-callback entry (SHA256 checksum auth) -> published +
  outbox content.published -> relay -> indexer worker -> chunker + embed ->
  dual-path index (rag_chunks/chunk_embeddings + OpenSearch).
- Body text lives in content_versions. The REST surface has no version-upload
  endpoint, so the initial version uses the repository's existing ops tool
  (backend/cmd/version_seed). Later versions use the real PR flow
  (POST /pr -> POST /pr/:id/merge) followed by PATCH /contents/:id (title) to
  emit content.updated and refresh the projection.
- Platform visibility is the is_public boolean only (no fans_only runtime
  semantics): public -> is_public=true, fans_only/private -> is_public=false.
  The corpus visibility label is preserved in content tags for golden-set use.
- corpus_item_key is the stable identity. A `c2:<key>` content tag is the
  idempotency anchor so a lost checkpoint cannot double-inject an item.

This module is intentionally dependency-free (stdlib only, Python 3.9+) so the
pure helpers stay unit-testable without a live stack.
"""
from __future__ import annotations

import hashlib
import json
import re
import time
from dataclasses import dataclass, field
from typing import Any, Dict, List, Optional, Tuple

CORPUS_EMAIL_DOMAIN = "corpus.omnicraft.local"
CORPUS_EMAIL_SUFFIX = "@" + CORPUS_EMAIL_DOMAIN
CORPUS_SUPPORT_MARKER = "corpus-v2"
IDEMPOTENCY_TAG_PREFIX = "c2:"
# Fixed bcrypt password hash for corpus fixture users (bcrypt cost 12 of
# "CorpusV2#2026", generated once; the plaintext lives here for the injector's
# REST logins).
FIXTURE_PASSWORD_HASH = "$2b$12$ZHaA7d8luEG3uvw6BQG.QeANbHfylOi62E2VlBV9fmqJFtnu.tu1y"
FIXTURE_PASSWORD = "CorpusV2#2026"
FIXTURE_REPUTATION = 80

# fanwork zone category domain comes from the categories table
# (zone=fanwork, level=ip_category): gaming/anime/music/film_tv/literature/other.
IP_CATEGORY: Dict[str, str] = {
    "原神": "gaming",
    "崩坏：星穹铁道": "gaming",
    "王者荣耀": "gaming",
    "西游记（孙悟空）": "anime",
    "哪吒/封神宇宙": "anime",
    "全职高手": "literature",
    "诡秘之主": "literature",
    "魔道祖师": "literature",
    "天官赐福": "literature",
    "盗墓笔记": "literature",
    "罗小黑战记": "anime",
    "哈利·波特": "film_tv",
    "双城之战": "anime",
    "火影忍者": "anime",
    "海贼王": "anime",
    "宝可梦": "gaming",
}

# Corpus category -> human tag (kept in content tags; the DB category column
# uses the fanwork ip_category slug above).
CATEGORY_LABEL = {
    "longform": "长篇同人",
    "shortform": "短篇",
    "settings": "设定集",
    "discussion": "讨论首帖",
}

LANGUAGE_LABEL = {"zh": "中文", "en": "英文", "mixed": "中英混合"}
TEMPERATURE_LABEL = {"hot": "热门", "cold": "冷门长尾"}
VISIBILITY_LABEL = {"public": "公开", "fans_only": "仅粉丝", "private": "私密"}
TAG_MAX_LEN = 50  # content_tags.tag VARCHAR(50)


def visibility_to_is_public(corpus_visibility: str) -> bool:
    """Map the corpus visibility label onto the platform boolean.

    The platform visibility scope is `is_public OR author` only; fans_only has
    no runtime carrier, so restricted labels collapse to is_public=false and
    the original label is preserved via tags.
    """
    return corpus_visibility == "public"


def ip_category_slug(ip_name: str) -> str:
    return IP_CATEGORY.get(ip_name, "other")


def clip_tag(tag: str) -> Optional[str]:
    tag = tag.strip()
    if not tag:
        return None
    if len(tag) > TAG_MAX_LEN:
        tag = tag[:TAG_MAX_LEN]
    return tag


def build_tags(item: Dict[str, Any]) -> List[str]:
    """Deterministic content tags: [IP, corpus category, language, temperature,
    visibility label, idempotency anchor]."""
    tags: List[str] = [
        item.get("ip", ""),
        CATEGORY_LABEL.get(item.get("category", ""), ""),
        LANGUAGE_LABEL.get(item.get("language", ""), ""),
        TEMPERATURE_LABEL.get(item.get("temperature", ""), ""),
        VISIBILITY_LABEL.get(item.get("visibility", ""), item.get("visibility", "")),
        IDEMPOTENCY_TAG_PREFIX + item["corpus_item_key"],
    ]
    out: List[str] = []
    for tag in tags:
        clipped = clip_tag(tag)
        if clipped and clipped not in out:
            out.append(clipped)
    return out


_MD_STRIP_RE = re.compile(r"[#*`>\-\[\]\(\)!_|]+")


def ensure_title(title: str, body_md: str, key: str, limit: int = 40) -> str:
    """Platform titles are required (min=1); 68 corpus discussion items are
    deliberately untitled (spec section 1 discussion short posts). Synthesize a
    deterministic lead-sentence title so the untitled-post sample still enters
    the real pipeline; the golden-set draft keeps title_form=fuzzy semantics."""
    title = (title or "").strip()
    if title:
        return title
    text = re.sub(r"\s+", " ", _MD_STRIP_RE.sub("", body_md)).strip()
    if len(text) > limit:
        text = text[:limit]
    return text or ("untitled %s" % key)


def build_description(body_md: str, limit: int = 120) -> str:
    """Plain-text summary of the body for the content list surface."""
    text = _MD_STRIP_RE.sub("", body_md)
    text = re.sub(r"\s+", " ", text).strip()
    if len(text) > limit:
        text = text[:limit]
    return text


def ai_callback_checksum(uid: str, seed: str, content_json: str) -> str:
    """SHA256(uid + seed + content) -- the production inbound-auth contract."""
    return hashlib.sha256((uid + seed + content_json).encode("utf-8")).hexdigest()


def build_ai_callback_payload(content_id: int, task_id: str, suggestion: str = "pass") -> Tuple[str, str]:
    """Return (form_content_json, checksum) for POST /internal/ai-callback."""
    content = json.dumps(
        {
            "dataId": "content:%d" % content_id,
            "taskId": task_id,
            "code": 200,
            "message": "OK",
            "results": [{"scene": "comment", "label": "non_ad", "suggestion": suggestion}],
        },
        separators=(",", ":"),
        ensure_ascii=False,
    )
    return content, task_id  # checksum computed separately (needs uid/seed)


def stable_task_id(corpus_item_key: str, version: int = 1) -> str:
    return "corpusv2-%s-v%d" % (corpus_item_key, version)


@dataclass
class ItemState:
    """Checkpoint state machine for one corpus item.

    done=True means fully indexed (final version projected); the injector
    skips done items on resume.
    """

    key: str
    content_id: Optional[int] = None
    versions_done: int = 0  # number of content_versions rows created
    published: bool = False
    pr_ids: List[int] = field(default_factory=list)
    indexed: bool = False
    error: Optional[str] = None

    @property
    def done(self) -> bool:
        return self.indexed

    def to_json(self) -> str:
        return json.dumps(
            {
                "key": self.key,
                "content_id": self.content_id,
                "versions_done": self.versions_done,
                "published": self.published,
                "pr_ids": self.pr_ids,
                "indexed": self.indexed,
                "error": self.error,
            },
            ensure_ascii=False,
            separators=(",", ":"),
        )

    @classmethod
    def from_json(cls, line: str) -> "ItemState":
        raw = json.loads(line)
        return cls(
            key=raw["key"],
            content_id=raw.get("content_id"),
            versions_done=raw.get("versions_done", 0),
            published=raw.get("published", False),
            pr_ids=raw.get("pr_ids", []),
            indexed=raw.get("indexed", False),
            error=raw.get("error"),
        )


def load_checkpoint(path: str) -> Dict[str, ItemState]:
    states: Dict[str, ItemState] = {}
    try:
        with open(path, "r", encoding="utf-8") as fh:
            for line in fh:
                line = line.strip()
                if not line:
                    continue
                state = ItemState.from_json(line)
                states[state.key] = state  # later lines win (resume log)
    except FileNotFoundError:
        pass
    return states


class CircuitBreaker:
    """Stop the run on consecutive failures or an error ratio above threshold.

    spec 2.1: 错误率阈值熔断（>2% 停止）；连续失败快速停机防止雪崩。
    """

    def __init__(self, max_error_ratio: float = 0.02, min_samples: int = 50, max_consecutive: int = 5):
        self.max_error_ratio = max_error_ratio
        self.min_samples = min_samples
        self.max_consecutive = max_consecutive
        self.total = 0
        self.errors = 0
        self.consecutive = 0

    def record(self, ok: bool) -> None:
        self.total += 1
        if ok:
            self.consecutive = 0
        else:
            self.errors += 1
            self.consecutive += 1

    @property
    def tripped(self) -> bool:
        if self.consecutive >= self.max_consecutive:
            return True
        if self.total >= self.min_samples and self.errors / max(self.total, 1) > self.max_error_ratio:
            return True
        return False

    @property
    def reason(self) -> Optional[str]:
        if self.consecutive >= self.max_consecutive:
            return "consecutive failures: %d" % self.consecutive
        if self.total >= self.min_samples and self.errors / max(self.total, 1) > self.max_error_ratio:
            return "error ratio %.3f > %.3f (%d/%d)" % (
                self.errors / max(self.total, 1),
                self.max_error_ratio,
                self.errors,
                self.total,
            )
        return None


class RateLimiter:
    """Minimum-interval pacing (target rps from spec: 2-5)."""

    def __init__(self, rps: float = 3.0):
        self.interval = 1.0 / rps if rps > 0 else 0.0
        self._last = 0.0

    def wait(self) -> None:
        now = time.monotonic()
        delta = self._last + self.interval - now
        if delta > 0:
            time.sleep(delta)
        self._last = time.monotonic()


def sql_quote(value: str) -> str:
    return "'" + value.replace("'", "''") + "'"


def fixture_user_rows(authors: List[Dict[str, Any]], admin_handle: str = "万象管理员") -> List[Tuple[str, str, str]]:
    """(email, username, support_info_json) fixture tuples for direct users
    insert. Passwords use the fixed seed bcrypt hash; reputation/verification
    satisfy the interaction gate (email verified + reputation >= threshold)."""
    rows: List[Tuple[str, str, str]] = []
    support = json.dumps({"namespace": CORPUS_SUPPORT_MARKER}, ensure_ascii=False)
    for author in authors:
        email = author["author_id"] + CORPUS_EMAIL_SUFFIX
        rows.append((email, author["handle"], support))
    rows.append(("admin" + CORPUS_EMAIL_SUFFIX, admin_handle, support))
    rows.append(("viewer-anon" + CORPUS_EMAIL_SUFFIX, "路人观测员", support))
    return rows


def fixture_users_sql(rows: List[Tuple[str, str, str]]) -> str:
    values = []
    for email, username, support in rows:
        values.append(
            "(%s, %s, %s, %s, %s, %d, 'user', NOW())"
            % (sql_quote(email), sql_quote(FIXTURE_PASSWORD_HASH), sql_quote(username), sql_quote(""), sql_quote(support), FIXTURE_REPUTATION)
        )
    admin_email = "admin" + CORPUS_EMAIL_SUFFIX
    # admin role for the corpus admin account (second to last row convention:
    # fixture_user_rows appends admin then viewer)
    stmt = (
        "INSERT INTO users (email, password_hash, username, avatar_url, support_info, reputation, role, email_verified_at)\n"
        "VALUES\n" + ",\n".join(values)
        + "\nON CONFLICT (email) DO NOTHING;\n"
        + "UPDATE users SET role = 'admin' WHERE email = %s AND role <> 'admin';\n" % sql_quote(admin_email)
    )
    return stmt
