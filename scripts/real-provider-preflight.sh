#!/usr/bin/env bash
set -Eeuo pipefail

# Validate local real-provider evidence configuration without making a network
# request. The key is only inspected for presence/shape and is never printed.
# Mirrors the fail-closed rules the Go factory enforces (backend/internal/pkg/
# llm/factory.go): cross-vendor split embedding must carry its own credential,
# and the native qwen adapter is retired for agent use.

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
if [[ -z "${AGENT_LLM_API_KEY:-}" && -f "$ROOT_DIR/.env" ]]; then
    set -a
    . "$ROOT_DIR/.env"
    set +a
fi

die() {
    printf 'provider preflight blocked: %s\n' "$1" >&2
    exit 1
}

trimmed() {
    printf '%s' "${1:-}" | awk '{$1=$1; print}'
}

lower() {
    printf '%s' "${1:-}" | tr '[:upper:]' '[:lower:]'
}

key="$(trimmed "${AGENT_LLM_API_KEY:-}")"
provider="$(trimmed "${AGENT_LLM_PROVIDER:-}")"
chat_model="$(trimmed "${AGENT_LLM_MODEL:-}")"
api_base="$(trimmed "${AGENT_LLM_API_BASE:-}")"
embedding_model="$(trimmed "${AGENT_EMBEDDING_MODEL:-}")"
embedding_provider="$(trimmed "${AGENT_EMBEDDING_PROVIDER:-}")"
embedding_api_base="$(trimmed "${AGENT_EMBEDDING_API_BASE:-}")"
embedding_key="$(trimmed "${AGENT_EMBEDDING_API_KEY:-}")"
embedding_group_id="$(trimmed "${AGENT_EMBEDDING_GROUP_ID:-}")"
index_embedding_model="$(trimmed "${RAG_INDEX_EMBEDDING_MODEL:-$embedding_model}")"
agent_enabled="$(trimmed "${AGENT_WEB_AGENT_ENABLED:-false}")"
provider_lower="$(lower "$provider")"
embedding_provider_lower="$(lower "$embedding_provider")"

