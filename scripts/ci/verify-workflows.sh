#!/usr/bin/env bash
# Contract-checks .github/workflows/ci.yml and .github/workflows/tauri-ci.yml:
# stable job names, triggers, concurrency, minimal permissions, SHA-pinned
# actions, lockfile-derived cache keys, always-upload evidence artifacts with
# the 30/90-day retention policy, no production secret references, and the
# Windows Tauri path-detected no-op job.
# Usage: bash scripts/ci/verify-workflows.sh [-WorkflowsDir <dir>]
set -u

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
WORKFLOWS_DIR="$REPO_ROOT/.github/workflows"

while [ $# -gt 0 ]; do
  case "$1" in
    -WorkflowsDir)
      if [ $# -lt 2 ]; then
        echo "missing value for -WorkflowsDir" >&2
        exit 2
      fi
      shift
      WORKFLOWS_DIR="$1"
      ;;
    *)
      echo "unknown argument: $1" >&2
      exit 2
      ;;
  esac
  shift
done

export OMNICRAFT_WORKFLOWS_DIR="$WORKFLOWS_DIR"

python3 - <<'PY'
import json
import os
import re
import sys

workflows_dir = os.environ["OMNICRAFT_WORKFLOWS_DIR"]


def fail(message):
    print(f"workflow contract violation: {message}", file=sys.stderr)
    sys.exit(1)


def strip_comment(line):
    quote = None
    for i, ch in enumerate(line):
        if quote:
            if ch == quote:
                quote = None
        elif ch in "\"'":
            quote = ch
        elif ch == "#" and (i == 0 or line[i - 1] in " \t"):
            return line[:i]
    return line


def parse_scalar(text):
    text = text.strip()
    if not text:
        return ""
    if len(text) >= 2 and text[0] == text[-1] and text[0] in "\"'":
        if text[0] == '"':
            return json.loads(text)
        return text[1:-1].replace("''", "'")
    if text == "true":
        return True
    if text == "false":
        return False
    if text == "null":
        return None
    if re.fullmatch(r"-?\d+", text):
        return int(text)
    if text.startswith("["):
        inner = text[1:-1].strip()
        if not inner:
            return []
        return [parse_scalar(part) for part in split_flow(inner)]
    return text


def split_flow(inner):
    parts = []
    current = ""
    quote = None
    for ch in inner:
        if quote:
            current += ch
            if ch == quote:
                quote = None
        elif ch in "\"'":
            quote = ch
            current += ch
        elif ch == ",":
            parts.append(current)
            current = ""
        else:
            current += ch
    parts.append(current)
    return parts


