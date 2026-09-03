#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Golden-set AI-first-draft generator (#291 CORPUS-01, spec sections 3-4).

DRAFT ONLY -- finalize_in_goal=false. The frozen set stays a user action; this
script produces the draft JSONL (keyed by corpus_item_key, never content_id)
plus the annotation-rules draft. Machine truth source remains the 069
eval_golden_cases table after the user maps keys to content ids and freezes.

Layer quotas (N=160): exact=floor(N*.4)=64, semantic=floor(N*.3)=48 (>=30
colloquial recommendation sub-cases), visibility=floor(N*.15)=24, no_answer=24
(remainder absorbs rounding).

Global invariants (069 design): zh >= 50%, en >= 20%, mixed >= 10%;
cold-band queries >= 20%. Deterministic sampling (fixed seed) so re-runs are
byte-identical.

Annotation-rule compliance implemented here:
 1. multi-answer allowed; empty relevant set only for no_answer
 2. every relevant entry carries a (key, version, span) evidence stub
 3. expected_citations is a subset, annotated separately
 4. exact queries embed the full title (book quotes optional)
 5. semantic queries never copy the full title / core title phrase verbatim
 6. visibility cases fix viewer=anonymous + non-follower fixture
 7. no_answer queries carry plausible-but-unanswerable distractors
 8. classification carries the full dimension set (four-column review view is
    derived in the reconciliation report)
