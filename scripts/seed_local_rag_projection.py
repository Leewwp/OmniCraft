#!/usr/bin/env python3
"""Seed a local keyword-only RAG projection from visible database content.

This is a local demo fixture, not a production indexer and not an embedding
provider. It creates one deterministic chunk per eligible published/public
seed row, leaves chunk_embeddings empty, and writes the same documents to the
canonical OpenSearch generation and read alias.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import subprocess
import sys
import urllib.error
import urllib.request
from pathlib import Path


ROOT = Path(__file__).resolve().parent.parent
INDEX = "omnicraft-rag-v1"
ALIAS = "omnicraft-rag-read"
EMBEDDING_MODEL = "local-keyword-seed-v1"


def psql(sql: str, *, capture: bool = False) -> str:
    command = ["docker", "compose", "exec", "-T", "postgres", "psql", "-At", "-U", "omnicraft", "-d", "omnicraft"]
    result = subprocess.run(command, cwd=ROOT, input=sql, text=True, capture_output=True, check=False)
    if result.returncode != 0:
        raise RuntimeError(f"local PostgreSQL seed failed: {result.stderr.strip()}")
    return result.stdout if capture else ""


def load_documents() -> list[dict[str, object]]:
    sql = """
SELECT COALESCE(json_agg(row_to_json(d) ORDER BY d.content_id), '[]'::json)
FROM (
  SELECT ci.id AS content_id,
         cv.version_number AS content_version,
         ci.title,
         COALESCE(NULLIF(cv.storage_key, ''), ci.description) AS description,
         ci.zone,
         ci.content_type,
         ci.category,
         ci.ip_id AS ip,
         ci.status
  FROM content_items ci
  JOIN users u ON u.id = ci.author_id
  JOIN content_versions cv ON cv.content_item_id = ci.id
    AND cv.status = 'active' AND cv.is_latest = TRUE
  WHERE ci.status = 'published'
    AND ci.is_public = TRUE
    AND ci.deleted_at IS NULL
    AND u.is_banned = FALSE
    AND u.deleted_at IS NULL
    AND (ci.id BETWEEN 1001 AND 1039 OR u.support_info @> '{"seed_namespace":"local-rich-ui-v1"}'::jsonb)
) d;
"""
    raw = psql(sql, capture=True).strip()
    if not raw:
        raise RuntimeError("local PostgreSQL returned no RAG seed rows")
    return json.loads(raw.splitlines()[-1])


def build_documents(rows: list[dict[str, object]]) -> list[dict[str, object]]:
    documents: list[dict[str, object]] = []
    for row in rows:
        content_id = int(row["content_id"])
        version = int(row["content_version"])
        title = str(row.get("title") or "").strip()
        description = str(row.get("description") or "").strip()
        if not title or not description:
            continue
        text = f"{title}\n{description}"
        source_end = len(description)
        text_hash = hashlib.sha256(text.encode("utf-8")).hexdigest()
        identity = f"{content_id}/{version}/1/0/{source_end}/{text_hash}"
        chunk_key = hashlib.sha256(identity.encode("utf-8")).hexdigest()
        documents.append(
            {
                "id": chunk_key,
                "chunk_key": chunk_key,
                "content_id": content_id,
                "content_version": version,
                "chunk_index": 0,
                "chunking_version": 1,
                "index_version": 1,
                "embedding_model": EMBEDDING_MODEL,
                "title": title,
                "heading": "",
                "text": text,
                "source_start": 0,
                "source_end": source_end,
                "zone": str(row.get("zone") or "original"),
                "content_type": str(row.get("content_type") or "article"),
                "category": row.get("category"),
                "ip": row.get("ip"),
                "tags": [],
                "status": str(row.get("status") or "published"),
            }
        )
    if not documents:
        raise RuntimeError("no eligible published/public content can form the local RAG projection")
    return documents


def sql_literal(value: object) -> str:
    if value is None:
        return "NULL"
    if isinstance(value, bool):
        return "TRUE" if value else "FALSE"
    if isinstance(value, (int, float)):
        return str(value)
    return "'" + str(value).replace("'", "''") + "'"


def write_postgres_projection(documents: list[dict[str, object]]) -> None:
    values = []
    for doc in documents:
        values.append(
            "(" + ", ".join(
                sql_literal(value)
                for value in (
                    doc["content_id"], doc["content_version"], 0, doc["chunk_key"], 1,
                    "", doc["text"], doc["source_start"], doc["source_end"],
                    doc["zone"], doc["content_type"], doc["category"], doc["ip"],
                    "{}", 1,
                )
            ) + ")"
        )
    sql = f"""