def parse_document(text):
    lines = []
    for raw in text.splitlines():
        line = strip_comment(raw)
        if not line.strip():
            continue
        stripped = line.lstrip(" ")
        if "\t" in line[: len(line) - len(stripped)]:
            raise ValueError("tab indentation is not supported")
        lines.append((len(line) - len(stripped), stripped))
    if not lines:
        return {}
    position = [0]

    def parse_block(indent):
        if position[0] < len(lines) and lines[position[0]][0] == indent and lines[position[0]][1].startswith("-"):
            return parse_sequence(indent)
        node = {}
        while position[0] < len(lines):
            cur_indent, content = lines[position[0]]
            if cur_indent < indent:
                return node
            if cur_indent > indent:
                raise ValueError(f"unexpected indentation: {content}")
            if content.startswith("-"):
                raise ValueError(f"unexpected sequence entry: {content}")
            if ":" not in content:
                raise ValueError(f"expected key: value entry: {content}")
            key, _, rest = content.partition(":")
            key = key.strip()
            rest = rest.strip()
            position[0] += 1
            if not rest:
                if position[0] < len(lines) and lines[position[0]][0] > indent:
                    node[key] = parse_block(lines[position[0]][0])
                else:
                    node[key] = {}
            else:
                node[key] = parse_scalar(rest)
        return node

    def parse_sequence(indent):
        seq = []
        while position[0] < len(lines):
            cur_indent, content = lines[position[0]]
            if cur_indent < indent:
                return seq
            if cur_indent != indent or not content.startswith("-"):
                raise ValueError(f"expected sequence entry: {content}")
            rest = content[1:].strip()
            position[0] += 1
            if not rest:
                if position[0] < len(lines) and lines[position[0]][0] > indent:
                    seq.append(parse_block(lines[position[0]][0]))
                else:
                    seq.append({})
            elif ":" in rest:
                key, _, value = rest.partition(":")
                key = key.strip()
                value = value.strip()
                item = {}
                if not value:
                    if position[0] < len(lines) and lines[position[0]][0] > indent:
                        item[key] = parse_block(lines[position[0]][0])
                    else:
                        item[key] = {}
                else:
                    item[key] = parse_scalar(value)
                if position[0] < len(lines) and lines[position[0]][0] > indent and not lines[position[0]][1].startswith("-"):
                    extra = parse_block(lines[position[0]][0])
                    if not isinstance(extra, dict):
                        raise ValueError("expected mapping continuation for sequence item")
                    item.update(extra)
                seq.append(item)
            else:
                seq.append(parse_scalar(rest))
        return seq

    doc = parse_block(0)
    if position[0] < len(lines):
        raise ValueError("trailing content after document")
    return doc


def steps_of(job):
    if not isinstance(job, dict):
        return []
    return job.get("steps") if isinstance(job.get("steps"), list) else []


def walk_steps(doc):
    jobs = doc.get("jobs")
    if not isinstance(jobs, dict):
        return []
    for job in jobs.values():
        for step in steps_of(job):
            yield step


def check_shared(doc, name, required_jobs, lockfile_keys):
    on = doc.get("on")
    if not isinstance(on, dict):
        fail(f"{name}: missing 'on' triggers")
    if "pull_request" not in on:
        fail(f"{name}: pull_request trigger is required")
    if "push" not in on:
        fail(f"{name}: push trigger is required")
    if isinstance(on["pull_request"], dict) and "paths" in on["pull_request"]:
        fail(f"{name}: workflow-level pull_request paths filter is forbidden")

    concurrency = doc.get("concurrency")
    if not isinstance(concurrency, dict):
        fail(f"{name}: concurrency must be configured")
    if concurrency.get("cancel-in-progress") is not True:
        fail(f"{name}: concurrency must cancel in progress runs")

    permissions = doc.get("permissions")
    if permissions != {"contents": "read"}:
        fail(f"{name}: permissions must be minimal (contents: read only)")

    jobs = doc.get("jobs")
    if not isinstance(jobs, dict):
        fail(f"{name}: jobs must be a mapping")
    for job_name in required_jobs:
        if job_name not in jobs:
            fail(f"{name}: required job '{job_name}' is missing")

    actions_pin = re.compile(r"^[\w.-]+/[\w.-]+@[0-9a-f]{40}$")
    cache_key_count = 0
    for step in walk_steps(doc):
        uses = step.get("uses", "")
        if uses:
            if not actions_pin.match(uses):
                fail(f"{name}: action reference must be SHA-pinned: {uses}")
        if str(uses).startswith("actions/cache@"):
            cache_key_count += 1
            key = str(step.get("with", {}).get("key", ""))
            if "hashFiles(" not in key:
                fail(f"{name}: cache key must derive from lockfiles via hashFiles")
        if str(uses).startswith("actions/upload-artifact@"):
            step_if = str(step.get("if", ""))
            if "always()" not in step_if:
                fail(f"{name}: evidence artifact upload must use if: always()")
            retention = str(step.get("with", {}).get("retention-days", ""))
            if "30" not in retention or "90" not in retention:
                fail(f"{name}: artifact retention must express the 30/90-day policy")

    for lockfile in lockfile_keys:
        if lockfile not in open(os.path.join(workflows_dir, name), encoding="utf-8").read():
            fail(f"{name}: cache keys must reference {lockfile}")

    if cache_key_count == 0:
        fail(f"{name}: at least one actions/cache step is required")


