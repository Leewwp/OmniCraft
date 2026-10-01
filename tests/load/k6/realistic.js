import http from 'k6/http';
import exec from 'k6/execution';
import { Trend, Counter } from 'k6/metrics';

// #730 Round A — "realistic query distribution" tier for the anonymous
// search endpoint (GET /api/v1/contents/search). Query mix comes from the
// frozen Golden Set v2 (evaluation-driven SYNTHETIC mix — see
// realistic-queries.json; the report must not call it observed traffic).
// This endpoint never touches rerank (verified link fact), so the tier runs
// a single configuration and says so in the report.
//
// Measurement discipline:
// - Direct-to-backend target (no nginx): entry limit_req would cap the run;
//   the load environment disables rate limiting via ops/load/config-override.yaml.
// - Warmup vs steady separation: the first WARMUP_S seconds are tagged
//   phase=warmup; headline numbers come from phase=steady only (filter the
//   summary with the tag, e.g. `k6 run --summary-export` then aggregate, or
//   use the per-phase Trend metrics below).
// - Errors / empty results are classified as counters (error vs empty vs
//   hit) — "empty" is a valid no_answer-layer outcome, not a failure.
//
// Env:
//   BASE       (default http://127.0.0.1:8080) backend origin
//   VUS        (default 8)                     concurrent virtual users
//   DURATION   (default 5m)                    total run length
//   WARMUP_S   (default 60)                    seconds tagged warmup

const QUERIES = JSON.parse(open(import.meta.resolve('./realistic-queries.json')));

const BASE = __ENV.BASE || 'http://127.0.0.1:8080';
const VUS = parseInt(__ENV.VUS || '8', 10);
const DURATION = __ENV.DURATION || '5m';
const WARMUP_S = parseFloat(__ENV.WARMUP_S || '60');

const steadyLatency = new Trend('search_steady_duration', true);
const steadyHits = new Counter('search_steady_hits');
const steadyEmpty = new Counter('search_steady_empty_results');
const steadyErrors = new Counter('search_steady_errors');
const steadyContractRejects = new Counter('search_steady_contract_rejects');
const warmupRequests = new Counter('search_warmup_requests');

export const options = {
  scenarios: {
    realistic_mix: {
      executor: 'constant-vus',
      vus: VUS,
      duration: DURATION,
    },
  },
  thresholds: {
    // Headline gate on the steady phase only (warmup excluded via metric
    // split). Contract rejections (HTTP 400 QUERY_TOO_LONG — the endpoint's
    // max_query_chars cap hit by some long natural-language golden queries)
    // are an expected classification bucket, NOT errors. "This-tier
    // baseline" — no saturation-point claim.
    'search_steady_errors': ['count==0'],
  },
};

const testStart = Date.now();

export default function () {
  const idx = exec.scenario.iterationInTest % QUERIES.queries.length;
  const item = QUERIES.queries[idx];
  const steady = (Date.now() - testStart) / 1000 > WARMUP_S;

  const url = `${BASE}/api/v1/contents/search?q=${encodeURIComponent(item.q)}&page=1&page_size=10`;
  const res = http.get(url, { tags: { phase: steady ? 'steady' : 'warmup', form: item.form, layer: item.layer } });

  if (!steady) {
    warmupRequests.add(1);
  }
  if (res.status === 200) {
    if (!steady) return;
    steadyLatency.add(res.timings.duration, { form: item.form, layer: item.layer });
    const items = res.json('items');
    if (Array.isArray(items) && items.length > 0) {
      steadyHits.add(1, { form: item.form });
    } else {
      steadyEmpty.add(1, { form: item.form, layer: item.layer });
    }
    return;
  }
  // Over-length golden queries hit the endpoint contract (max_query_chars,
  // factory 120) — classify separately; only unexpected failures are errors.
  const contractReject = res.status === 400 && String(res.json('code') || '') === 'QUERY_TOO_LONG';
  if (!steady) return;
  if (contractReject) {
    steadyContractRejects.add(1, { form: item.form });
  } else {
    steadyErrors.add(1, { form: item.form, layer: item.layer, status: String(res.status) });
  }
}

export function handleSummary(data) {
  // Extend the standard summary with the mix provenance so archived runs are
  // self-describing (source hash/seed live in realistic-queries.json).
  data.mix = {
    source: QUERIES.source,
    seed: QUERIES.seed,
    layers: QUERIES.layers,
    forms: QUERIES.forms,
    measurement: 'k6 direct-to-backend, synthetic golden-set mix, steady phase only for headline numbers',
  };
  return {
    stdout: JSON.stringify({
      mix: data.mix,
      steady_duration: data.metrics.search_steady_duration
        ? data.metrics.search_steady_duration.values
        : null,
      steady_hits: data.metrics.search_steady_hits ? data.metrics.search_steady_hits.values.count : null,
      steady_empty: data.metrics.search_steady_empty_results ? data.metrics.search_steady_empty_results.values.count : null,
      steady_contract_rejects: data.metrics.search_steady_contract_rejects ? data.metrics.search_steady_contract_rejects.values.count : null,
      steady_errors: data.metrics.search_steady_errors ? data.metrics.search_steady_errors.values.count : null,
    }, null, 2),
  };
}
