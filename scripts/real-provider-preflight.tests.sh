#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

bash -n "$ROOT_DIR/scripts/real-provider-preflight.sh"

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT
cd "$TMP_DIR"

if AGENT_LLM_API_KEY=placeholder \
    AGENT_WEB_AGENT_ENABLED=true \
    AGENT_LLM_PROVIDER=minimax \
    AGENT_LLM_MODEL=MiniMax-M1 \
    AGENT_LLM_API_BASE=https://api.minimaxi.com \
    AGENT_EMBEDDING_MODEL=embo-01 \
    AGENT_EMBEDDING_API_BASE=https://api.minimax.chat \
    AGENT_EMBEDDING_GROUP_ID=test-group \
    RAG_INDEX_EMBEDDING_MODEL=embo-01 \
    bash "$ROOT_DIR/scripts/real-provider-preflight.sh" >/dev/null 2>&1; then
    echo 'placeholder key must be rejected' >&2
    exit 1
fi

AGENT_LLM_API_KEY=abcdefghijk \
AGENT_WEB_AGENT_ENABLED=true \
AGENT_LLM_PROVIDER=minimax \
AGENT_LLM_MODEL=MiniMax-M1 \
AGENT_LLM_API_BASE=https://api.minimaxi.com \
AGENT_EMBEDDING_MODEL=embo-01 \
AGENT_EMBEDDING_API_BASE=https://api.minimax.chat \
AGENT_EMBEDDING_GROUP_ID=test-group \
RAG_INDEX_EMBEDDING_MODEL=embo-01 \
bash "$ROOT_DIR/scripts/real-provider-preflight.sh" >/dev/null

echo 'real provider preflight script contracts passed'
