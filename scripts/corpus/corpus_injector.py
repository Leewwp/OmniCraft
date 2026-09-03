#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Corpus-v2 injector: drive 1600 synthetic items through the REAL local
publishing pipeline via REST (#291 CORPUS-01).

Pipeline per item (see corpus_lib.py docstring and artifacts/lane1-status.md):

  1. POST /api/v1/contents            (author token; form validation -> pending)
  2. version_seed                     (ops tool: initial content_version row)
  3. POST /api/v1/internal/ai-callback(production moderation callback entry,
                                       SHA256 checksum auth -> published +
                                       outbox content.published)
  4. worker relay -> indexer -> RAG projection (chunk + embed -> rag_chunks /
     chunk_embeddings + OpenSearch keyword path)
  5. multi-version items: real PR flow per version (POST /pr, POST /pr/:id/merge)
     + PATCH /contents/:id (title) -> content.updated -> re-projection
  6. wait until index_projection_status is ready for the final version

Idempotency: checkpoint JSONL in artifacts/corpus-v2/injection/ plus a
`c2:<corpus_item_key>` content tag as the DB-side anchor.

Circuit breaker: consecutive failures >= 5 or error ratio > 2% stops the run.

Usage (each subcommand is resumable):
  python3 corpus_injector.py ensure-users
  python3 corpus_injector.py ensure-ips
  python3 corpus_injector.py ensure-follows
  python3 corpus_injector.py inject --limit 10
  python3 corpus_injector.py backfill-timestamps
  python3 corpus_injector.py verify
  python3 corpus_injector.py smoke --n 20
  python3 corpus_injector.py export-mapping

Environment:
  API_BASE        default http://localhost:8080/api/v1
  GREEN_SEED      local callback seed (must match the server env)
  GREEN_UID       local callback uid  (must match the server env)
  CORPUS_DIR      default <repo>/artifacts/corpus-v2
  INJECTION_DIR   default $CORPUS_DIR/injection
  PSQL            default: docker exec -i omnicraft-postgres psql -U omnicraft -d omnicraft -X -q
  VERSION_SEED_BIN default /tmp/omnicraft-version-seed
"""
from __future__ import annotations

import argparse
import http.cookiejar
import json
import os
import subprocess
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
from typing import Any, Dict, List, Optional, Tuple

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import corpus_lib as lib  # noqa: E402

REPO_ROOT = os.path.abspath(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", ".."))
MAIN_REPO_ROOT = os.path.dirname(REPO_ROOT.rstrip("/")) if os.path.basename(REPO_ROOT).startswith("OmniCraft-wt") else REPO_ROOT
# The corpus lives in the main checkout's gitignored artifacts/ dir; when this
# script runs from a worktree, fall back to the sibling main checkout.
_CORPUS_DEFAULT = os.path.join(REPO_ROOT, "artifacts", "corpus-v2")
# worktree layout: <project>/OmniCraft-wt-l1 sits beside <project>/OmniCraft;
# the corpus only exists in the main checkout (gitignored artifacts).
if not os.path.exists(os.path.join(_CORPUS_DEFAULT, "index.jsonl")):
    _main = os.path.join(os.path.dirname(REPO_ROOT), "OmniCraft", "artifacts", "corpus-v2")
    if os.path.exists(os.path.join(_main, "index.jsonl")):
        _CORPUS_DEFAULT = _main
API_BASE = os.environ.get("API_BASE", "http://localhost:8080/api/v1")
CORPUS_DIR = os.environ.get("CORPUS_DIR", _CORPUS_DEFAULT)
INJECTION_DIR = os.environ.get("INJECTION_DIR", os.path.join(CORPUS_DIR, "injection"))
PSQL = os.environ.get(
    "PSQL", "docker exec -i omnicraft-postgres psql -U omnicraft -d omnicraft -X -q"
).split()
VERSION_SEED_BIN = os.environ.get("VERSION_SEED_BIN", "/tmp/omnicraft-version-seed")
GREEN_SEED = os.environ.get("GREEN_SEED", "")
GREEN_UID = os.environ.get("GREEN_UID", "")

CHECKPOINT_PATH = os.path.join(INJECTION_DIR, "checkpoint.jsonl")
MAPPING_PATH = os.path.join(INJECTION_DIR, "mapping.jsonl")
TOKENS_PATH = os.path.join(INJECTION_DIR, "tokens.json")
LOG_PATH = os.path.join(INJECTION_DIR, "inject.log")

ACCESS_TTL = 110  # access_token_ttl is 120s; refresh margin
MAX_LOGINS_PER_MIN = 3  # credential limit is 5/min per IP per calendar minute


def log(msg: str) -> None:
    line = "[%s] %s" % (time.strftime("%H:%M:%S"), msg)
    print(line, flush=True)
    os.makedirs(INJECTION_DIR, exist_ok=True)
    with open(LOG_PATH, "a", encoding="utf-8") as fh:
        fh.write(line + "\n")


# ---------------------------------------------------------------- SQL helpers


def psql(sql: str) -> str:
    proc = subprocess.run(PSQL + ["-c", sql], capture_output=True, text=True)
    if proc.returncode != 0:
        raise RuntimeError("psql failed: %s\nSQL: %s" % (proc.stderr.strip()[:400], sql[:200]))
    return proc.stdout


def psql_scalar(sql: str) -> Optional[str]:
    proc = subprocess.run(PSQL + ["-t", "-A", "-c", sql], capture_output=True, text=True)
    if proc.returncode != 0:
        raise RuntimeError("psql failed: %s\nSQL: %s" % (proc.stderr.strip()[:400], sql[:200]))
    out = proc.stdout.strip()
    return out if out else None


# ---------------------------------------------------------------- HTTP client


class Api:
    def __init__(self) -> None:
        self.tokens: Dict[str, Dict[str, Any]] = {}
        if os.path.exists(TOKENS_PATH):
            with open(TOKENS_PATH, "r", encoding="utf-8") as fh:
                self.tokens = json.load(fh)
        self.login_times: List[float] = []
        self._limiter = lib.RateLimiter(float(os.environ.get("RPS", "3")))
        # CSRF is a double-submit cookie: every response refreshes the
        # csrf-token cookie; write methods must echo it in X-CSRF-Token.
        self.jar = http.cookiejar.CookieJar()
        self.opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(self.jar))

    def _csrf_token(self) -> str:
        for cookie in self.jar:
            if cookie.name == "csrf-token":
                return cookie.value
        return ""

    # -- raw request ------------------------------------------------------

    def _request(
        self,
        method: str,
        path: str,
        body: Optional[Dict[str, Any]] = None,
        token: Optional[str] = None,
        form: Optional[Dict[str, str]] = None,
    ) -> Tuple[int, Dict[str, Any]]:
        url = API_BASE + path
        data = None
        headers = {"Accept": "application/json"}
        if form is not None:
            data = urllib.parse.urlencode(form).encode("utf-8")
            headers["Content-Type"] = "application/x-www-form-urlencoded"
        elif body is not None:
            data = json.dumps(body, ensure_ascii=False).encode("utf-8")
            headers["Content-Type"] = "application/json"
        if token:
            headers["Authorization"] = "Bearer " + token
        if method in ("POST", "PATCH", "PUT", "DELETE") and "/internal/" not in path:
            if not self._csrf_token():
                # prime the session cookie before the first write
                self.opener.open(urllib.request.Request(API_BASE + "/auth/csrf", method="GET"), timeout=30).read()
            csrf = self._csrf_token()
            if csrf:
                headers["X-CSRF-Token"] = csrf
        req = urllib.request.Request(url, data=data, headers=headers, method=method)
        self._limiter.wait()
        try:
            with self.opener.open(req, timeout=60) as resp:
                payload = resp.read().decode("utf-8")
                status = resp.status
        except urllib.error.HTTPError as err:
            payload = err.read().decode("utf-8", "replace")
            status = err.code
        parsed: Dict[str, Any] = {}
        if payload:
            try:
                parsed = json.loads(payload)
            except ValueError:
                parsed = {"_raw": payload[:500]}
        return status, parsed

    # -- auth ---------------------------------------------------------------

    def login(self, email: str) -> Tuple[str, int]:
        cached = self.tokens.get(email)
        if cached and cached.get("exp", 0) > time.time() + 5:
            return cached["access"], int(cached["exp"])
        # global login throttle: credential endpoint is 5/min per IP
        now = time.time()
        self.login_times = [t for t in self.login_times if now - t < 60]
        if len(self.login_times) >= MAX_LOGINS_PER_MIN:
            sleep = 60 - (now - self.login_times[0]) + 1
            log("login throttle: sleeping %.1fs" % sleep)
            time.sleep(max(sleep, 1.0))
            now = time.time()
            self.login_times = [t for t in self.login_times if now - t < 60]
        status, payload = self._request("POST", "/auth/login", {"email": email, "password": lib.FIXTURE_PASSWORD})
        if status == 429:
            # server window is the calendar minute; back off past the boundary
            time.sleep(90)
            status, payload = self._request("POST", "/auth/login", {"email": email, "password": lib.FIXTURE_PASSWORD})
        self.login_times.append(time.time())
        if status != 200 or "tokens" not in payload:
            raise RuntimeError("login failed for %s: %d %s" % (email, status, str(payload)[:200]))
        access = payload["tokens"]["access_token"]
        exp = time.time() + ACCESS_TTL
        self.tokens[email] = {"access": access, "exp": exp}
        os.makedirs(INJECTION_DIR, exist_ok=True)
        with open(TOKENS_PATH, "w", encoding="utf-8") as fh:
            json.dump(self.tokens, fh)
        return access, int(exp)

    def user_id(self, email: str) -> int:
        row = psql_scalar("SELECT id FROM users WHERE email = %s" % lib.sql_quote(email))
        if row is None:
            raise RuntimeError("fixture user missing: %s" % email)
        return int(row)


# ---------------------------------------------------------------- corpus load


def load_corpus() -> Tuple[List[Dict[str, Any]], Dict[str, Dict[str, Any]], Dict[str, Any]]:
    """Return (index_rows, bodies_by_key, manifest)."""
    index_rows: List[Dict[str, Any]] = []
    with open(os.path.join(CORPUS_DIR, "index.jsonl"), "r", encoding="utf-8") as fh:
        for line in fh:
            line = line.strip()
            if line:
                index_rows.append(json.loads(line))
    bodies: Dict[str, Dict[str, Any]] = {}
    for name in sorted(os.listdir(CORPUS_DIR)):
        if not name.startswith("corpus-batch-") or not name.endswith(".jsonl"):
            continue
        with open(os.path.join(CORPUS_DIR, name), "r", encoding="utf-8") as fh:
            for line in fh:
                line = line.strip()
                if not line:
                    continue
                row = json.loads(line)
                bodies[row["corpus_item_key"]] = row
    with open(os.path.join(CORPUS_DIR, "manifest.json"), "r", encoding="utf-8") as fh:
        manifest = json.load(fh)
    return index_rows, bodies, manifest


def author_email(author_id: str) -> str:
    return author_id + lib.CORPUS_EMAIL_SUFFIX


ADMIN_EMAIL = "admin" + lib.CORPUS_EMAIL_SUFFIX


# ---------------------------------------------------------------- checkpoints


class Checkpoint:
    def __init__(self) -> None:
        os.makedirs(INJECTION_DIR, exist_ok=True)
        self.states = lib.load_checkpoint(CHECKPOINT_PATH)
        self._fh = open(CHECKPOINT_PATH, "a", encoding="utf-8")

    def get(self, key: str) -> lib.ItemState:
        if key not in self.states:
            self.states[key] = lib.ItemState(key=key)
        return self.states[key]

    def save(self, state: lib.ItemState) -> None:
        self.states[state.key] = state
        self._fh.write(state.to_json() + "\n")
        self._fh.flush()


# ---------------------------------------------------------------- fixture setup


def cmd_ensure_users(api: Api, manifest: Dict[str, Any]) -> None:
    rows = lib.fixture_user_rows(manifest["authors"])
    sql = lib.fixture_users_sql(rows)
    psql(sql)
    # verify logins work for everyone (throttled)
    ok = 0
    for email, _, _ in rows:
        try:
            api.login(email)
            ok += 1
        except RuntimeError as err:
            log("LOGIN FAIL %s: %s" % (email, err))
    log("ensure-users: %d/%d fixture users ready" % (ok, len(rows)))


def cmd_ensure_ips(api: Api, manifest: Dict[str, Any]) -> None:
    admin_token, _ = api.login(ADMIN_EMAIL)
    # generateSlug collapses pure-CJK names to ip-<utf8-len>; the collision
    # fallback appends the creator id, so equal-length names from ONE creator
    # still collide (500). Rotate a distinct creator per IP to stay unique.
    author_ids = [a["author_id"] for a in manifest["authors"]]
    ip_map_path = os.path.join(INJECTION_DIR, "ip_map.json")
    ip_map: Dict[str, int] = {}
    if os.path.exists(ip_map_path):
        with open(ip_map_path, "r", encoding="utf-8") as fh:
            ip_map = {k: int(v) for k, v in json.load(fh).items()}
    status, payload = api._request("GET", "/ips?limit=100", token=admin_token)
    if status != 200:
        raise RuntimeError("list ips failed: %d" % status)
    existing: Dict[str, int] = {}
    items = payload.get("ips") or payload.get("items") or payload.get("list") or []
    for row in items:
        existing[row.get("name", "")] = int(row.get("id", 0))
    for idx, ip_name in enumerate(manifest["ip_roster"]):
        if ip_name in ip_map:
            continue
        if ip_name in existing and existing[ip_name]:
            ip_map[ip_name] = existing[ip_name]
            continue
        creator_token, _ = api.login(author_email(author_ids[idx % len(author_ids)]))
        status, payload = api._request(
            "POST",
            "/ips",
            {
                "name": ip_name,
                "description": "%s 同人创作（合成语料 v2 演示数据）" % ip_name,
                "category": lib.ip_category_slug(ip_name),
                "tags": [lib.CATEGORY_LABEL["longform"], "语料v2"],
            },
            token=creator_token,
        )
        if status != 201:
            raise RuntimeError("create ip %s failed: %d %s" % (ip_name, status, str(payload)[:200]))
        ip_id = int(payload["ip"]["id"])
        status, payload = api._request("POST", "/admin/ips/%d/approve" % ip_id, {}, token=admin_token)
        if status != 200:
            raise RuntimeError("approve ip %s failed: %d %s" % (ip_name, status, str(payload)[:200]))
        ip_map[ip_name] = ip_id
        log("ensure-ips: created+approved %s -> %d" % (ip_name, ip_id))
    with open(ip_map_path, "w", encoding="utf-8") as fh:
        json.dump(ip_map, fh, ensure_ascii=False, indent=1)
    log("ensure-ips: %d IPs mapped" % len(ip_map))


def cmd_ensure_follows(api: Api, manifest: Dict[str, Any]) -> None:
    done_path = os.path.join(INJECTION_DIR, "follows.done")
    done: List[str] = []
    if os.path.exists(done_path):
        with open(done_path, "r", encoding="utf-8") as fh:
            done = [x.strip() for x in fh if x.strip()]
    done_set = set(done)
    edges = manifest["follower_edges"]
    # group by follower so each login covers all its edges inside the ttl
    by_follower: Dict[str, List[Dict[str, Any]]] = {}
    for edge in edges:
        by_follower.setdefault(edge["follower"], []).append(edge)
    id_cache: Dict[str, int] = {}

    def uid(author_id: str) -> int:
        if author_id not in id_cache:
            id_cache[author_id] = api.user_id(author_email(author_id))
        return id_cache[author_id]

    for follower_id, group in by_follower.items():
        pending = [e for e in group if "%s>%s" % (e["follower"], e["follows"]) not in done_set]
        if not pending:
            continue
        token, _ = api.login(author_email(follower_id))
        for edge in pending:
            sig = "%s>%s" % (edge["follower"], edge["follows"])
            status, payload = api._request("POST", "/users/%d/follow" % uid(edge["follows"]), {}, token=token)
            if status not in (200, 409):
                raise RuntimeError("follow %s failed: %d %s" % (sig, status, str(payload)[:200]))
            done.append(sig)
            with open(done_path, "a", encoding="utf-8") as fh:
                fh.write(sig + "\n")
    log("ensure-follows: %d edges materialized" % len(done))


# ---------------------------------------------------------------- injection


def find_content_by_key(key: str) -> Optional[int]:
    row = psql_scalar(
        "SELECT ct.content_item_id FROM content_tags ct WHERE ct.tag = %s LIMIT 1"
        % lib.sql_quote(lib.IDEMPOTENCY_TAG_PREFIX + key)
    )
    return int(row) if row else None


def index_ready(content_id: int, min_version: int) -> bool:
    row = psql_scalar(
        "SELECT COALESCE(MAX(rc.content_version), -1) FROM rag_chunks rc "
        "JOIN index_projection_status ips2 ON ips2.content_id = rc.content_id AND ips2.index_version = rc.index_version "
        "WHERE rc.content_id = %d AND ips2.is_current = TRUE AND ips2.state = 'ready'" % content_id
    )
    return row is not None and int(row) >= min_version


def wait_index_ready(content_id: int, min_version: int, timeout: float = 180.0) -> bool:
    deadline = time.time() + timeout
    while time.time() < deadline:
        if index_ready(content_id, min_version):
            return True
        time.sleep(2.0)
    return False


def ai_callback_pass(api: Api, content_id: int, task_id: str) -> None:
    content_json = json.dumps(
        {
            "dataId": "content:%d" % content_id,
            "taskId": task_id,
            "code": 200,
            "message": "OK",
            "results": [{"scene": "comment", "label": "non_ad", "suggestion": "pass"}],
        },
        separators=(",", ":"),
        ensure_ascii=False,
    )
    checksum = lib.ai_callback_checksum(GREEN_UID, GREEN_SEED, content_json)
    status, payload = api._request(
        "POST", "/internal/ai-callback", form={"content": content_json, "checksum": checksum}
    )
    if status != 200:
        raise RuntimeError("ai-callback failed (%d): %s" % (status, str(payload)[:200]))


def seed_initial_version(content_id: int, author_db_id: int, body_md: str) -> None:
    proc = subprocess.run(
        [VERSION_SEED_BIN, str(content_id), str(author_db_id), body_md],
        capture_output=True,
        text=True,
    )
    if proc.returncode != 0:
        raise RuntimeError("version_seed failed: %s" % proc.stderr.strip()[:300])


def wait_status_published(api: Api, token: str, content_id: int, timeout: float = 60.0) -> bool:
    deadline = time.time() + timeout
    while time.time() < deadline:
        status, payload = api._request("GET", "/contents/%d" % content_id, token=token)
        if status == 200 and payload.get("content", {}).get("status") == "published":
            return True
        time.sleep(1.5)
    return False


def version_titles(item_meta: Dict[str, Any], index_row: Dict[str, Any]) -> List[str]:
    """Ordered historical titles: version_history (older first) + current."""
    history = item_meta.get("version_history") or []
    titles = [h["title"] for h in history]
    titles.append(index_row["title"])
    return titles


def latest_version_row(content_id: int) -> Tuple[int, int]:
    """(version_number, version_row_id) of the latest active version."""
    row = psql_scalar(
        "SELECT version_number || ':' || id FROM content_versions "
        "WHERE content_item_id = %d AND status = 'active' ORDER BY version_number DESC LIMIT 1" % content_id
    )
    if not row:
        raise RuntimeError("no active version for content %d" % content_id)
    num, vid = row.split(":")
    return int(num), int(vid)


def apply_pr_version(
    api: Api,
    author_token: str,
    submitter_token: str,
    content_id: int,
    body_md: str,
    new_title: str,
    message: str,
) -> int:
    """One version bump through the real PR flow. Returns the new PR id."""
    _, base_version_id = latest_version_row(content_id)
    status, payload = api._request(
        "POST",
        "/pr",
        {
            "content_item_id": content_id,
            "base_version_id": base_version_id,
            "message": message,
            "new_text": body_md,
        },
        token=submitter_token,
    )
    if status == 409:
        # stale open PR from a previous crashed run: load and merge it first
        status2, list_payload = api._request("GET", "/contents/%d/prs" % content_id, token=author_token)
        prs = (list_payload.get("prs") or list_payload.get("items") or []) if status2 == 200 else []
        pr_id = None
        for pr in prs:
            if pr.get("status") == "open":
                pr_id = pr.get("id")
                break
        if pr_id is None:
            raise RuntimeError("PR 409 but no open PR found for content %d" % content_id)
    elif status != 201:
        raise RuntimeError("submit PR failed (%d): %s" % (status, str(payload)[:200]))
    else:
        pr_id = payload["pr"]["id"]
    status, payload = api._request(
        "POST", "/pr/%d/merge" % pr_id, {"merged_text": body_md}, token=author_token
    )
    if status != 200:
        raise RuntimeError("merge PR %s failed (%d): %s" % (pr_id, status, str(payload)[:200]))
    # title bump emits content.updated -> re-projection at the new version
    status, payload = api._request("PATCH", "/contents/%d" % content_id, {"title": new_title}, token=author_token)
    if status != 200:
        raise RuntimeError("patch title failed (%d): %s" % (status, str(payload)[:200]))
    return int(pr_id)


def pick_submitter(manifest: Dict[str, Any], author_id: str) -> str:
    """A different author (collaborator) submits the PR; deterministic."""
    authors = [a["author_id"] for a in manifest["authors"]]
    idx = authors.index(author_id)
    for offset in range(1, len(authors)):
        candidate = authors[(idx + offset) % len(authors)]
        if candidate != author_id:
            return candidate
    return authors[0]


def cmd_inject(api: Api, limit: Optional[int], only_key: Optional[str]) -> None:
    if not GREEN_SEED or not GREEN_UID:
        raise SystemExit("GREEN_SEED / GREEN_UID env vars are required (must match the server env)")
    index_rows, bodies, manifest = load_corpus()
    with open(os.path.join(INJECTION_DIR, "ip_map.json"), "r", encoding="utf-8") as fh:
        ip_map = {k: int(v) for k, v in json.load(fh).items()}

    # author-bucketed order: same author's items run together so one login
    # covers many items inside the 110s access-token window.
    by_author: Dict[str, List[Dict[str, Any]]] = {}
    for row in index_rows:
        by_author.setdefault(row["author_id"], []).append(row)

    work: List[Dict[str, Any]] = []
    for author_id in [a["author_id"] for a in manifest["authors"]]:
        work.extend(by_author.get(author_id, []))
    if only_key:
        work = [row for row in work if row["corpus_item_key"] == only_key]
    if limit is not None:
        work = work[:limit]

    checkpoint = Checkpoint()
    breaker = lib.CircuitBreaker()
    token_cache: Dict[str, Tuple[str, float]] = {}
    stats = {"done": 0, "skip": 0, "fail": 0}

    def token_for(email: str) -> str:
        cached = token_cache.get(email)
        if cached and cached[1] > time.time() + 5:
            return cached[0]
        access, exp = api.login(email)
        token_cache[email] = (access, float(exp))
        return access

    for row in work:
        key = row["corpus_item_key"]
        state = checkpoint.get(key)
        if state.done:
            stats["skip"] += 1
            continue
        body_row = bodies.get(key)
        if body_row is None:
            state.error = "body missing"
            checkpoint.save(state)
            breaker.record(False)
            continue
        meta = body_row["metadata"]
        author_email_ = author_email(row["author_id"])
        author_db_id = api.user_id(author_email_)
        try:
            # 1. create (idempotent via tag anchor)
            if state.content_id is None:
                found = find_content_by_key(key)
                if found:
                    state.content_id = found
                else:
                    token = token_for(author_email_)
                    status, payload = api._request(
                        "POST",
                        "/contents",
                        {
                            "title": lib.ensure_title(row["title"], body_row["body_md"], key),
                            "description": lib.build_description(body_row["body_md"]),
                            "zone": "fanwork",
                            "ip_id": ip_map[row["ip"]],
                            "category": lib.ip_category_slug(row["ip"]),
                            "content_type": "article",
                            "is_public": lib.visibility_to_is_public(row["visibility"]),
                            "allow_copy": True,
                            "tags": lib.build_tags(row),
                        },
                        token=token,
                    )
                    if status != 201:
                        raise RuntimeError("create failed (%d): %s" % (status, str(payload)[:300]))
                    state.content_id = int(payload["content"]["id"])
                    # GORM default:true zero-value pitfall: a create-body
                    # is_public=false lands as true (product bug, not fixed
                    # here); a follow-up PATCH is the reliable carrier.
                    if not lib.visibility_to_is_public(row["visibility"]):
                        s2, p2 = api._request(
                            "PATCH", "/contents/%d" % state.content_id,
                            {"is_public": False}, token=token,
                        )
                        if s2 != 200:
                            raise RuntimeError("restrict patch failed (%d): %s" % (s2, str(p2)[:200]))
                checkpoint.save(state)
            content_id = state.content_id

            # 2. initial version
            if state.versions_done < 1:
                seed_initial_version(content_id, author_db_id, body_row["body_md"])
                state.versions_done = 1
                checkpoint.save(state)

            # 3. moderation callback -> published
            if not state.published:
                ai_callback_pass(api, content_id, lib.stable_task_id(key, 1))
                token = token_for(author_email_)
                if not wait_status_published(api, token, content_id):
                    raise RuntimeError("not published after callback (timeout)")
                state.published = True
                checkpoint.save(state)

            # 4. version chain (titles evolve; body stays the latest text)
            titles = version_titles(meta, row)
            total_versions = max(int(meta.get("version_count", 1)), len(titles))
            while state.versions_done < total_versions:
                next_i = state.versions_done  # 0-based next version index
                title = titles[next_i] if next_i < len(titles) else titles[-1]
                author_token = token_for(author_email_)
                submitter = pick_submitter(manifest, row["author_id"])
                submitter_token = token_for(author_email(submitter))
                pr_id = apply_pr_version(
                    api, author_token, submitter_token, content_id, body_row["body_md"],
                    title, "corpus v2 版本链 %s v%d" % (key, next_i + 1),
                )
                state.pr_ids.append(pr_id)
                state.versions_done += 1
                checkpoint.save(state)

            # 5. projection ready at the final version
            if not wait_index_ready(content_id, state.versions_done):
                raise RuntimeError("index not ready for v%d" % state.versions_done)
            state.indexed = True
            state.error = None
            checkpoint.save(state)
            stats["done"] += 1
            breaker.record(True)
            if stats["done"] % 20 == 0:
                log("inject: %d done, %d skipped, %d failed" % (stats["done"], stats["skip"], stats["fail"]))
        except Exception as err:  # noqa: BLE001 - record and continue
            state.error = str(err)[:500]
            checkpoint.save(state)
            stats["fail"] += 1
            breaker.record(False)
            log("inject FAIL %s: %s" % (key, state.error))
            if breaker.tripped:
                log("CIRCUIT BREAKER TRIPPED: %s -- stopping run" % breaker.reason)
                break
        if breaker.tripped:
            break
    log("inject summary: %s breaker=%s" % (stats, breaker.reason))
    if stats["fail"]:
        log("failed keys: %s" % ", ".join(k for k, s in checkpoint.states.items() if s.error and not s.done))


def cmd_backfill_timestamps() -> None:
    """Align content_items.created_at with the corpus published_at window
    (>=90-day spread, spec section 1). Display-only column: no events, no
    index impact (projection freshness keys on updated_at)."""
    index_rows, _, _ = load_corpus()
    checkpoint = Checkpoint()
    updated = 0
    for row in index_rows:
        state = checkpoint.states.get(row["corpus_item_key"])
        if not state or not state.content_id:
            continue
        sql = (
            "UPDATE content_items SET created_at = %s::timestamptz WHERE id = %d"
            % (lib.sql_quote(row["published_at"]), state.content_id)
        )
        psql(sql)
        updated += 1
    log("backfill-timestamps: %d rows aligned" % updated)


def cmd_verify() -> None:
    total = psql_scalar("SELECT count(*) FROM content_tags WHERE tag LIKE 'c2:%'") or "0"
    distinct = psql_scalar("SELECT count(DISTINCT tag) FROM content_tags WHERE tag LIKE 'c2:%'") or "0"
    published = psql_scalar(
        "SELECT count(DISTINCT ci.id) FROM content_items ci JOIN content_tags ct ON ct.content_item_id = ci.id "
        "WHERE ct.tag LIKE 'c2:%' AND ci.status = 'published'"
    ) or "0"
    restricted = psql_scalar(
        "SELECT count(DISTINCT ci.id) FROM content_items ci JOIN content_tags ct ON ct.content_item_id = ci.id "
        "WHERE ct.tag LIKE 'c2:%' AND ci.is_public = false"
    ) or "0"
    chunks = psql_scalar(
        "SELECT count(DISTINCT rc.content_id) FROM rag_chunks rc JOIN content_tags ct ON ct.content_item_id = rc.content_id "
        "WHERE ct.tag LIKE 'c2:%'"
    ) or "0"
    chunk_rows = psql_scalar(
        "SELECT count(*) FROM rag_chunks rc JOIN content_tags ct ON ct.content_item_id = rc.content_id "
        "WHERE ct.tag LIKE 'c2:%'"
    ) or "0"
    versions = psql_scalar(
        "SELECT count(*) FROM content_versions cv JOIN content_tags ct ON ct.content_item_id = cv.content_item_id "
        "WHERE ct.tag LIKE 'c2:%'"
    ) or "0"
    log(
        "verify: tags=%s distinct_keys=%s published=%s restricted=%s indexed_contents=%s chunk_rows=%s version_rows=%s"
        % (total, distinct, published, restricted, chunks, chunk_rows, versions)
    )
    failed = [(k, s.error) for k, s in lib.load_checkpoint(CHECKPOINT_PATH).items() if s.error and not s.done]
    if failed:
        log("failed items (%d):" % len(failed))
        for key, error in failed[:30]:
            log("  %s: %s" % (key, error))


def cmd_export_mapping() -> None:
    checkpoint = lib.load_checkpoint(CHECKPOINT_PATH)
    index_rows, _, _ = load_corpus()
    meta_by_key = {row["corpus_item_key"]: row for row in index_rows}
    os.makedirs(INJECTION_DIR, exist_ok=True)
    count = 0
    with open(MAPPING_PATH, "w", encoding="utf-8") as fh:
        for key in sorted(meta_by_key):
            state = checkpoint.get(key)
            if state is None:
                state = lib.ItemState(key=key)
            row = meta_by_key[key]
            fh.write(
                json.dumps(
                    {
                        "corpus_item_key": key,
                        "content_id": state.content_id,
                        "indexed": state.indexed,
                        "versions": state.versions_done,
                        "ip": row["ip"],
                        "category": row["category"],
                        "visibility": row["visibility"],
                        "language": row["language"],
                        "temperature": row["temperature"],
                        "title_form": row["title_form"],
                        "is_public": lib.visibility_to_is_public(row["visibility"]),
                        "author_id": row["author_id"],
                        "published_at": row["published_at"],
                    },
                    ensure_ascii=False,
                    separators=(",", ":"),
                )
                + "\n"
            )
            count += 1
    log("export-mapping: %d rows -> %s" % (count, MAPPING_PATH))


def cmd_smoke(api: Api, n: int) -> None:
    """Exact-title retrieval smoke: n random exact-form items must hit."""
    import random

    index_rows, _, _ = load_corpus()
    checkpoint = lib.load_checkpoint(CHECKPOINT_PATH)
    # restricted items are invisible to the anonymous viewer by design; the
    # exact-title smoke samples the public exact pool (their invisibility is
    # separately covered by the golden-set visibility layer)
    exact = [
        r
        for r in index_rows
        if r["title_form"] == "exact"
        and r.get("visibility") == "public"
        and checkpoint.get(r["corpus_item_key"]) is not None
        and checkpoint[r["corpus_item_key"]].done
    ]
    random.seed(20260903)
    sample = random.sample(exact, min(n, len(exact)))
    viewer, _ = api.login("viewer-anon" + lib.CORPUS_EMAIL_SUFFIX)
    hits = 0
    results = []
    for row in sample:
        state = checkpoint[row["corpus_item_key"]]
        query = urllib.parse.quote(row["title"])
        status, payload = api._request("GET", "/contents/search?q=" + query, token=viewer)
        found = False
        rank = -1
        if status == 200:
            items = payload.get("contents") or payload.get("items") or payload.get("list") or []
            for i, item in enumerate(items[:10]):
                if int(item.get("id", 0)) == state.content_id:
                    found = True
                    rank = i + 1
                    break
        hits += 1 if found else 0
        results.append({"key": row["corpus_item_key"], "content_id": state.content_id, "hit": found, "rank": rank})
        if not found:
            log("smoke MISS %s (status=%d)" % (row["corpus_item_key"], status))
    log("smoke: %d/%d exact-title hits" % (hits, len(sample)))
    with open(os.path.join(INJECTION_DIR, "smoke-result.json"), "w", encoding="utf-8") as fh:
        json.dump({"hits": hits, "total": len(sample), "results": results}, fh, ensure_ascii=False, indent=1)


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="cmd", required=True)
    sub.add_parser("ensure-users")
    sub.add_parser("ensure-ips")
    sub.add_parser("ensure-follows")
    p_inject = sub.add_parser("inject")
    p_inject.add_argument("--limit", type=int, default=None)
    p_inject.add_argument("--only", type=str, default=None)
    sub.add_parser("backfill-timestamps")
    sub.add_parser("verify")
    p_smoke = sub.add_parser("smoke")
    p_smoke.add_argument("--n", type=int, default=20)
    sub.add_parser("export-mapping")
    args = parser.parse_args()

    os.makedirs(INJECTION_DIR, exist_ok=True)
    api = Api()
    _, _, manifest = load_corpus()

    if args.cmd == "ensure-users":
        cmd_ensure_users(api, manifest)
    elif args.cmd == "ensure-ips":
        cmd_ensure_ips(api, manifest)
    elif args.cmd == "ensure-follows":
        cmd_ensure_follows(api, manifest)
    elif args.cmd == "inject":
        cmd_inject(api, args.limit, args.only)
    elif args.cmd == "backfill-timestamps":
        cmd_backfill_timestamps()
    elif args.cmd == "verify":
        cmd_verify()
    elif args.cmd == "smoke":
        cmd_smoke(api, args.n)
    elif args.cmd == "export-mapping":
        cmd_export_mapping()


if __name__ == "__main__":
    main()
