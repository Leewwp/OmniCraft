#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Unit tests for golden_set_draft (rule compliance + invariants)."""
from __future__ import annotations

import json
import os
import random
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import golden_set_draft as gs  # noqa: E402
import corpus_lib as lib  # noqa: E402


def find_corpus_index() -> str:
    here = os.path.abspath(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", ".."))
    cand = os.path.join(here, "artifacts", "corpus-v2", "index.jsonl")
    if not os.path.exists(cand):
        cand = os.path.join(os.path.dirname(here), "OmniCraft", "artifacts", "corpus-v2", "index.jsonl")
    return cand if os.path.exists(cand) else ""


class TitleToolsTests(unittest.TestCase):
    def test_strip_book_marks(self):
        self.assertEqual(gs.strip_title_marks("《原神：风起》"), "原神：风起")

    def test_title_core_removes_notes_and_ip_prefix(self):
        self.assertEqual(gs.title_core("《原神·层岩巨渊采矿日志（附勘勘表）》"), "层岩巨渊采矿日志")

    def test_title_topic_takes_tail(self):
        self.assertEqual(gs.title_topic("《原神·潮汐法庭：最高审判官的七天》"), "最高审判官的七天")


class NoCopyRuleTests(unittest.TestCase):
    def test_full_title_rejected(self):
        item = {"title": "《原神：风起地的协奏曲（全五章）》"}
        self.assertTrue(gs.violates_no_copyrule("请找到原神：风起地的协奏曲（全五章）这篇", item))

    def test_full_core_phrase_rejected(self):
        item = {"title": "《潮汐法庭：最高审判官的七天》"}
        # the full core phrase (after notes/ip prefix removal) is banned
        self.assertTrue(gs.violates_no_copyrule("想看潮汐法庭：最高审判官的七天这篇", item))
        # a partial tail phrase alone is acceptable (proper nouns may survive)
        self.assertFalse(gs.violates_no_copyrule("想看最高审判官的七天题材的", item))

    def test_paraphrase_allowed(self):
        item = {"title": "《潮汐法庭：最高审判官的七天》"}
        self.assertFalse(gs.violates_no_copyrule("有没有讲审判官日常的潮汐法庭同人", item))


class QuotaTests(unittest.TestCase):
    def test_layer_quota_language_budget(self):
        quota = gs.LayerQuota("exact")
        self.assertEqual(quota.total, gs.QUOTA_EXACT)
        zh_item = {"language": "zh", "temperature": "hot"}
        en_item = {"language": "en", "temperature": "hot"}
        for _ in range(45):
            self.assertTrue(quota.take(zh_item))
            quota.commit(zh_item)
        self.assertFalse(quota.take(zh_item))  # zh budget exhausted
        self.assertTrue(quota.take(en_item))
        quota.commit(en_item)
        self.assertEqual(quota.lang["en"], 12)

    def test_planned_totals_meet_floors(self):
        for lang, floor in gs.LANG_FLOOR.items():
            planned = sum(q[lang] for q in gs.LAYER_LANG_QUOTA.values())
            self.assertGreaterEqual(planned, floor, lang)
        self.assertEqual(sum(sum(q.values()) for q in gs.LAYER_LANG_QUOTA.values()), gs.N_TOTAL)
        planned_cold = sum(gs.LAYER_COLD_TARGET.values())
        self.assertGreaterEqual(planned_cold, gs.COLD_FLOOR)


class EnsureTitleTests(unittest.TestCase):
    def test_empty_title_synthesizes_from_body(self):
        title = lib.ensure_title("", "风花节前两周，西风教堂的管风琴哑了三个音。" + "正文" * 100, "k")
        self.assertTrue(title)
        self.assertLessEqual(len(title), 40)

    def test_keeps_existing_title(self):
        self.assertEqual(lib.ensure_title("已有标题", "正文", "k"), "已有标题")

    def test_fallback_when_body_blank(self):
        self.assertEqual(lib.ensure_title("", "  ", "k1"), "untitled k1")


class GenerationTests(unittest.TestCase):
    """End-to-end over the real corpus index (deterministic, seed-fixed)."""

    @classmethod
    def setUpClass(cls):
        path = find_corpus_index()
        if not path:
            raise unittest.SkipTest("corpus index not present")
        cls.rows = [json.loads(l) for l in open(path, encoding="utf-8") if l.strip()]

    def test_generate_meets_all_invariants(self):
        cases = gs.generate(self.rows)
        errors = gs.validate(cases)
        self.assertEqual(errors, [])

    def test_generate_is_deterministic(self):
        a = gs.generate(self.rows)
        b = gs.generate(self.rows)
        self.assertEqual(
            [c["case_key"] for c in a], [c["case_key"] for c in b]
        )
        self.assertEqual([c["query"] for c in a], [c["query"] for c in b])

    def test_semantic_queries_respect_no_copy(self):
        cases = gs.generate(self.rows)
        by_key = {r["corpus_item_key"]: r for r in self.rows}
        for case in cases:
            if case["classification"]["primary_layer"] != "semantic":
                continue
            # rule 5 binds against the primary target (expected citation);
            # recommendation siblings are additional answers, not query sources
            for key in case["expected_citation_keys"]:
                item = by_key.get(key)
                if item:
                    self.assertFalse(
                        gs.violates_no_copyrule(case["query"], item),
                        "query copied title phrase: %s" % case["query"],
                    )

    def test_no_answer_has_distractors_and_empty_relevant(self):
        cases = gs.generate(self.rows)
        no_answer = [c for c in cases if c["classification"]["primary_layer"] == "no_answer"]
        self.assertEqual(len(no_answer), gs.QUOTA_NO_ANSWER)
        for case in no_answer:
            self.assertEqual(case["relevant_refs"], [])
            self.assertTrue(case["forbidden_keys"], "missing distractors")

    def test_visibility_forbidden_targets_restricted(self):
        cases = gs.generate(self.rows)
        by_key = {r["corpus_item_key"]: r for r in self.rows}
        vis = [c for c in cases if c["classification"]["primary_layer"] == "visibility"]
        self.assertEqual(len(vis), gs.QUOTA_VISIBILITY)
        for case in vis:
            for key in case["forbidden_keys"]:
                self.assertIn(by_key[key]["visibility"], ("fans_only", "private"))

    def test_ip_localized_cases_cover_290_ip_search(self):
        cases = gs.generate(self.rows)
        scoped = [c for c in cases if c["classification"]["ip_scope"] == "ip_localized"]
        self.assertGreaterEqual(len(scoped), 16)
        ips = {c["classification"]["ip"] for c in scoped}
        self.assertEqual(len(ips), 16)


if __name__ == "__main__":
    unittest.main()
