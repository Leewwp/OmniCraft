"""Shared helpers for the OmniCraft generation-layer eval harness (SP-22 E1).

Reads the per-case JSONL produced by `cmd/rag-eval` (generation rows carry
question/answer/status/citations/retrieved_ids), reconstructs the context
surface the agent actually saw (title + first 240 runes of the ranked chunk
text — the exact shape `retrievalSummaries` renders into the tool result),
and derives the answer/refusal buckets used by the report.

Context reconstruction is post-hoc by design: generation rows keep only
content ids, so chunk texts are resolved from PostgreSQL at harness time.
For cited candidates the citation's chunk_key pins the exact chunk; for
uncited candidates the chunk with the highest character-bigram overlap
with the query stands in for the retrieval hit (documented approximation).

No secrets here: judge/embedding keys are read from the environment by the
metric backends, never persisted into artifacts.
"""

from __future__ import annotations

import json
import os
import subprocess
from dataclasses import dataclass, field

# Mirrors truncateRunes(..., 240) in agent_tools.go — the excerpt length the
# model actually received per candidate.
EXCERPT_RUNES = 240

PSQL_CONTAINER = os.environ.get("EVALS_PG_CONTAINER", "omnicraft-postgres")
PSQL_USER = os.environ.get("EVALS_PG_USER", "omnicraft")
PSQL_DB = os.environ.get("EVALS_PG_DB", "omnicraft")


@dataclass
class EvalContext:
    content_id: int
    chunk_key: str
    title: str
    text: str

    def surface(self) -> str:
        """The context string exactly as the agent's tool result renders it."""
        return f"{self.title}\n{self.text[:EXCERPT_RUNES]}"


@dataclass
class EvalCase:
    case_key: str
    query: str
    query_language: str
    status: str
    answer_kind: str
    answer: str
    citations: list
    retrieved_ids: list
    contexts: list = field(default_factory=list)

    @property
    def refused(self) -> bool:
        return self.status in ("no_evidence", "provider_error") or self.answer_kind == "no_evidence"

    @property
    def cited_ids(self) -> set:
        return {c.get("content_id") for c in self.citations if c.get("content_id")}


def load_generation_rows(path: str) -> tuple[dict, list[EvalCase]]:
    """Parse a rag-eval JSONL into (header, generation cases).

    The first `{"kind": "rag-eval", ...}` line carries the run header
    (label/switches/runtime); `phase == "generation"` lines are the cases.
    Legacy rows without a phase field are treated as generation rows.
    """
    header = {}
    cases: dict[str, EvalCase] = {}
    with open(path, encoding="utf-8") as fh:
        for line in fh:
            line = line.strip()
            if not line:
                continue
            row = json.loads(line)
            if row.get("kind") == "rag-eval":
                header = row
                continue
            phase = row.get("phase", "generation")
            if phase != "generation":
                continue
            key = row.get("case_key", "")
            if not key:
                continue
            cases[key] = EvalCase(
                case_key=key,
                query=row.get("query", ""),
                query_language=row.get("query_language", "zh"),
                status=row.get("status", ""),
                answer_kind=row.get("answer_kind", ""),
                answer=row.get("answer", ""),
                citations=row.get("citations") or [],
                retrieved_ids=row.get("retrieved_ids") or [],
            )
    return header, [cases[k] for k in sorted(cases)]


def _psql(sql: str) -> str:
    proc = subprocess.run(
        ["docker", "exec", PSQL_CONTAINER, "psql", "-U", PSQL_USER, "-d", PSQL_DB,
         "-t", "-A", "-F", "\x1f", "-c", sql],
        capture_output=True, text=True, check=True,
    )
    return proc.stdout


def _bigrams(text: str) -> set:
    chars = [c for c in text if not c.isspace()]
    return {chars[i] + chars[i + 1] for i in range(len(chars) - 1)} if len(chars) > 1 else set(chars)