ci_path = os.path.join(workflows_dir, "ci.yml")
tauri_path = os.path.join(workflows_dir, "tauri-ci.yml")
for path in (ci_path, tauri_path):
    if not os.path.exists(path):
        fail(f"{os.path.basename(path)}: workflow file missing")
    raw = open(path, encoding="utf-8").read()
    if "secrets." in raw:
        fail(f"{os.path.basename(path)}: production secret references are forbidden")
    try:
        doc = parse_document(raw)
    except ValueError as e:
        fail(f"{os.path.basename(path)}: invalid YAML: {e}")

if os.path.exists(ci_path):
    with open(ci_path, encoding="utf-8") as f:
        ci = parse_document(f.read())
    check_shared(ci, "ci.yml", ["backend", "frontend", "docs", "project-gate"],
                 ["go.sum", "package-lock.json"])
    dispatch = ci.get("on", {}).get("workflow_dispatch")
    if not isinstance(dispatch, dict) or "inputs" not in dispatch:
        fail("ci.yml: workflow_dispatch must guard the failure_probe input")
    probe = dispatch.get("inputs", {}).get("failure_probe")
    if not isinstance(probe, dict) or probe.get("type") != "boolean" or probe.get("default") is not False:
        fail("ci.yml: failure_probe input must be a boolean defaulting to false")
    project_gate = ci.get("jobs", {}).get("project-gate")
    if not isinstance(project_gate, dict):
        fail("ci.yml: project-gate job is missing")
    if "always()" not in str(project_gate.get("if", "")):
        fail("ci.yml: project-gate must use if: always() so dependency failures cannot skip the required check")
    dependency_results = {
        "BACKEND_RESULT": "needs.backend.result",
        "FRONTEND_RESULT": "needs.frontend.result",
        "DOCS_RESULT": "needs.docs.result",
    }
    dependency_guard = False
    for step in steps_of(project_gate):
        env = step.get("env") if isinstance(step, dict) else None
        run = str(step.get("run", "")) if isinstance(step, dict) else ""
        if not isinstance(env, dict):
            continue
        if all(expected in str(env.get(name, "")) for name, expected in dependency_results.items()) and all(
            f'test "${name}" = success' in run for name in dependency_results
        ):
            dependency_guard = True
            break
    if not dependency_guard:
        fail("ci.yml: project-gate must fail unless backend, frontend, and docs all succeeded")

if os.path.exists(tauri_path):
    with open(tauri_path, encoding="utf-8") as f:
        tauri = parse_document(f.read())
    check_shared(tauri, "tauri-ci.yml", ["tauri-windows"],
                 ["Cargo.lock", "package-lock.json"])
    jobs = tauri.get("jobs", {})
    tauri_job = jobs.get("tauri-windows")
    if not isinstance(tauri_job, dict):
        fail("tauri-ci.yml: tauri-windows job is missing")
    if tauri_job.get("runs-on") != "windows-latest":
        fail("tauri-ci.yml: tauri-windows must run on windows-latest")
    noop_found = False
    detect_found = False
    for step in steps_of(tauri_job):
        step_if = str(step.get("if", ""))
        if "needs.detect.outputs.changed != 'true'" in step_if:
            noop_found = True
    detect_job = jobs.get("detect")
    if isinstance(detect_job, dict):
        for step in steps_of(detect_job):
            if "GITHUB_OUTPUT" in str(step.get("run", "")) and "changed" in str(step.get("run", "")):
                detect_found = True
    if not detect_found:
        fail("tauri-ci.yml: path detection must publish a changed output")
    if not noop_found:
        fail("tauri-ci.yml: tauri-windows must keep the explicit no-op branch")

print("workflow contract valid")
PY
