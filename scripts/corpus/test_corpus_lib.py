#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Unit tests for corpus_lib (pure helpers) -- python3 -m unittest discover."""
from __future__ import annotations

import json
import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import corpus_lib as lib  # noqa: E402


SAMPLE_ITEM = {
    "corpus_item_key": "c2-ip01-b01-001",
    "ip": "原神",
    "category": "longform",
    "title": "《原神：风起地的协奏曲（全五章）》",
    "visibility": "public",
    "language": "zh",
    "temperature": "hot",
    "title_form": "exact",
    "version_count": 1,
    "published_at": "2026-03-15T07:12:00+08:00",
    "author_id": "a01",
    "word_count": 1786,
}


class VisibilityTests(unittest.TestCase):
    def test_public_is_public(self):
        self.assertTrue(lib.visibility_to_is_public("public"))

    def test_fans_only_collapses_to_private(self):
        # platform has no fans_only runtime carrier; both restricted labels
        # map to is_public=false and the label survives in tags
        self.assertFalse(lib.visibility_to_is_public("fans_only"))
        self.assertFalse(lib.visibility_to_is_public("private"))

    def test_unknown_label_is_restricted(self):
        self.assertFalse(lib.visibility_to_is_public(""))


class TagTests(unittest.TestCase):
    def test_tags_contain_idempotency_anchor(self):
        tags = lib.build_tags(SAMPLE_ITEM)
        self.assertIn("c2:c2-ip01-b01-001", tags)
        self.assertIn("原神", tags)
        self.assertIn("公开", tags)

    def test_tags_clip_to_50_chars(self):
        item = dict(SAMPLE_ITEM, corpus_item_key="c2-" + "x" * 200)
        for tag in lib.build_tags(item):
            self.assertLessEqual(len(tag), 50)

    def test_tags_dedupe(self):
        tags = lib.build_tags(SAMPLE_ITEM)
        self.assertEqual(len(tags), len(set(tags)))


class CategoryTests(unittest.TestCase):
    def test_known_ip_slugs(self):
        self.assertEqual(lib.ip_category_slug("原神"), "gaming")
        self.assertEqual(lib.ip_category_slug("哈利·波特"), "film_tv")
        self.assertEqual(lib.ip_category_slug("全职高手"), "literature")

    def test_unknown_ip_falls_back(self):
        self.assertEqual(lib.ip_category_slug("不存在的IP"), "other")


class CallbackTests(unittest.TestCase):
    def test_checksum_matches_handler_contract(self):
        # handler: sha256Hex(uid + seed + content)
        import hashlib

        uid, seed, content = "uid-1", "seed-2", '{"dataId":"content:9"}'
        expect = hashlib.sha256((uid + seed + content).encode()).hexdigest()
        self.assertEqual(lib.ai_callback_checksum(uid, seed, content), expect)

    def test_checksum_changes_with_content(self):
        a = lib.ai_callback_checksum("u", "s", "a")
        b = lib.ai_callback_checksum("u", "s", "b")
        self.assertNotEqual(a, b)

    def test_stable_task_id(self):
        self.assertEqual(lib.stable_task_id("c2-ip01-b01-001", 2), "corpusv2-c2-ip01-b01-001-v2")


class CheckpointTests(unittest.TestCase):
    def test_roundtrip(self):
        st = lib.ItemState(key="k", content_id=42, versions_done=2, published=True, pr_ids=[7, 8], indexed=True)
        revived = lib.ItemState.from_json(st.to_json())
        self.assertEqual(revived.content_id, 42)
        self.assertEqual(revived.versions_done, 2)
        self.assertEqual(revived.pr_ids, [7, 8])
        self.assertTrue(revived.done)

    def test_load_checkpoint_last_wins(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = os.path.join(tmp, "cp.jsonl")
            with open(path, "w", encoding="utf-8") as fh:
                fh.write(lib.ItemState(key="k", content_id=1).to_json() + "\n")
                fh.write(lib.ItemState(key="k", content_id=2, indexed=True).to_json() + "\n")
            states = lib.load_checkpoint(path)
            self.assertEqual(states["k"].content_id, 2)
            self.assertTrue(states["k"].done)

    def test_load_missing_file(self):
        states = lib.load_checkpoint("/nonexistent/path.jsonl")
        self.assertEqual(states, {})


class BreakerTests(unittest.TestCase):
    def test_consecutive_trip(self):
        breaker = lib.CircuitBreaker(max_consecutive=5, min_samples=1000)
        for _ in range(4):
            breaker.record(False)
        self.assertFalse(breaker.tripped)
        breaker.record(False)
        self.assertTrue(breaker.tripped)
        self.assertIn("consecutive", breaker.reason or "")

    def test_ratio_trip(self):
        breaker = lib.CircuitBreaker(max_error_ratio=0.02, min_samples=50, max_consecutive=1000)
        for _ in range(49):
            breaker.record(True)
        self.assertFalse(breaker.tripped)
        breaker.record(False)  # 1/50 = 2% -> not strictly above
        self.assertFalse(breaker.tripped)
        breaker.record(False)  # 2/51 > 2%
        self.assertTrue(breaker.tripped)
        self.assertIn("error ratio", breaker.reason or "")

    def test_reset_on_success(self):
        breaker = lib.CircuitBreaker(max_consecutive=3, min_samples=1000)
        breaker.record(False)
        breaker.record(False)
        breaker.record(True)
        self.assertFalse(breaker.tripped)


class DescriptionTests(unittest.TestCase):
    def test_strips_markdown_and_clips(self):
        body = "# 标题\n\n**加粗**段落`code`[链接](x)继续" + "很长的正文" * 100
        desc = lib.build_description(body, limit=50)
        self.assertNotIn("#", desc)
        self.assertNotIn("**", desc)
        self.assertLessEqual(len(desc), 50)

    def test_plain_body_kept(self):
        self.assertEqual(lib.build_description("简单正文"), "简单正文")


class FixtureSqlTests(unittest.TestCase):
    def test_fixture_rows_cover_authors_admin_viewer(self):
        authors = [{"author_id": "a01", "handle": "星河拾遗"}]
        rows = lib.fixture_user_rows(authors)
        emails = [r[0] for r in rows]
        self.assertIn("a01@corpus.omnicraft.local", emails)
        self.assertIn("admin@corpus.omnicraft.local", emails)
        self.assertIn("viewer-anon@corpus.omnicraft.local", emails)

    def test_sql_quotes_handles(self):
        authors = [{"author_id": "a01", "handle": "O'Brien's"}]
        sql = lib.fixture_users_sql(lib.fixture_user_rows(authors))
        self.assertIn("'O''Brien''s'", sql)
        self.assertIn("ON CONFLICT (email) DO NOTHING", sql)
        self.assertIn("role = 'admin'", sql)

    def test_support_info_json_valid(self):
        rows = lib.fixture_user_rows([{"author_id": "a01", "handle": "x"}])
        parsed = json.loads(rows[0][2])
        self.assertEqual(parsed["namespace"], "corpus-v2")


if __name__ == "__main__":
    unittest.main()