def load_noise_pool(exclude_ids, target: int = 120) -> list[str]:
    """First-chunk surfaces of random unrelated contents for noise injection.

    The pool must come from OUTSIDE the evaluated cases' retrieval set, so it
    samples content ids from the database excluding them. json_agg keeps
    multi-line chunk texts intact.
    """
    exclude = sorted(int(i) for i in exclude_ids if int(i) > 0)
    sql = (
        "SELECT COALESCE(json_agg(t), '[]'::json) FROM ("
        "SELECT c.title AS title, COALESCE((SELECT rc.text FROM rag_chunks rc "
        "WHERE rc.content_id = c.id AND rc.content_version = "
        "(SELECT MAX(rc2.content_version) FROM rag_chunks rc2 "
        "WHERE rc2.content_id = c.id) ORDER BY rc.chunk_index LIMIT 1), c.title) AS text "
        "FROM content_items c "
        "WHERE c.id NOT IN (" + (",".join(str(i) for i in exclude) or "-1") + ") "
        "ORDER BY random() LIMIT " + str(target) + ") t"
    )
    rows = json.loads(_psql(sql).strip() or "[]")
    return [f"{r['title']}\n{r['text'][:EXCERPT_RUNES]}" for r in rows if r.get("text")]


def load_chunks(content_ids) -> dict:
    """Fetch latest-version chunks for the given content ids from PostgreSQL.

    Returns {content_id: [{"chunk_key", "chunk_index", "title", "text"}, ...]}.
    Chunk texts contain newlines, so the result comes back as one json_agg
    blob instead of line-oriented -t -A output.
    """
    ids = sorted({int(i) for i in content_ids if int(i) > 0})
    if not ids:
        return {}
    id_list = ",".join(str(i) for i in ids)
    sql = (
        "SELECT COALESCE(json_agg(t), '[]'::json) FROM ("
        "SELECT rc.content_id AS content_id, rc.chunk_key AS chunk_key, "
        "rc.chunk_index AS chunk_index, rc.text AS text, ci.title AS title "
        "FROM rag_chunks rc "
        "JOIN LATERAL (SELECT title FROM content_items WHERE id = rc.content_id) ci ON TRUE "
        "WHERE rc.content_id IN (" + id_list + ") "
        "AND rc.content_version = (SELECT MAX(rc2.content_version) FROM rag_chunks rc2 "
        "WHERE rc2.content_id = rc.content_id) "
        "ORDER BY rc.content_id, rc.chunk_index) t"
    )
    rows = json.loads(_psql(sql).strip() or "[]")
    chunks: dict = {}
    for row in rows:
        chunks.setdefault(int(row["content_id"]), []).append(
            {"chunk_key": row["chunk_key"], "text": row["text"], "title": row["title"]}
        )
    return chunks


def attach_contexts(case: EvalCase, chunks: dict) -> None:
    """Reconstruct the ranked context surface for one case.

    Cited candidates resolve to the citation's chunk; uncited candidates fall
    back to the highest query-overlap chunk of that content.
    """
    citation_chunks = {c.get("chunk_key") for c in case.citations if c.get("chunk_key")}
    qgrams = _bigrams(case.query)
    for cid in case.retrieved_ids:
        rows = chunks.get(int(cid))
        if not rows:
            continue
        chosen = None
        for row in rows:
            if row["chunk_key"] in citation_chunks:
                chosen = row
                break
        if chosen is None:
            best = -1
            for row in rows:
                overlap = len(_bigrams(row["text"]) & qgrams)
                if overlap > best:
                    best, chosen = overlap, row
        case.contexts.append(
            EvalContext(content_id=int(cid), chunk_key=chosen["chunk_key"],
                        title=chosen["title"], text=chosen["text"])
        )


def build_dataset(runs_path: str) -> tuple[dict, list[EvalCase]]:
    """Load generation rows and attach reconstructed contexts for every case."""
    header, cases = load_generation_rows(runs_path)
    all_ids = [cid for case in cases for cid in case.retrieved_ids]
    chunks = load_chunks(all_ids)
    for case in cases:
        attach_contexts(case, chunks)
    return header, cases


def buckets(cases: list[EvalCase]) -> dict:
    """Answer/refusal split with per-language and per-kind sub-buckets."""
    out = {
        "total": len(cases),
        "answered": sum(1 for c in cases if not c.refused and c.answer),
        "refused": sum(1 for c in cases if c.refused),
        "by_language": {},
        "by_kind": {},
    }
    for c in cases:
        out["by_language"][c.query_language] = out["by_language"].get(c.query_language, 0) + 1
        if c.answer_kind:
            out["by_kind"][c.answer_kind] = out["by_kind"].get(c.answer_kind, 0) + 1
    return out
