#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Build the #730 "realistic query distribution" k6 query set from the frozen
Golden Set v2 cases (artifacts/corpus-v2/golden-set/v2-cases.jsonl — local
artifacts, never committed).

The golden set is the only query source: 196 authored/selected cases with the
six primary layers (known_item_exact/semantic_discovery/body_evidence/
hard_neighbor/no_answer/visibility) and the three native query_forms
(exact/semantic/body). This is an evaluation-driven SYNTHETIC mix — the report
must say so, never "observed production traffic".

Output tests/load/k6/realistic-queries.json embeds: source file sha256, layer
and form census, the fixed shuffle seed, and the queries themselves (corpus
public content queries — no credentials/PII). Re-run this script only when
the golden set version changes; commit the output so the tier is reproducible
without the local artifacts tree.
"""
import collections
import hashlib
import json
import os
import random
import sys
from pathlib import Path

REPO = Path(__file__).resolve().parents[2]
# artifacts/ is local-only (gitignored): inside a worktree it does not exist;
# GOLDEN_SRC lets the builder point at the main checkout's frozen copy.
SRC = Path(os.environ.get("GOLDEN_SRC") or (REPO / "artifacts/corpus-v2/golden-set/v2-cases.jsonl"))
OUT = REPO / "tests/load/k6/realistic-queries.json"
SEED = 20261001  # frozen; bump only together with a golden-set version bump


def main() -> int:
    if not SRC.exists():
        print(f"golden set not found: {SRC} (corpus-v2 artifacts are local-only)", file=sys.stderr)
        return 1
    raw = SRC.read_bytes()
    sha = hashlib.sha256(raw).hexdigest()
    cases = [json.loads(line) for line in raw.decode().splitlines() if line.strip()]

    queries = []
    layers = collections.Counter()
    forms = collections.Counter()
    for case in cases:
        cls = case.get("classification") or {}
        layer = cls.get("primary_layer")
        form = cls.get("query_form")
        if not layer or not form:
            print(f"case {case.get('case_key')} missing layer/form", file=sys.stderr)
            return 1
        queries.append({
            "q": case["query"],
            "layer": layer,
            "form": form,
            "lang": case.get("query_language") or cls.get("language"),
        })
        layers[layer] += 1
        forms[form] += 1

    rng = random.Random(SEED)
    rng.shuffle(queries)

    payload = {
        "source": {
            "file": "artifacts/corpus-v2/golden-set/v2-cases.jsonl",
            "sha256": sha,
            "cases": len(queries),
        },
        "seed": SEED,
        "layers": dict(sorted(layers.items())),
        "forms": dict(sorted(forms.items())),
        "queries": queries,
    }
    OUT.write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(f"wrote {OUT} ({len(queries)} queries, seed {SEED}, src {sha[:12]})")
    print(f"layers={dict(sorted(layers.items()))} forms={dict(sorted(forms.items()))}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
