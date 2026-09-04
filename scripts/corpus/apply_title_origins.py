#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Golden Set v2 step 3: apply the reviewed title origins to the corpus
source, the derived index and the manifest checksums.

Authority: docs/working/2026-09-04-golden-set-v2-annotation-spec.md §6.1 —
68 deliberately untitled discussion posts keep raw_title="" and gain a
deterministic display_title with title_origin=fallback (never eligible for
the known_item_exact layer); every titled row is stamped title_origin=raw
with display_title=title (canonical form, 《》 preserved).

The frozen adjudication lives in
artifacts/corpus-v2/golden-set/title-review-v1.json (68 rows; user-reviewed
2026-09-04 with 0 reclassifications). Rows with status=pending are skipped
until their handcrafted titles pass the final review pass, so this script is
safe to run at each review stage and is idempotent: re-running with the same
adjudication rewrites nothing.

Usage:
  python3 apply_title_origins.py            # dry-run report
  python3 apply_title_origins.py --apply    # write source/index/manifest
"""
from __future__ import annotations

import argparse
import glob
import hashlib
import json
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import corpus_lib as lib  # noqa: E402

BASE = os.path.abspath(os.path.join(HERE, "..", "..", "artifacts", "corpus-v2"))
REVIEW_PATH = os.path.join(BASE, "golden-set", "title-review-v1.json")
MANIFEST_PATH = os.path.join(BASE, "manifest.json")


def load_jsonl(path):
    with open(path, "r", encoding="utf-8") as fh:
        return [json.loads(line) for line in fh if line.strip()]


def dump_jsonl(path, rows):
    with open(path, "w", encoding="utf-8") as fh:
        for row in rows:
            fh.write(json.dumps(row, ensure_ascii=False) + "\n")


def sha256_file(path):
    h = hashlib.sha256()
    with open(path, "rb") as fh:
        for chunk in iter(lambda: fh.read(1 << 16), b""):
            h.update(chunk)
    return h.hexdigest()


def load_adjudication():
    with open(REVIEW_PATH, "r", encoding="utf-8") as fh:
        doc = json.load(fh)
    rows = {r["corpus_item_key"]: r for r in doc["rows"]}
    if len(rows) != len(doc["rows"]):
        raise SystemExit("adjudication contains duplicate corpus_item_key")
    return doc, rows


def verify_double_entry(rows, sources):
    """Formula rows must reproduce corpus_lib.ensure_title from the source
    body: the frozen value and the deterministic rule must agree."""
    for key, adj in rows.items():
        if adj.get("rule") != "ensure_title-formula":
            continue
        body = sources[key]["body_md"]
        want = lib.ensure_title("", body, key)
        if adj["display_title"] != want:
            raise SystemExit(
                "double-entry mismatch for %s: frozen %r != formula %r"
                % (key, adj["display_title"], want)
            )


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--apply", action="store_true", help="write changes (default: dry-run)")
    args = parser.parse_args()

    doc, adjudicated = load_adjudication()
    source_files = sorted(glob.glob(os.path.join(BASE, "corpus-batch-*.jsonl")))
    sources = {}
    for path in source_files:
        for row in load_jsonl(path):
            sources[row["corpus_item_key"]] = row
    verify_double_entry(adjudicated, sources)

    changed_batches = {}
    stats = {"raw": 0, "fallback": 0, "skipped_pending": 0, "lines_rewritten": 0}
    for path in source_files:
        out_lines, touched = [], False
        with open(path, "r", encoding="utf-8") as fh:
            for line in fh:
                row = json.loads(line)
                key = row["corpus_item_key"]
                title = (row.get("title") or "").strip()
                if title:
                    origin, display = "raw", row["title"]
                elif key in adjudicated and adjudicated[key]["status"] == "approved":
                    origin = adjudicated[key]["title_origin"]
                    display = adjudicated[key]["display_title"]
                else:
                    origin, display = None, None
                    if key in adjudicated:
                        stats["skipped_pending"] += 1
                if origin is None:
                    out_lines.append(line.rstrip("\n"))
                    continue
                meta = dict(row.get("metadata") or {})
                if meta.get("title_origin") == origin and meta.get("display_title") == display:
                    out_lines.append(line.rstrip("\n"))
                    stats[origin] += 1
                    continue
                meta["title_origin"] = origin
                meta["display_title"] = display
                row["metadata"] = meta
                out_lines.append(json.dumps(row, ensure_ascii=False))
                stats["lines_rewritten"] += 1
                stats[origin] += 1
                touched = True
        if touched:
            changed_batches[path] = out_lines

    # derived index: same two additive fields, title stays the raw form
    index_files = [os.path.join(BASE, "index.jsonl")] + sorted(
        glob.glob(os.path.join(BASE, "index-part-*.jsonl"))
    )
    changed_indexes = {}
    for path in index_files:
        rows = load_jsonl(path)
        touched = False
        for row in rows:
            key = row["corpus_item_key"]
            title = (row.get("title") or "").strip()
            if title:
                origin, display = "raw", row["title"]
            elif key in adjudicated and adjudicated[key]["status"] == "approved":
                origin = adjudicated[key]["title_origin"]
                display = adjudicated[key]["display_title"]
            else:
                continue
            if row.get("title_origin") != origin or row.get("display_title") != display:
                row["title_origin"] = origin
                row["display_title"] = display
                touched = True
        if touched:
            changed_indexes[path] = rows

    mode = "APPLY" if args.apply else "DRY-RUN"
    print("[%s] adjudication v%d: %d rows (%d approved, %d pending)"
          % (mode, doc["version"], len(adjudicated),
             sum(1 for r in adjudicated.values() if r["status"] == "approved"),
             sum(1 for r in adjudicated.values() if r["status"] == "pending")))
    print("[%s] source lines stamped: raw=%d fallback=%d rewritten=%d pending-skipped=%d"
          % (mode, stats["raw"], stats["fallback"], stats["lines_rewritten"], stats["skipped_pending"]))
    print("[%s] files to change: %d batch, %d index"
          % (mode, len(changed_batches), len(changed_indexes)))
    if not args.apply:
        return

    for path, lines in changed_batches.items():
        with open(path, "w", encoding="utf-8") as fh:
            for line in lines:
                fh.write(line + "\n")
    for path, rows in changed_indexes.items():
        dump_jsonl(path, rows)

    with open(MANIFEST_PATH, "r", encoding="utf-8") as fh:
        manifest = json.load(fh)
    note = ("2026-09-04 golden-set v2 step3: title_origin/display_title stamped "
            "(adjudication golden-set/title-review-v1.json; raw=canonical title, "
            "fallback=reviewed deterministic display title); checksums recomputed")
    for path in list(changed_batches) + list(changed_indexes):
        name = os.path.basename(path)
        if name in manifest.get("checksums", {}):
            manifest["checksums"][name] = sha256_file(path)
    if note not in manifest.get("notes", []):
        manifest.setdefault("notes", []).append(note)
    with open(MANIFEST_PATH, "w", encoding="utf-8") as fh:
        json.dump(manifest, fh, ensure_ascii=False, indent=1)
        fh.write("\n")
    print("[APPLY] manifest checksums updated for %d files; note appended"
          % (len(changed_batches) + len(changed_indexes)))


if __name__ == "__main__":
    main()