[[ -n "$key" ]] || die 'AGENT_LLM_API_KEY is empty'
[[ "$key" != *'your-'* && "$key" != *'change-'* && "$key" != 'placeholder' && "$key" != 'local-disabled-no-network' ]] || die 'AGENT_LLM_API_KEY is still a placeholder'
[[ ${#key} -ge 8 ]] || die 'AGENT_LLM_API_KEY is unexpectedly short'
[[ "$(lower "$agent_enabled")" == '1' || "$(lower "$agent_enabled")" == 'true' ]] || die 'set AGENT_WEB_AGENT_ENABLED=true for real Agent smoke'
[[ -n "$provider" ]] || die 'AGENT_LLM_PROVIDER is empty'
[[ -n "$chat_model" ]] || die 'AGENT_LLM_MODEL is empty'
[[ -n "$api_base" ]] || die 'AGENT_LLM_API_BASE is empty'
[[ -n "$embedding_model" ]] || die 'AGENT_EMBEDDING_MODEL is empty'
[[ "$embedding_model" == "$index_embedding_model" ]] || die 'AGENT_EMBEDDING_MODEL must match RAG_INDEX_EMBEDDING_MODEL'
[[ -n "$embedding_api_base" ]] || die 'AGENT_EMBEDDING_API_BASE is empty'
[[ "$embedding_api_base" == https://* ]] || die 'AGENT_EMBEDDING_API_BASE must use HTTPS'

case "$provider_lower" in
    minimax)
        [[ "$api_base" == https://api.minimaxi.com* ]] || die 'MiniMax chat evidence must use https://api.minimaxi.com (no trailing /v1)'
        ;;
    openai_compat)
        [[ "$api_base" == https://* ]] || die 'openai_compat evidence must use an HTTPS API base'
        ;;
    qwen)
        die 'provider "qwen" (native DashScope adapter) is retired for agent use: use openai_compat with https://dashscope.aliyuncs.com/compatible-mode'
        ;;
    *) die "unsupported provider: $provider" ;;
esac

# Embedding provider: empty = follow the chat provider. A different provider
# is cross-vendor split wiring and must carry its own credential — borrowing
# the chat key across vendors always 401s (factory fails closed on the same
# rule).
if [[ -n "$embedding_provider_lower" && "$embedding_provider_lower" != "$provider_lower" ]]; then
    [[ -n "$embedding_key" ]] || die 'AGENT_EMBEDDING_PROVIDER differs from AGENT_LLM_PROVIDER: AGENT_EMBEDDING_API_KEY is required (fail-closed, never borrow the chat key)'
fi
case "$embedding_provider_lower" in
    "" | openai_compat | minimax) ;;
    qwen) die 'embedding provider "qwen" is retired: use openai_compat with the DashScope compatible-mode base' ;;
    *) die "unsupported embedding provider: $embedding_provider" ;;
esac
if [[ "$embedding_provider_lower" == "minimax" ]]; then
    [[ "$embedding_api_base" == https://api.minimax.chat* ]] || die 'MiniMax embo embeddings must use https://api.minimax.chat'
    [[ -n "$embedding_group_id" ]] || die 'AGENT_EMBEDDING_GROUP_ID is required for MiniMax embo embeddings'
fi

# Canonical embedding identity: text-embedding-v4 must pin 1536 dimensions
# (config.yaml agent.embedding_dimensions; no env override exists).
if [[ "$embedding_model" == "text-embedding-v4" ]]; then
    dims="$(awk '/^agent:/{inagent=1; next} inagent && /embedding_dimensions:/ {gsub(/[" ]/, "", $2); print $2; exit}' "$ROOT_DIR/backend/config.yaml")"
    [[ "$dims" == "1536" ]] || die "text-embedding-v4 requires embedding_dimensions 1536 (backend/config.yaml has ${dims:-none})"
fi

# Rerank pairing: a set primary key must ride the DashScope host with
# qwen3-rerank (gte-rerank is offline); the SiliconFlow fallback needs its own
# key. No rerank keys = rerank leg off (RRF order), which is valid.
if [[ -n "$(trimmed "${RAG_RERANK_API_KEY:-}")" ]]; then
    rerank_base="$(trimmed "${RAG_RERANK_API_BASE:-https://dashscope.aliyuncs.com}")"
    [[ "$rerank_base" == https://dashscope.aliyuncs.com* ]] || die 'RAG_RERANK_API_BASE must be the DashScope host for qwen3-rerank'
fi
if [[ -n "$(trimmed "${RAG_RERANK_FALLBACK_API_KEY:-}")" ]]; then
    rerank_fallback_base="$(trimmed "${RAG_RERANK_FALLBACK_API_BASE:-https://api.siliconflow.cn}")"
    [[ "$rerank_fallback_base" == https://* ]] || die 'RAG_RERANK_FALLBACK_API_BASE must use HTTPS'
fi

if [[ -f .env ]]; then
    mode="$(stat -f '%Lp' .env 2>/dev/null || stat -c '%a' .env 2>/dev/null || true)"
    [[ "$mode" == '600' || "$mode" == '400' ]] || die ".env permissions must be 600 or 400 (found $mode)"
fi

if git ls-files --error-unmatch .env >/dev/null 2>&1; then
    die '.env is tracked by git; remove it from the index before using a real key'
fi

printf 'provider preflight passed\n'
printf 'provider=%s\nchat_model=%s\napi_base=%s\nembedding_provider=%s\nembedding_model=%s\nembedding_api_base=%s\napi_key=present (not logged)\n' \
    "$provider" "$chat_model" "$api_base" "${embedding_provider:-<follows chat provider>}" "$embedding_model" "$embedding_api_base"
