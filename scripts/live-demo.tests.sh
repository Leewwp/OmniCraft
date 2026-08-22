#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

bash -n "$ROOT_DIR/scripts/live-demo.sh"
python3 -m py_compile "$ROOT_DIR/scripts/seed_local_rag_projection.py"
bash -n "$ROOT_DIR/ops/opensearch/seed.sh"

grep -Fq 'set -Eeuo pipefail' "$ROOT_DIR/scripts/live-demo.sh"
grep -Fq 'will not create it' "$ROOT_DIR/scripts/live-demo.sh"
grep -Fq 'compose --profile full-infra up -d opensearch' "$ROOT_DIR/scripts/live-demo.sh"
grep -Fq 'container lifecycle is owned by another local Compose project' "$ROOT_DIR/scripts/live-demo.sh"
grep -Fq 'OMNICRAFT_LIVE_DEMO_BACKEND_FALLBACK_PORT' "$ROOT_DIR/scripts/live-demo.sh"
grep -Fq 'OMNICRAFT_LIVE_DEMO_METRICS_FALLBACK_PORT' "$ROOT_DIR/scripts/live-demo.sh"
grep -Fq 'exec nohup env' "$ROOT_DIR/scripts/live-demo.sh"
grep -Fq 'compose --profile full-infra up -d jaeger otel-collector' "$ROOT_DIR/scripts/live-demo.sh"
grep -Fq 'omnicraft-rag-read' "$ROOT_DIR/ops/opensearch/seed.sh"
grep -Fq 'DELETE FROM notifications' "$ROOT_DIR/scripts/seed_local_rich_data.py"
grep -Fq 'content_items_id_seq' "$ROOT_DIR/scripts/seed_local_rich_data.py"
grep -Fq 'ON_ERROR_STOP=1' "$ROOT_DIR/scripts/live-demo.sh"
if grep -Eq '(^|[[:space:]])(git[[:space:]]+push|docker[[:space:]]+compose[^\n]*down[[:space:]]+-v)' "$ROOT_DIR/scripts/live-demo.sh"; then
    echo 'live-demo script contains a destructive/unapproved command' >&2
    exit 1
fi

echo 'live-demo script contracts passed'
