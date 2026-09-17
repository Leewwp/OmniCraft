"""Generation-layer metrics, Ragas definitions on a DeepSeek judge (SP-22 E1).

Ragas-caliber metric implementations against the OpenAI-compatible DeepSeek
endpoint (direct client, no ragas dependency): the system Python here is 3.9
and ragas' dependency tree does not resolve on it (verified 2026-09-17;
resolver spins >15min with zero packages installed). Metric recipes follow
the Ragas docs so numbers stay comparable when the library becomes viable:

- faithfulness      two-stage: extract atomic claims from the answer, then
                    verdict each claim against the contexts; score =
                    supported / total.
- answer_relevancy  judge reverse-generates 3 questions from the answer;
                    score = mean cosine(original question, generated) via a
                    Chinese-friendly embedding endpoint.
- context_precision MAP-style over the ranked context list: the judge emits
                    one relevance verdict per chunk in a single call; score =
                    weighted precision @ k over relevant chunks.
- noise_sensitivity re-verdicts the extracted claims against contexts + two
                    sampled irrelevant chunks; score = fraction of claims
                    whose verdict flips to unsupported under noise (lower is
                    better).

All judge calls: temperature 0, JSON-forced replies, bounded retries with
backoff, and a courtesy sleep between requests. Keys arrive only via env
(EVALS_JUDGE_* / EVALS_EMBED_*); nothing is persisted into artifacts.
"""

from __future__ import annotations

import json
import os
import random
import re
import time

from openai import OpenAI

from evals_lib import EvalCase

CLAIMS_PROMPT = """You are evaluating a RAG assistant answer for an omnimedia creative-sharing platform.

Question:
{question}

Answer:
{answer}

Decompose the answer into atomic claims (each independently verifiable single statement). Ignore purely stylistic sentences, greetings, and disclaimers that state no fact. If the answer contains no factual claims (e.g. it only apologizes or asks for clarification), return an empty list.

Respond ONLY with JSON: {{"claims": ["...", "..."]}}"""

VERDICTS_PROMPT = """You are verifying claims against retrieved context passages from a creative-sharing platform.

Context passages:
{contexts}

Claims:
{claims}

For EACH claim, decide whether it is directly supported by the passages above (a claim is supported only if the passages contain the information needed to confirm it; do not use your own world knowledge). Paraphrase counts as support; a claim about passage content that exaggerates or adds unstated detail is NOT supported.

Respond ONLY with JSON: {{"verdicts": [true, false, ...]}} with one verdict per claim, in order."""

REVERSE_QUESTION_PROMPT = """Generate {n} questions that the given answer could be responding to. The questions must be in the SAME LANGUAGE as the answer. Each question should be specific and capture a distinct intent the answer addresses.

Answer:
{answer}

Respond ONLY with JSON: {{"questions": ["...", "..."]}}"""

CONTEXT_PRECISION_PROMPT = """You are judging retrieved passages for a search on a creative-sharing platform.

Question:
{question}

Retrieved passages (in rank order):
{passages}

For EACH passage, decide whether it is useful for answering the question (contains relevant information the answer could build on).

Respond ONLY with JSON: {{"verdicts": [true, false, ...]}} with one verdict per passage, in order."""


class JudgeBackend:
    """DeepSeek judge + Chinese-friendly embeddings via the OpenAI SDK."""

    def __init__(self, sleep_ms: int = 300, max_retries: int = 4):
        self.judge = OpenAI(
            api_key=os.environ.get("EVALS_JUDGE_API_KEY", ""),
            base_url=os.environ.get("EVALS_JUDGE_API_BASE", "https://api.deepseek.com"),
        )
        self.judge_model = os.environ.get("EVALS_JUDGE_MODEL", "deepseek-chat")
        self.embed = OpenAI(
            api_key=os.environ.get("EVALS_EMBED_API_KEY", ""),
            # OpenAI SDK appends /embeddings, so dashscope compatible-mode
            # bases need the /v1 suffix here (unlike the Go llm client).
            base_url=os.environ.get(
                "EVALS_EMBED_API_BASE",
                "https://dashscope.aliyuncs.com/compatible-mode/v1",
            ),
        )
        self.embed_model = os.environ.get("EVALS_EMBED_MODEL", "text-embedding-v4")
        self.sleep_ms = sleep_ms
        self.max_retries = max_retries
        self.calls = 0
        self.judge_tokens = 0

    def _chat_json(self, prompt: str, max_tokens: int = 2048):
        last_err = None
        for attempt in range(self.max_retries):
            try:
                self.calls += 1
                resp = self.judge.chat.completions.create(
                    model=self.judge_model,
                    messages=[{"role": "user", "content": prompt}],
                    temperature=0,
                    max_tokens=max_tokens,
                    response_format={"type": "json_object"},
                )
                if resp.usage:
                    self.judge_tokens += (resp.usage.prompt_tokens or 0) + (resp.usage.completion_tokens or 0)
                content = resp.choices[0].message.content or ""
                time.sleep(self.sleep_ms / 1000.0)
                return _parse_json(content)
            except Exception as err:  # noqa: BLE001 - retry any transport/parse error
                last_err = err
                time.sleep(min(2 ** attempt, 10))
        raise RuntimeError(f"judge call failed after {self.max_retries} retries: {last_err}")

    def embed_texts(self, texts: list[str]) -> list[list[float]]:
        out: list[list[float]] = []
        for i in range(0, len(texts), 16):
            batch = texts[i:i + 16]
            resp = self.embed.embeddings.create(model=self.embed_model, input=batch)
            out.extend(d.embedding for d in resp.data)
            time.sleep(self.sleep_ms / 1000.0)
        return out


