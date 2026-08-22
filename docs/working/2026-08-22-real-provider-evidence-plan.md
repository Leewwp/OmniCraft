# Real Provider Evidence Plan

> 创建日期：2026-08-22
> **预计失效日期**: 2026-10-22

## Decision

The minimum resume-oriented evidence slice is one real provider contract:

1. Use MiniMax's OpenAI-compatible Chat Completions API for chat, and its
   legacy `embo-01` embedding API as a separate endpoint. The existing provider
   factory supports streaming, tool calls, and the required embedding wire
   shape after the endpoint split in this change.
2. Re-run the existing 63-case retrieval evaluation with real embeddings and
   record the same-run keyword baseline, model identity, vector dimension,
   dataset checksum, and artifact path.
3. Run `cmd/agent-smoke` for a bounded real chat/stream contract. This proves
   provider connectivity and output shape; it is not a groundedness or
   production-quality metric.
4. Only add a real LLM answer-evaluation runner after the provider smoke is
   healthy. Groundedness, answer relevance, and first-token latency must be
   measured from actual Agent answers, not inferred from retrieval metrics.

Jaeger full-chain evidence is a second batch. It is worth doing only if the
resume keeps an explicit OpenTelemetry/trace-propagation claim. ClamAV/EICAR
full-stack evidence is a third, independent batch and can remain deferred
unless archive malware scanning is a chosen resume highlight.

## Provider choice

| Provider | Recommended use now | Constraint |
|---|---|---|
| MiniMax | First combined chat + embedding contract | Confirm the account has API quota and the selected chat model is enabled; Coding Plan access may not equal API quota. |
| DeepSeek | Optional chat-only smoke | The current provider does not supply the embedding contract used by this repository. |
| Aliyun Bailian/Qwen | Optional later comparison | Existing Qwen chat path is not the first choice for Agent tool-loop evidence; use it as a separate comparison only. |

Do not run all three providers for the minimum slice. A second chat provider is
useful as a resilience/comparison story, but it does not add as much resume
value as a clean first provider run with reproducible artifacts.

## Local configuration

Copy `.env.example` to `.env`, set mode `600`, and fill only the local values:

```dotenv
AGENT_LLM_API_KEY=<short-lived-budget-limited-key>
AGENT_WEB_AGENT_ENABLED=true
AGENT_LLM_PROVIDER=minimax
AGENT_LLM_MODEL=MiniMax-M3
AGENT_LLM_API_BASE=https://api.minimaxi.com
AGENT_EMBEDDING_MODEL=embo-01
AGENT_EMBEDDING_API_BASE=https://api.minimax.chat
AGENT_EMBEDDING_GROUP_ID=<your MiniMax GroupId>
RAG_INDEX_EMBEDDING_MODEL=embo-01
```

The index model must match the Agent embedding model. The runtime now mirrors
`AGENT_EMBEDDING_MODEL` into the index setting when the explicit index variable
is omitted; setting both explicitly is recommended for an evidence run.
MiniMax's documented embedding endpoint requires `GroupId`; it is separate from
the Chat base URL and must not be omitted.

Before any network call, run:

```bash
chmod 600 .env
set -a; source .env; set +a
bash scripts/real-provider-preflight.sh
```

The preflight only checks shape, provider/base compatibility, permissions, and
model consistency. It never sends the key to a provider and never prints it in
full.

## Evidence sequence after configuration

1. Run the preflight and then `go run ./cmd/agent-smoke`; preserve the JSON
   output after removing any user content or secret-bearing fields.
2. Seed the dedicated local evaluation database and OpenSearch generation.
3. Run `TestHybridGoldenSetEval` with
   `OMNICRAFT_RAG_EMBEDDING_MODE=real`. Do not mix stand-in and real vectors in
   one generation; rebuild the generation when changing models.
4. Compare real MRR/Recall/nDCG to the same-run keyword baseline. If the real
   embedding gate fails, record the failure and cause; do not relabel it as a
   pass.
5. Capture provider/model/version, vector dimension, dataset checksum, commit,
   timestamp, latency, and estimated token cost. Never capture API keys.

The later Jaeger and ClamAV batches have separate environment prerequisites
and should not block this first provider slice.

## Security boundary

Never send an API key in chat, issue comments, progress logs, screenshots, or
committed YAML. If a key has ever been committed or exposed, revoke it before
using a replacement. Apply provider-side budget/rate limits and use a key that
can be revoked independently of any production credential.