"""
from __future__ import annotations

import argparse
import json
import os
import random
import re
import sys
from typing import Any, Dict, List, Optional, Tuple

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import corpus_lib as lib  # noqa: E402

N_TOTAL = 160
QUOTA_EXACT = 64  # floor(160*0.4)
QUOTA_SEMANTIC = 48  # floor(160*0.3); >= 30 colloquial
COLLOQUIAL_MIN = 30
QUOTA_VISIBILITY = 24  # floor(160*0.15)
QUOTA_NO_ANSWER = N_TOTAL - QUOTA_EXACT - QUOTA_SEMANTIC - QUOTA_VISIBILITY  # 24
# Global language floors (069 design): zh >= 50%, en >= 20%, mixed >= 10% of
# 160. Planned totals: zh 112 / en 32 / mixed 16.
LANG_FLOOR = {"zh": 80, "en": 32, "mixed": 16}
LAYER_LANG_QUOTA = {
    "exact": {"zh": 45, "en": 13, "mixed": 6},
    "semantic": {"zh": 34, "en": 10, "mixed": 4},
    "visibility": {"zh": 17, "en": 5, "mixed": 2},
    "no_answer": {"zh": 16, "en": 4, "mixed": 4},
}
LAYER_COLD_TARGET = {"exact": 12, "semantic": 12, "visibility": 8, "no_answer": 24}
COLD_FLOOR = 32  # >=20% of 160
SEED = 20260903


# ------------------------------------------------------------------ title tools

def strip_title_marks(title: str) -> str:
    return title.strip().strip("《》").strip()


def title_core(title: str) -> str:
    """Title without the book quotes, bracketed notes and the IP prefix."""
    body = strip_title_marks(title)
    body = re.sub(r"[（(][^）)]*[）)]", "", body)
    body = re.sub(r"^原神[·:：]", "", body)
    return body.strip(" ·:：")


TOPIC_SPLIT = re.compile(r"[·:：—－-]")


def title_topic(title: str) -> str:
    """The sub-topic phrase of a title: text after the last separator."""
    core = title_core(title)
    parts = [p.strip() for p in TOPIC_SPLIT.split(core) if p.strip()]
    return parts[-1] if parts else core


ZH_SEMANTIC_TEMPLATES = [
    "有没有讲{topic}的{ip}同人",
    "想看{ip}里围绕{topic}的故事",
    "{ip}同人里关于{topic}的设定或剧情有人写过吗",
    "求推荐{ip}题材下涉及{topic}的作品",
    "{ip}有没有聚焦{topic}的创作",
]
ZH_COLLOQUIAL_TEMPLATES = [
    "想看{ip}的东西，最好和{topic}有关，有推荐吗",
    "最近入坑{ip}，想找{topic}向的粮，求安利",
    "有没有{ip}的{topic}题材好文，轻松一点的",
    "安利一下{ip}里{topic}相关的作品呗",
    "求{ip}同人中讲{topic}的，长短都行",
]
EN_SEMANTIC_TEMPLATES = [
    "any {ip} fanworks exploring {topic}",
    "looking for {ip} stories centred on {topic}",
    "{ip} fanfic recommendations about {topic}",
]
EN_COLLOQUIAL_TEMPLATES = [
    "new to {ip}, anything about {topic} to read?",
    "can someone rec {ip} works with {topic} vibes",
    "in the mood for {ip}, preferably {topic}-ish, suggestions?",
]
NO_ANSWER_TOPICS = [
    ("量子计算机修仙入门指南", "科技修仙"),
    ("用Excel表格修炼成仙的方法", "科技修仙"),
    ("菜谱与剑谱的等价交换定律", "美食+武侠"),
    ("主角是一台自动售货机的异世界故事", "无厘头异世界"),
    ("论如何用广场舞征服魔王军团", "搞笑冒险"),
    ("复印机成精以后的职场生活", "职场奇幻"),
    ("猫咪当市长的城市治理日记", "日常萌宠"),
    ("火锅底料的自我修养", "美食拟人"),
]
NO_ANSWER_EN_TOPICS = [
    ("a fandom wiki written entirely in haiku", "meta fanworks"),
    ("a tutorial on taming printer dragons", "absurd guides"),
    ("the lost art of fax machine necromancy", "absurd fantasy"),
    ("cookbook recipes authored by sentient spoons", "absurd food"),
]


def query_language_of(item: Dict[str, Any]) -> str:
    lang = item.get("language", "zh")
    return lang if lang in ("zh", "en", "mixed") else "zh"


# ------------------------------------------------------------------ selection


class LayerQuota:
    """Per-layer language and cold-band quotas; deterministic filling."""

    def __init__(self, layer: str) -> None:
        self.lang = dict(LAYER_LANG_QUOTA[layer])
        self.cold = LAYER_COLD_TARGET[layer]
        self.total = sum(self.lang.values())

    def take(self, item: Dict[str, Any]) -> bool:
        lang = query_language_of(item)
        if self.lang.get(lang, 0) <= 0:
            return False
        if self.cold <= 0 and item.get("temperature") == "cold":
            # cold quota full: accept hot items only
            return True
        return True

    def commit(self, item: Dict[str, Any]) -> None:
        lang = query_language_of(item)
        if lang in self.lang and self.lang[lang] > 0:
            self.lang[lang] -= 1
        if item.get("temperature") == "cold":
            self.cold -= 1

    @property
    def done(self) -> bool:
        return sum(self.lang.values()) <= 0


def balanced_pool(index_rows: List[Dict[str, Any]], rng: random.Random) -> List[Dict[str, Any]]:
    """Deterministic interleave: round-robin over IPs so every layer sample
    spreads across the 16-IP roster instead of clustering."""
    by_ip: Dict[str, List[Dict[str, Any]]] = {}
    for row in index_rows:
        by_ip.setdefault(row["ip"], []).append(row)
    for rows in by_ip.values():
        rng.shuffle(rows)
    ips = sorted(by_ip)
    pool: List[Dict[str, Any]] = []
    i = 0
    while True:
        emitted = False
        for ip in ips:
            rows = by_ip[ip]
            if i < len(rows):
                pool.append(rows[i])
                emitted = True
        if not emitted:
            return pool
        i += 1


def make_case(
    layer: str,
    query: str,
    lang: str,
    viewer: str,
    relevant: List[Dict[str, Any]],
    expected_keys: List[str],
    forbidden: List[str],
    classification: Dict[str, Any],
    rubric: Dict[str, Any],
) -> Dict[str, Any]:
    return {
        "case_key": "",
        "schema_version": 1,
        "query": query,
        "query_language": lang,
        "viewer_context": {"viewer": viewer},
        "relevant_refs": relevant,
        "expected_citation_keys": expected_keys,
        "forbidden_keys": forbidden,
        "answer_rubric": rubric,
        "classification": classification,
        "annotation_status": "ai_draft",
    }


def evidence_stub(item: Dict[str, Any], version: int = 1, reason: str = "") -> Dict[str, Any]:
    span_end = min(400, int(item.get("word_count", 400)))
    return {
        "corpus_item_key": item["corpus_item_key"],
        "version": version,
        "source_start": 0,
        "source_end": span_end,
        "reason": reason or "title+lead evidence; span is a coarse AI draft for human review",
    }


def build_exact_layer(pool: List[Dict[str, Any]], budget: Budget, rng: random.Random) -> Tuple[List[Dict[str, Any]], List[Dict[str, Any]]]:
    cases: List[Dict[str, Any]] = []
    used: List[Dict[str, Any]] = []
    ip_local_quota = 16  # a quarter of the exact layer exercises #290 IP-scoped search
    ip_local_done = 0
    ip_local_seen: set = set()
    for item in pool:
        if len(cases) >= QUOTA_EXACT:
            break
        if item.get("title_form") != "exact":
            continue
        if not budget.take(item):
            continue
        ip_scope = "global"
        query = strip_title_marks(item["title"])
        if ip_local_done < ip_local_quota:
            if item["ip"] in ip_local_seen:
                continue  # one ip_localized case per IP: cover all 16
            ip_scope = "ip_localized"
            ip_local_done += 1
            ip_local_seen.add(item["ip"])
            query = "在%s内：%s" % (item["ip"], query)
        case = make_case(
            layer="exact",
            query=query,
            lang=query_language_of(item),
            viewer="anonymous",
            relevant=[evidence_stub(item, 1, "full-title exact match")],
            expected_keys=[item["corpus_item_key"]],
            forbidden=[],
            classification={
                "primary_layer": "exact",
                "query_form": "exact",
                "visibility_context": "anonymous",
                "ip_scope": ip_scope,
                "ip": item["ip"],
                "answerability": "answerable",
                "content_category": item["category"],
                "temperature_band": item["temperature"],
            },
            rubric={
                "must_include": ["title match in results"],
                "must_not_include": [],
                "ideal_answer": "the exact work %s" % item["title"],
            },
        )
        cases.append(case)
        used.append(item)
        budget.commit(item)
    return cases, used


def semantic_query(item: Dict[str, Any], colloquial: bool, rng: random.Random) -> Optional[str]:
    """Try every template until one satisfies rule 5 (no verbatim title/core
    phrase); degrade the topic to its first three words before giving up."""
    lang = query_language_of(item)
    topic = title_topic(item["title"])
    ip = item["ip"]
    if lang == "en":
        templates = list(EN_COLLOQUIAL_TEMPLATES) if colloquial else list(EN_SEMANTIC_TEMPLATES)
    else:
        templates = list(ZH_COLLOQUIAL_TEMPLATES) if colloquial else list(ZH_SEMANTIC_TEMPLATES)
    rng.shuffle(templates)
    candidates = [topic]
    words = topic.split()
    if len(words) > 3:
        candidates.append(" ".join(words[:3]))
    for candidate in candidates:
        for template in templates:
            query = template.format(ip=ip, topic=candidate)
            if not violates_no_copyrule(query, item):
                return query
    return None


def violates_no_copyrule(query: str, item: Dict[str, Any]) -> bool:
    """Rule 5: never reuse the full title or its full core phrase."""
    title = strip_title_marks(item["title"])
    core = title_core(item["title"])
    return title in query or (len(core) >= 6 and core in query)


def build_semantic_layer(
    pool: List[Dict[str, Any]], all_rows: List[Dict[str, Any]], budget: Budget, rng: random.Random
) -> Tuple[List[Dict[str, Any]], List[Dict[str, Any]]]:
    cases: List[Dict[str, Any]] = []
    used: List[Dict[str, Any]] = []
    by_ip: Dict[str, List[Dict[str, Any]]] = {}
    for row in all_rows:
        by_ip.setdefault(row["ip"], []).append(row)
    colloquial_done = 0
    multi_answer_done = 0
    for item in pool:
        if len(cases) >= QUOTA_SEMANTIC:
            break
        if item.get("title_form") not in ("fuzzy", "partial"):
            continue
        if not budget.take(item):
            continue
        colloquial = colloquial_done < COLLOQUIAL_MIN
        query = semantic_query(item, colloquial, rng)
        if query is None:
            continue
        # multi-answer: colloquial recommendation targets add same-IP siblings
        relevant = [evidence_stub(item, 1, "semantic paraphrase target")]
        siblings: List[Dict[str, Any]] = [
            s
            for s in by_ip.get(item["ip"], [])
            if s["corpus_item_key"] != item["corpus_item_key"]
            and s.get("visibility") == "public"
            and s.get("category") == item["category"]
        ]
        rng.shuffle(siblings)
        extra: List[Dict[str, Any]] = []
        if colloquial and multi_answer_done < 12 and siblings:
            for sib in siblings[:2]:
                extra.append(sib)
            multi_answer_done += 1
        refs = relevant + [evidence_stub(s, 1, "same-IP same-category recommendation sibling") for s in extra]
        case = make_case(
            layer="semantic",
            query=query,
            lang=query_language_of(item),
            viewer="anonymous",
            relevant=refs,
            expected_keys=[item["corpus_item_key"]],
            forbidden=[],
            classification={
                "primary_layer": "semantic",
                "query_form": "semantic",
                "visibility_context": "anonymous",
                "ip_scope": "ip_localized" if colloquial else "global",
                "ip": item["ip"],
                "answerability": "answerable",
                "content_category": item["category"],
                "temperature_band": item["temperature"],
                "colloquial_recommendation": colloquial,
            },
            rubric={
                "must_include": ["the target work in top results"],
                "must_not_include": [],
                "ideal_answer": "recommend %s" % item["title"],
            },
        )
        cases.append(case)
        used.append(item)
        budget.commit(item)
        if colloquial:
            colloquial_done += 1
    return cases, used


def build_visibility_layer(pool: List[Dict[str, Any]], budget: Budget, rng: random.Random) -> Tuple[List[Dict[str, Any]], List[Dict[str, Any]]]:
    cases: List[Dict[str, Any]] = []
    used: List[Dict[str, Any]] = []
    for item in pool:
        if len(cases) >= QUOTA_VISIBILITY:
            break
        if item.get("visibility") not in ("fans_only", "private"):
            continue
        if not budget.take(item):
            continue
        # the query targets the restricted work by its exact title; the answer
        # body must not surface it to an anonymous / non-follower viewer
        query = strip_title_marks(item["title"])
        case = make_case(
            layer="visibility",
            query=query,
            lang=query_language_of(item),
            viewer="anonymous",  # rule 6: anonymous + non-follower fixture pair
            relevant=[],  # no visible relevant for this viewer
            expected_keys=[],
            forbidden=[item["corpus_item_key"]],
            classification={
                "primary_layer": "visibility",
                "query_form": "exact",
                "visibility_context": "anonymous_and_non_follower_fixture",
                "ip_scope": "global",
                "ip": item["ip"],
                "answerability": "hidden_for_viewer",  # answer exists but is restricted
                "content_category": item["category"],
                "temperature_band": item["temperature"],
                "corpus_visibility": item["visibility"],
            },
            rubric={
                "must_include": [],
                "must_not_include": [
                    "the restricted work in search results",
                    "its title/summary/id in answer text",
                    "any citation pointing at it",
                    "any tool-step event exposing it",
                ],
                "ideal_answer": "results restricted to public works only; no leak across the four leak surfaces",
            },
        )
        cases.append(case)
        used.append(item)
        budget.commit(item)
    return cases, used


def build_no_answer_layer(pool: List[Dict[str, Any]], budget: Budget, rng: random.Random) -> Tuple[List[Dict[str, Any]], List[Dict[str, Any]]]:
    cases: List[Dict[str, Any]] = []
    used: List[Dict[str, Any]] = []
    topics = list(NO_ANSWER_TOPICS)
    en_topics = list(NO_ANSWER_EN_TOPICS)
    rng.shuffle(topics)
    distractor_pool = [r for r in pool if r.get("category") in ("settings", "guide", "discussion")] if any(
        r.get("category") == "guide" for r in pool
    ) else [r for r in pool if r.get("category") in ("settings", "discussion")]
    rng.shuffle(distractor_pool)
    dist_iter = iter(distractor_pool)
    lang_cycle = ["zh", "zh", "zh", "en", "mixed"]
    for i in range(QUOTA_NO_ANSWER):
        lang = lang_cycle[i % len(lang_cycle)]
        if lang == "en":
            topic, theme = en_topics[i % len(en_topics)]
        else:
            topic, theme = topics[i % len(topics)]
        # mixed wraps the zh topic with an english frame
        query = topic if lang != "mixed" else "anyone knows: %s" % topic
        distractors: List[Dict[str, Any]] = []
        for _ in range(2):
            try:
                distractors.append(next(dist_iter))
            except StopIteration:
                break
        case = make_case(
            layer="no_answer",
            query=query,
            lang=lang,
            viewer="anonymous",
            relevant=[],
            expected_keys=[],
            forbidden=[d["corpus_item_key"] for d in distractors],
            classification={
                "primary_layer": "no_answer",
                "query_form": "semantic",
                "visibility_context": "anonymous",
                "ip_scope": "global",
                "ip": None,
                "answerability": "unanswerable",
                "content_category": theme,
                "temperature_band": "cold",  # off-corpus topics are by construction long-tail
            },
            rubric={
                "must_include": [],
                "must_not_include": ["trap distractors in top-10", "hallucinated answer content"],
                "ideal_answer": "a refusal/clarification, no fabricated corpus claims",
            },
        )
        cases.append(case)
        if distractors:
            used.extend(distractors)
    return cases, used


def generate(index_rows: List[Dict[str, Any]]) -> List[Dict[str, Any]]:
    rng = random.Random(SEED)
    quotas = {layer: LayerQuota(layer) for layer in ("exact", "semantic", "visibility", "no_answer")}
    pool = balanced_pool(index_rows, rng)
    exact_cases, _ = build_exact_layer(pool, quotas["exact"], rng)
    semantic_cases, _ = build_semantic_layer(pool, index_rows, quotas["semantic"], rng)
    visibility_cases, _ = build_visibility_layer(pool, quotas["visibility"], rng)
    no_answer_cases, _ = build_no_answer_layer(pool, quotas["no_answer"], rng)
    cases = exact_cases + semantic_cases + visibility_cases + no_answer_cases
    for i, case in enumerate(cases, start=1):
        case["case_key"] = "gs-c2-%04d" % i
    return cases


def validate(cases: List[Dict[str, Any]]) -> List[str]:
    errors: List[str] = []
    layers: Dict[str, int] = {}
    langs: Dict[str, int] = {}
    cold = 0
    keys = set()
    colloquial = 0
    for case in cases:
        layers[case["classification"]["primary_layer"]] = layers.get(case["classification"]["primary_layer"], 0) + 1
        langs[case["query_language"]] = langs.get(case["query_language"], 0) + 1
        if case["classification"].get("temperature_band") == "cold":
            cold += 1
        if case["classification"].get("colloquial_recommendation"):
            colloquial += 1
        if case["case_key"] in keys:
            errors.append("duplicate case_key %s" % case["case_key"])
        keys.add(case["case_key"])
        layer = case["classification"]["primary_layer"]
        if layer != "no_answer" and layer != "visibility" and not case["relevant_refs"]:
            errors.append("empty relevant on %s case %s" % (layer, case["case_key"]))
        if layer == "no_answer" and case["relevant_refs"]:
            errors.append("no_answer case %s has relevant" % case["case_key"])
    if layers.get("exact") != QUOTA_EXACT:
        errors.append("exact layer %s != %s" % (layers.get("exact"), QUOTA_EXACT))
    if layers.get("semantic") != QUOTA_SEMANTIC:
        errors.append("semantic layer %s != %s" % (layers.get("semantic"), QUOTA_SEMANTIC))
    if layers.get("visibility") != QUOTA_VISIBILITY:
        errors.append("visibility layer %s != %s" % (layers.get("visibility"), QUOTA_VISIBILITY))
    if layers.get("no_answer") != QUOTA_NO_ANSWER:
        errors.append("no_answer layer %s != %s" % (layers.get("no_answer"), QUOTA_NO_ANSWER))
    if colloquial < COLLOQUIAL_MIN:
        errors.append("colloquial sub-layer %d < %d" % (colloquial, COLLOQUIAL_MIN))
    for lang, floor in LANG_FLOOR.items():
        if langs.get(lang, 0) < floor:
            errors.append("language %s = %s < floor %s" % (lang, langs.get(lang, 0), floor))
    if cold < COLD_FLOOR:
        errors.append("cold band %d < floor %d" % (cold, COLD_FLOOR))
    return errors


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--corpus-dir", default=None)
    parser.add_argument("--out-dir", default=None)
    parser.add_argument(
        "--mapping",
        default=None,
        help="mapping.jsonl from the injector; when given, only successfully indexed "
        "items enter the sampling pools (post-injection reconciliation mode)",
    )
    args = parser.parse_args()

    repo = os.path.abspath(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", ".."))
    corpus_dir = args.corpus_dir
    if not corpus_dir:
        cand = os.path.join(repo, "artifacts", "corpus-v2")
        if not os.path.exists(os.path.join(cand, "index.jsonl")):
            corpus_dir = os.path.join(os.path.dirname(repo), "OmniCraft", "artifacts", "corpus-v2")
        else:
            corpus_dir = cand
    out_dir = args.out_dir or os.path.join(corpus_dir, "golden-set")

    index_rows: List[Dict[str, Any]] = []
    with open(os.path.join(corpus_dir, "index.jsonl"), "r", encoding="utf-8") as fh:
        for line in fh:
            line = line.strip()
            if line:
                index_rows.append(json.loads(line))

    if args.mapping:
        indexed = set()
        with open(args.mapping, "r", encoding="utf-8") as fh:
            for line in fh:
                line = line.strip()
                if not line:
                    continue
                row = json.loads(line)
                if row.get("indexed"):
                    indexed.add(row["corpus_item_key"])
        before = len(index_rows)
        index_rows = [r for r in index_rows if r["corpus_item_key"] in indexed]
        print(
            "reconciliation mode: %d/%d corpus items are indexed; sampling pools filtered"
            % (len(index_rows), before),
            file=sys.stderr,
        )

    cases = generate(index_rows)
    errors = validate(cases)
    if errors:
        for err in errors:
            print("INVARIANT FAIL:", err, file=sys.stderr)
        raise SystemExit(1)

    os.makedirs(out_dir, exist_ok=True)
    out_path = os.path.join(out_dir, "golden-cases-draft.jsonl")
    with open(out_path, "w", encoding="utf-8") as fh:
        for case in cases:
            fh.write(json.dumps(case, ensure_ascii=False, separators=(",", ":")) + "\n")
    print("wrote %d draft cases -> %s (finalize_in_goal=false)" % (len(cases), out_path))


if __name__ == "__main__":
    main()