def _parse_json(content: str):
    text = content.strip()
    text = re.sub(r"^```(?:json)?\s*|\s*```$", "", text)
    try:
        return json.loads(text)
    except json.JSONDecodeError:
        match = re.search(r"\{.*\}", text, re.S)
        if match:
            return json.loads(match.group(0))
        raise


def _cosine(a: list[float], b: list[float]) -> float:
    num = sum(x * y for x, y in zip(a, b))
    na = sum(x * x for x in a) ** 0.5
    nb = sum(y * y for y in b) ** 0.5
    if na == 0 or nb == 0:
        return 0.0
    return num / (na * nb)


def _numbered(contexts: list[str]) -> str:
    return "\n\n".join(f"[{i + 1}] {c}" for i, c in enumerate(contexts))


def extract_claims(be: JudgeBackend, case: EvalCase) -> list[str]:
    data = be._chat_json(CLAIMS_PROMPT.format(question=case.query, answer=case.answer[:4000]))
    return [str(c) for c in (data.get("claims") or [])]


def verdict_claims(be: JudgeBackend, case: EvalCase, claims: list[str], contexts: list[str]) -> list[bool]:
    if not claims:
        return []
    data = be._chat_json(VERDICTS_PROMPT.format(
        contexts=_numbered(contexts)[:12000],
        claims="\n".join(f"{i + 1}. {c}" for i, c in enumerate(claims)),
    ))
    verdicts = data.get("verdicts") or []
    out = []
    for i in range(len(claims)):
        v = verdicts[i] if i < len(verdicts) else False
        out.append(bool(v))
    return out


def faithfulness(be: JudgeBackend, case: EvalCase, contexts: list[str]) -> dict:
    """Two-stage Ragas faithfulness; returns claims + verdicts for reuse."""
    claims = extract_claims(be, case)
    if not claims:
        return {"claims": [], "verdicts": [], "score": None}
    verdicts = verdict_claims(be, case, claims, contexts)
    supported = sum(1 for v in verdicts if v)
    return {
        "claims": claims,
        "verdicts": verdicts,
        "score": supported / len(claims),
    }


def answer_relevancy(be: JudgeBackend, case: EvalCase) -> dict:
    data = be._chat_json(REVERSE_QUESTION_PROMPT.format(n=3, answer=case.answer[:4000]))
    questions = [str(q) for q in (data.get("questions") or [])][:3]
    if not questions:
        return {"score": None}
    vecs = be.embed_texts([case.query] + questions)
    origin = vecs[0]
    sims = [_cosine(origin, v) for v in vecs[1:]]
    return {"questions": questions, "score": sum(sims) / len(sims)}


def context_precision(be: JudgeBackend, case: EvalCase, contexts: list[str]) -> dict:
    if not contexts:
        return {"score": None}
    data = be._chat_json(CONTEXT_PRECISION_PROMPT.format(
        question=case.query,
        passages=_numbered([c[:600] for c in contexts])[:12000],
    ))
    verdicts = data.get("verdicts") or []
    hits = 0
    weighted = 0.0
    relevant = 0
    for i in range(len(contexts)):
        v = bool(verdicts[i]) if i < len(verdicts) else False
        if v:
            relevant += 1
    if relevant == 0:
        return {"relevant": 0, "score": 0.0}
    for i in range(len(contexts)):
        v = bool(verdicts[i]) if i < len(verdicts) else False
        if v:
            hits += 1
            weighted += hits / (i + 1)
    return {"relevant": relevant, "score": weighted / relevant}


def noise_sensitivity(be: JudgeBackend, case: EvalCase, claims: list[str],
                      verdicts: list[bool], contexts: list[str], noise_pool: list[str]) -> dict:
    """Fraction of previously-supported claims that flip under added noise."""
    supported_idx = [i for i, v in enumerate(verdicts) if v]
    if not supported_idx or not noise_pool:
        return {"flipped": 0, "supported": len(supported_idx), "score": None}
    noise = random.sample(noise_pool, k=min(2, len(noise_pool)))
    noisy = contexts + noise
    noisy_claims = [claims[i] for i in supported_idx]
    noisy_verdicts = verdict_claims(be, case, noisy_claims, noisy)
    flipped = sum(1 for v in noisy_verdicts if not v)
    return {"flipped": flipped, "supported": len(supported_idx), "score": flipped / len(supported_idx)}


def evaluate_case(be: JudgeBackend, case: EvalCase, noise_pool: list[str]) -> dict:
    """Run the four metrics for one case. Refusals skip generation metrics."""
    out = {"case_key": case.case_key, "status": case.status, "refused": case.refused}
    if case.refused or not case.answer or not case.contexts:
        out.update({"faithfulness": None, "answer_relevancy": None,
                    "context_precision": None, "noise_sensitivity": None})
        return out
    surfaces = [c.surface() for c in case.contexts]
    f = faithfulness(be, case, surfaces)
    ar = answer_relevancy(be, case)
    cp = context_precision(be, case, surfaces)
    ns = noise_sensitivity(be, case, f["claims"], f["verdicts"], surfaces, noise_pool)
    out.update({
        "faithfulness": f["score"],
        "faithfulness_claims": len(f["claims"]),
        "answer_relevancy": ar["score"],
        "context_precision": cp["score"],
        "noise_sensitivity": ns["score"],
        "cited_ids": sorted(case.cited_ids),
    })
    return out
