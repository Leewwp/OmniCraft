#!/usr/bin/env bash
set -Eeuo pipefail

# Validate local real-provider evidence configuration without making a network
# request. The key is only inspected for presence/shape and is never printed.

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

key="$(trimmed "${AGENT_LLM_API_KEY:-}")"
provider="$(trimmed "${AGENT_LLM_PROVIDER:-}")"
chat_model="$(trimmed "${AGENT_LLM_MODEL:-}")"
api_base="$(trimmed "${AGENT_LLM_API_BASE:-}")"
embedding_model="$(trimmed "${AGENT_EMBEDDING_MODEL:-}")"
embedding_api_base="$(trimmed "${AGENT_EMBEDDING_API_BASE:-}")"
embedding_group_id="$(trimmed "${AGENT_EMBEDDING_GROUP_ID:-}")"
index_embedding_model="$(trimmed "${RAG_INDEX_EMBEDDING_MODEL:-$embedding_model}")"
agent_enabled="$(trimmed "${AGENT_WEB_AGENT_ENABLED:-false}")"
agent_enabled_lower="$(printf '%s' "$agent_enabled" | tr '[:upper:]' '[:lower:]')"
provider_lower="$(printf '%s' "$provider" | tr '[:upper:]' '[:lower:]')"

[[ -n "$key" ]] || die 'AGENT_LLM_API_KEY is empty'
[[ "$key" != *'your-'* && "$key" != *'change-'* && "$key" != 'placeholder' && "$key" != 'local-disabled-no-network' ]] || die 'AGENT_LLM_API_KEY is still a placeholder'
[[ ${#key} -ge 8 ]] || die 'AGENT_LLM_API_KEY is unexpectedly short'
[[ "$agent_enabled" == '1' || "$agent_enabled_lower" == 'true' ]] || die 'set AGENT_WEB_AGENT_ENABLED=true for real Agent smoke'
[[ -n "$provider" ]] || die 'AGENT_LLM_PROVIDER is empty'
[[ -n "$chat_model" ]] || die 'AGENT_LLM_MODEL is empty'
[[ -n "$api_base" ]] || die 'AGENT_LLM_API_BASE is empty'
[[ -n "$embedding_model" ]] || die 'AGENT_EMBEDDING_MODEL is empty'
[[ "$embedding_model" == "$index_embedding_model" ]] || die 'AGENT_EMBEDDING_MODEL must match RAG_INDEX_EMBEDDING_MODEL'
[[ -n "$embedding_api_base" ]] || die 'AGENT_EMBEDDING_API_BASE is empty'
[[ "$embedding_api_base" == https://* ]] || die 'AGENT_EMBEDDING_API_BASE must use HTTPS'

case "$provider_lower" in
    minimax)
        [[ "$api_base" == https://api.minimaxi.com* ]] || die 'MiniMax evidence must use https://api.minimaxi.com'
        [[ "$embedding_api_base" == https://api.minimax.chat* ]] || die 'MiniMax embo-01 evidence must use https://api.minimax.chat'
        [[ -n "$embedding_group_id" ]] || die 'AGENT_EMBEDDING_GROUP_ID is required for MiniMax embo-01'
        ;;
    openai_compat)
        [[ "$api_base" == https://* ]] || die 'openai_compat evidence must use an HTTPS API base'
        ;;
    qwen)
        [[ "$api_base" == https://dashscope.aliyuncs.com/compatible-mode* || "$api_base" == https://* ]] || die 'Qwen evidence must use an HTTPS API base'
        ;;
    *) die "unsupported provider: $provider" ;;
esac

if [[ -f .env ]]; then
    mode="$(stat -f '%Lp' .env 2>/dev/null || stat -c '%a' .env 2>/dev/null || true)"
    [[ "$mode" == '600' || "$mode" == '400' ]] || die ".env permissions must be 600 or 400 (found $mode)"
fi

if git ls-files --error-unmatch .env >/dev/null 2>&1; then
    die '.env is tracked by git; remove it from the index before using a real key'
fi

printf 'provider preflight passed\n'
printf 'provider=%s\nchat_model=%s\napi_base=%s\nembedding_model=%s\nembedding_api_base=%s\nembedding_group_id=present\napi_key=present (not logged)\n' \
    "$provider" "$chat_model" "$api_base" "$embedding_model" "$embedding_api_base"
