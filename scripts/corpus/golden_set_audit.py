#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Golden set draft audit (#291) — retrieval validity, 2026-09-04.

One-off audit tooling: title-leakage measurement, template census, no_answer
duplication, distractor relatedness, visibility consistency, corpus title
defects. Read-only over index.jsonl / golden-cases-draft.jsonl / mapping.jsonl.
"""
from __future__ import annotations

import json
import os
import re
from collections import Counter, defaultdict

REPO = os.path.abspath(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", ".."))
BASE = os.path.join(REPO, "artifacts", "corpus-v2")

idx = {}
for line in open(f"{BASE}/index.jsonl", encoding="utf-8"):
    r = json.loads(line)
    idx[r["corpus_item_key"]] = r

mapping = {}
for line in open(f"{BASE}/injection/mapping.jsonl", encoding="utf-8"):
    r = json.loads(line)
    mapping[r["corpus_item_key"]] = r

cases = [json.loads(l) for l in open(f"{BASE}/golden-set/golden-cases-draft.jsonl", encoding="utf-8")]

EMPTY_TITLES = {k for k, r in idx.items() if not r["title"].strip()}

# ---------------------------------------------------------------- helpers

def strip_marks(t: str) -> str:
    return t.strip().strip("《》").strip()

def title_core(title: str, ip: str) -> str:
    """Title minus brackets/notes/IP prefix (generic version of the generator's)."""
    body = strip_marks(title)
    body = re.sub(r"[（(][^）)]*[）)]", "", body)
    if ip and body.startswith(ip):
        body = body[len(ip):]
    return body.strip(" ·:：—－-")

def title_core_gen(title: str) -> str:
    """Replicate the generator's title_core() exactly (only strips 原神 prefix)."""
    body = strip_marks(title)
    body = re.sub(r"[（(][^）)]*[）)]", "", body)
    body = re.sub(r"^原神[·:：]", "", body)
    return body.strip(" ·:：")

TOPIC_SPLIT = re.compile(r"[·:：—－-]")

def title_topic(title: str) -> str:
    core = title_core_gen(title)
    parts = [p.strip() for p in TOPIC_SPLIT.split(core) if p.strip()]
    return parts[-1] if parts else core

def lcs_substr(a: str, b: str) -> str:
    """Longest common contiguous substring."""
    if not a or not b:
        return ""
    m, n = len(a), len(b)
    dp = [[0] * (n + 1) for _ in range(m + 1)]
    best, end = 0, 0
    for i in range(1, m + 1):
        for j in range(1, n + 1):
            if a[i - 1] == b[j - 1]:
                dp[i][j] = dp[i - 1][j - 1] + 1
                if dp[i][j] > best:
                    best, end = dp[i][j], i
    return a[end - best:end]

EN_STOP = {"a", "an", "the", "of", "and", "or", "to", "in", "on", "at", "for",
           "from", "with", "is", "are", "it", "its", "that", "this"}

def en_words(s: str):
    return [w.lower() for w in re.findall(r"[A-Za-z0-9'’]+", s)]

def en_content_overlap(query: str, title: str):
    """Longest contiguous run of title content-words appearing in the query."""
    qw = en_words(query)
    qset = set(qw)
    tw = [w for w in en_words(title) if w not in EN_STOP]
    best_run, run = 0, 0
    for w in tw:
        if w in qset:
            run += 1
            best_run = max(best_run, run)
        else:
            run = 0
    return best_run, len(tw)

def bigrams(s: str):
    s = re.sub(r"\s+", "", s)
    return {s[i:i + 2] for i in range(len(s) - 1)}

def jaccard(a: str, b: str) -> float:
    A, B = bigrams(a), bigrams(b)
    if not A or not B:
        return 0.0
    return len(A & B) / len(A | B)

# ---------------------------------------------------------------- semantic templates

TEMPLATES = {
    "zh_sem": ["有没有讲{topic}的{ip}同人", "想看{ip}里围绕{topic}的故事",
               "{ip}同人里关于{topic}的设定或剧情有人写过吗", "求推荐{ip}题材下涉及{topic}的作品",
               "{ip}有没有聚焦{topic}的创作"],
    "zh_col": ["想看{ip}的东西，最好和{topic}有关，有推荐吗", "最近入坑{ip}，想找{topic}向的粮，求安利",
               "有没有{ip}的{topic}题材好文，轻松一点的", "安利一下{ip}里{topic}相关的作品呗",
               "求{ip}同人中讲{topic}的，长短都行"],
    "en_sem": ["any {ip} fanworks exploring {topic}", "looking for {ip} stories centred on {topic}",
               "{ip} fanfic recommendations about {topic}"],
    "en_col": ["new to {ip}, anything about {topic} to read?", "can someone rec {ip} works with {topic} vibes",
               "in the mood for {ip}, preferably {topic}-ish, suggestions?"],
}

def match_template(query: str, ip: str):
    ipesc = re.escape(ip)
    for kind, tlist in TEMPLATES.items():
        for t in tlist:
            pat = re.escape(t).replace(re.escape("{ip}"), "(" + ipesc + ")").replace(
                re.escape("{topic}"), "(.+?)")
            m = re.fullmatch("^" + pat + "$", query)
            if m:
                return kind, t, m.group(2)
    return None, None, None

# ---------------------------------------------------------------- audit

report = {}
by_layer = defaultdict(list)
for c in cases:
    by_layer[c["classification"]["primary_layer"]].append(c)

print("=" * 72)
print("A. 分层与配额复核")
print("=" * 72)
print("layers:", dict(Counter(c["classification"]["primary_layer"] for c in cases)))
print("ip_scope:", dict(Counter((c["classification"]["primary_layer"], c["classification"]["ip_scope"])
                                for c in cases)))
colloquial = [c for c in by_layer["semantic"] if c["classification"].get("colloquial_recommendation")]
print("semantic colloquial:", len(colloquial))

print()
print("=" * 72)
print("B. semantic 层标题泄漏测量（48 条）")
print("=" * 72)
sem_rows = []
for c in by_layer["semantic"]:
    key = c["expected_citation_keys"][0]
    item = idx[key]
    ip = item["ip"]
    title = strip_marks(item["title"])
    core = title_core(item["title"], ip)
    topic = title_topic(item["title"])
    q = c["query"]
    lcs = lcs_substr(q, core)
    cov = len(lcs) / len(core) if core else 0
    topic_copied = topic in q if len(topic) >= 2 else False
    en_run, en_tot = (en_content_overlap(q, title) if re.search(r"[A-Za-z]", title) else (0, 0))
    kind, tmpl, used_topic = match_template(q, ip)
    # 分档：A=标题复读级（副题整段或长串原文）；B=长片段；C=低重合
    if topic_copied or cov >= 0.5 or len(lcs) >= 6 or en_run >= 2:
        tier = "A"
    elif len(lcs) >= 4 or cov >= 0.3 or en_run == 1:
        tier = "B"
    else:
        tier = "C"
    sem_rows.append({
        "case_key": c["case_key"], "lang": c["query_language"], "query": q, "title": title,
        "core": core, "topic": topic, "topic_copied": topic_copied, "lcs": lcs,
        "lcs_len": len(lcs), "coverage": round(cov, 2), "en_run": en_run, "en_tot": en_tot,
        "tier": tier, "tmpl_kind": kind or "?", "used_topic": used_topic or "",
        "colloquial": bool(c["classification"].get("colloquial_recommendation")),
    })

tier_counts = Counter(r["tier"] for r in sem_rows)
print("tiers:", dict(tier_counts))
print("topic 整段复读（title_topic 原文出现在查询中）:", sum(1 for r in sem_rows if r["topic_copied"]))
print("覆盖率≥0.5:", sum(1 for r in sem_rows if r["coverage"] >= 0.5))
print()
print("-- 48 条逐条 --")
for r in sem_rows:
    print(f"{r['case_key']} [{r['tier']}] {r['lang']} cov={r['coverage']:.2f} lcs={r['lcs_len']}({r['lcs'][:18]!r}) "
          f"topicCopied={r['topic_copied']} enRun={r['en_run']}/{r['en_tot']} tmpl={r['tmpl_kind']}")
    print(f"    Q: {r['query']}")
    print(f"    T: {r['title']}   [core={r['core']!r} topic={r['topic']!r}]")

print()
print("=" * 72)
print("C. no_answer 层重复与干扰项质量（24 条）")
print("=" * 72)
na = by_layer["no_answer"]
norm = lambda q: re.sub(r"^anyone knows:\s*", "", q.strip())
str_counts = Counter(c["query"] for c in na)
norm_counts = Counter(norm(c["query"]) for c in na)
print("精确重复组（同一字符串出现>1）:", sum(1 for v in str_counts.values() if v > 1),
      "| 不同字符串:", len(str_counts), "| 去掉 anyone knows: 前缀后不同问题:", len(norm_counts))
print("话题分布:", dict(norm_counts))
worst_rel = []
for c in na:
    q = c["query"]
    rels = []
    for fk in c["forbidden_keys"]:
        t = strip_marks(idx[fk]["title"]) if fk in idx else "<missing>"
        rels.append((fk, t if t else "<EMPTY-TITLE>", round(jaccard(q, t), 3)))
    worst_rel.append((c["case_key"], q, rels, max(r[2] for r in rels)))
print("查询↔干扰项 双字 bigram Jaccard 最大值分布:",
      dict(Counter("0" if m == 0 else "<0.05" if m < 0.05 else "<0.15" if m < 0.15 else ">=0.15"
                   for _, _, _, m in worst_rel)))
for ck, q, rels, m in worst_rel:
    print(f"{ck} maxJ={m} Q={q[:40]!r} distractors={[r[1][:22] for r in rels]}")

print()
print("=" * 72)
print("D. 可见性一致性与身份字段")
print("=" * 72)
print("viewer_context 取值:", dict(Counter(json.dumps(c["viewer_context"], ensure_ascii=False) for c in cases)))
print("classification.visibility_context:",
      dict(Counter((c["classification"]["primary_layer"], c["classification"]["visibility_context"]) for c in cases)))
conflicts = []
for layer in ("exact", "semantic"):
    for c in by_layer[layer]:
        for k in c["expected_citation_keys"]:
            vis = idx[k]["visibility"]
            if vis != "public":
                conflicts.append((c["case_key"], layer, k, vis, strip_marks(idx[k]["title"])))
print("exact/semantic 期望命中但文档非 public（viewer=anonymous 矛盾）:", len(conflicts))
for row in conflicts:
    print("   ", row)
vis_forbidden = {fk for c in by_layer["visibility"] for fk in c["forbidden_keys"]}
exp_keys = {fk for layer in ("exact", "semantic") for c in by_layer[layer] for fk in c["expected_citation_keys"]}
cross = exp_keys & vis_forbidden
print("exact/semantic 期望 ∩ visibility 禁止（同 key 双重角色）:", len(cross))
for k in sorted(cross):
    print("   ", k, idx[k]["visibility"], strip_marks(idx[k]["title"]))
# 多 relevant refs 的可见性（semantic siblings）
sib_issues = []
for c in by_layer["semantic"]:
    for r in c["relevant_refs"]:
        k = r["corpus_item_key"]
        if k not in c["expected_citation_keys"] and idx[k]["visibility"] != "public":
            sib_issues.append((c["case_key"], k, idx[k]["visibility"]))
print("semantic sibling relevant_refs 非 public:", len(sib_issues), sib_issues[:10])

print()
print("=" * 72)
print("E. 语料标题缺陷与引用命中")
print("=" * 72)
print("index.jsonl 空标题条目:", len(EMPTY_TITLES))
refs_expected = [(c["case_key"], k) for layer in ("exact", "semantic") for c in by_layer[layer]
                 for k in c["expected_citation_keys"]]
refs_forbidden = [(c["case_key"], k) for c in cases for k in c["forbidden_keys"]]
print("期望命中引用空标题:", [r for r in refs_expected if r[1] in EMPTY_TITLES])
print("禁止引用空标题:", [r for r in refs_forbidden if r[1] in EMPTY_TITLES])
print("索引标题自带《》:", sum(1 for r in idx.values() if r["title"].startswith("《")))

print()
print("=" * 72)
print("F. 冒烟结果归因（smoke-result.json 18/20）")
print("=" * 72)
try:
    smoke = json.load(open(f"{BASE}/injection/smoke-result.json", encoding="utf-8"))
    for r in smoke.get("results", []):
        if not r.get("hit"):
            k = r["key"]
            it = idx.get(k, {})
            print("miss:", k, "| vis=", it.get("visibility"), "| title_form=", it.get("title_form"),
                  "| title=", repr(it.get("title")), "| empty=", k in EMPTY_TITLES)
except FileNotFoundError:
    print("smoke-result.json 不存在")

print()
print("=" * 72)
print("G. exact 层口径（IP 前缀与标题复述）")
print("=" * 72)
ex_ip = [c for c in by_layer["exact"] if c["classification"]["ip_scope"] == "ip_localized"]
print("exact 带「在IP内：」前缀:", len(ex_ip), "/ 64")
exact_title_eq = 0
for c in by_layer["exact"]:
    k = c["expected_citation_keys"][0]
    t = strip_marks(idx[k]["title"])
    q = re.sub(r"^在.+?内：", "", c["query"])
    if q.strip() == t.strip():
        exact_title_eq += 1
print("exact 查询==完整标题（去前缀后）:", exact_title_eq, "/ 64")