BEGIN;
DELETE FROM index_projection_status
 WHERE index_version = 1 AND content_id IN ({','.join(str(int(d['content_id'])) for d in documents)});
DELETE FROM rag_chunks
 WHERE index_version = 1 AND content_id IN ({','.join(str(int(d['content_id'])) for d in documents)});
INSERT INTO rag_chunks (
  content_id, content_version, chunk_index, chunk_key, chunking_version,
  heading, text, source_start, source_end, zone, content_type, category, ip,
  tags, index_version
)
VALUES {','.join(values)};
INSERT INTO index_projection_status (
  content_id, index_version, chunking_version, embedding_model, state,
  error_summary, last_indexed_at, is_current
)
SELECT DISTINCT content_id, 1, 1, {sql_literal(EMBEDDING_MODEL)}, 'ready',
       'local keyword-only demo projection; no embedding provider configured', NOW(), TRUE
FROM rag_chunks
WHERE index_version = 1 AND content_id IN ({','.join(str(int(d['content_id'])) for d in documents)});
COMMIT;
"""
    psql(sql)


def opensearch_request(base_url: str, path: str, method: str = "GET", body: object | None = None) -> object:
    payload = None if body is None else json.dumps(body, ensure_ascii=False).encode("utf-8")
    request = urllib.request.Request(base_url.rstrip("/") + path, data=payload, method=method)
    if payload is not None:
        request.add_header("Content-Type", "application/json")
    try:
        with urllib.request.urlopen(request, timeout=20) as response:
            raw = response.read()
    except (urllib.error.URLError, urllib.error.HTTPError) as error:
        raise RuntimeError(f"OpenSearch request failed: {method} {path}: {error}") from error
    return json.loads(raw) if raw else {}


def write_opensearch_projection(base_url: str, documents: list[dict[str, object]]) -> None:
    lines: list[str] = []
    for doc in documents:
        document_id = str(doc["id"])
        lines.append(json.dumps({"index": {"_index": INDEX, "_id": document_id}}, ensure_ascii=False))
        lines.append(json.dumps({key: value for key, value in doc.items() if key != "id"}, ensure_ascii=False))
    body = "\n".join(lines) + "\n"
    request = urllib.request.Request(
        base_url.rstrip("/") + "/_bulk?refresh=wait_for",
        data=body.encode("utf-8"),
        method="POST",
        headers={"Content-Type": "application/x-ndjson"},
    )
    try:
        with urllib.request.urlopen(request, timeout=30) as response:
            result = json.loads(response.read())
    except (urllib.error.URLError, urllib.error.HTTPError) as error:
        raise RuntimeError(f"OpenSearch bulk projection failed: {error}") from error
    if result.get("errors"):
        raise RuntimeError("OpenSearch bulk projection returned item errors")
    opensearch_request(base_url, "/_aliases", "POST", {
        "actions": [
            {"remove": {"index": "*", "alias": ALIAS, "must_exist": False}},
            {"add": {"index": INDEX, "alias": ALIAS}},
        ]
    })
    alias = opensearch_request(base_url, f"/_alias/{ALIAS}")
    if INDEX not in alias:
        raise RuntimeError(f"OpenSearch alias {ALIAS} does not resolve to {INDEX}")


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--opensearch-url", required=True)
    args = parser.parse_args()
    try:
        documents = build_documents(load_documents())
        write_postgres_projection(documents)
        write_opensearch_projection(args.opensearch_url, documents)
    except (RuntimeError, json.JSONDecodeError) as error:
        print(f"seed_local_rag_projection: {error}", file=sys.stderr)
        return 1
    print(f"local RAG projection ready: documents={len(documents)} index={INDEX} alias={ALIAS} embedding=none")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
