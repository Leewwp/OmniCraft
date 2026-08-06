import http from 'k6/http';
import { check } from 'k6';
import { login } from './auth.js';

// Shared scenario helpers for the OmniCraft load suite. Each tier script
// (smoke/load/stress) builds its options from the release profile and the
// thresholds file, then drives the endpoint mix below. The mix mirrors the
// release-profile `endpoints` weights: health, feed, search, detail
// (anonymous) and authenticated read/write. Paid Provider/OSS paths are
// never called.
//
// Auth design: the backend rate-limits credential logins per IP (default
// 5/min). All k6 VUs share the runner IP, so only a bounded pool of VUs
// (AUTH_VUS, default 5, matching one identity each) perform authenticated
// paths; every other VU only hits anonymous endpoints. This dilutes the
// realized auth_read/auth_write share below the profile weights on Load and
// Stress tiers (a documented trade-off forced by the per-IP login limit);
// the Smoke tier mixes them fully. Each auth VU logs in at most once and
// reuses the bearer token. The write path re-fetches the CSRF token per
// request because the CSRF cookie must be echoed alongside it; the per-VU
// cookie jar does persist across iterations, but rebuilding the pairing is
// defensive against future jar semantics changes.

const TESTDATA = JSON.parse(open(import.meta.resolve('../testdata.json')));
const THRESHOLDS = JSON.parse(open(import.meta.resolve('../thresholds.json')));

const AUTH_VUS = parseInt(__ENV.AUTH_VUS || '5', 10);
const ANON_NAMES = ['health', 'feed', 'search', 'detail'];

// Per-VU login cache: only populated for VUs in the auth pool.
const SESSIONS = {};

// Profiles may only be read during the init stage (k6 open() restriction),
// so the parsed profile is cached once per tier script at import time.
let PROFILE_CACHE = null;
function getProfile(path) {
  if (!PROFILE_CACHE) {
    PROFILE_CACHE = JSON.parse(open(path));
  }
  return PROFILE_CACHE;
}

function pickWeighted(weights, names) {
  let total = 0;
  for (const n of names) {
    total += weights[n];
  }
  if (total <= 0) {
    return names[0];
  }
  let r = Math.random() * total;
  for (const n of names) {
    r -= weights[n];
    if (r <= 0) {
      return n;
    }
  }
  return names[names.length - 1];
}

function randomFrom(arr) {
  return arr[Math.floor(Math.random() * arr.length)];
}

// Anonymous requests.
function health(baseUrl) {
  const res = http.get(`${baseUrl}/healthz`, { tags: { name: 'health' } });
  check(res, { 'health 200': (r) => r.status === 200 });
  return res;
}

function feed(baseUrl) {
  const res = http.get(`${baseUrl}/api/v1/contents?page=1&page_size=${TESTDATA.feed.page_size}`, {
    tags: { name: 'feed' },
  });
  check(res, { 'feed 200': (r) => r.status === 200 });
  return res;
}

function search(baseUrl) {
  const q = encodeURIComponent(randomFrom(TESTDATA.search.terms));
  const res = http.get(`${baseUrl}/api/v1/contents/search?q=${q}&page=1`, {
    tags: { name: 'search' },
  });
  check(res, { 'search 200': (r) => r.status === 200 });
  return res;
}

function detail(baseUrl) {
  const id = __ENV.CONTENT_ID || 1;
  const res = http.get(`${baseUrl}/api/v1/contents/${id}`, {
    tags: { name: 'detail' },
  });
  check(res, { 'detail 200': (r) => r.status === 200 });
  return res;
}

// Authenticated read: cached bearer token only (no CSRF needed for GET).
function authRead(baseUrl, token) {
  const res = http.get(`${baseUrl}/api/v1/auth/me`, {
    headers: { Authorization: `Bearer ${token}` },
    tags: { name: 'auth_me' },
  });
  check(res, { 'auth/me 200': (r) => r.status === 200 });
  return res;
}

// Authenticated write: cached bearer token plus a fresh CSRF token fetched
// per request (the CSRF cookie must be echoed alongside the header; the
// per-VU jar persists across iterations but rebuilding the pairing here is
// defensive against future jar semantics changes).
function authWrite(baseUrl, token) {
  const csrfRes = http.get(`${baseUrl}/api/v1/auth/csrf`, {
    tags: { name: 'auth_csrf' },
  });
  const csrfToken = csrfRes.json('csrf_token');

  const title = `${randomFrom(TESTDATA.content.titles)} ${__VU}-${__ITER}`;
  const body = JSON.stringify({
    title,
    description: `load test sample content created by VU ${__VU} iteration ${__ITER}`,
    zone: 'original',
    category: randomFrom(TESTDATA.content.categories),
    content_type: randomFrom(TESTDATA.content.content_types),
    tags: TESTDATA.content.tags,
  });
  const res = http.post(`${baseUrl}/api/v1/contents`, body, {
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
      'X-CSRF-Token': csrfToken,
    },
    tags: { name: 'content_create' },
  });
  check(res, { 'content create 200/201': (r) => r.status === 200 || r.status === 201 });
  return res;
}

function ensureSession(baseUrl) {
  if (!SESSIONS[__VU]) {
    const identity = `${TESTDATA.identities.prefix}${String(__VU).padStart(3, '0')}${TESTDATA.identities.suffix}`;
    const session = login(baseUrl, identity, TESTDATA.identities.password);
    if (!session) {
      return null;
    }
    SESSIONS[__VU] = session;
  }
  return SESSIONS[__VU];
}

// Build k6 options for the given tier from profile + thresholds.
export function buildOptions(profilePath, tier) {
  const profile = getProfile(profilePath);
  const tierKey = tier.toLowerCase();
  const t = profile.tiers[tierKey];
  if (!t) {
    throw new Error(`tier ${tier} not present in release profile`);
  }
  const thresholds = THRESHOLDS[tier] || THRESHOLDS.Smoke;

  const base = { thresholds };

  if (tier === 'Smoke') {
    base.vus = t.vus || 2;
    base.iterations = t.iterations || 10;
    base.duration = t.duration || '30s';
    return base;
  }

  const stages = [];
  if (t.stages) {
    for (const s of t.stages) {
      stages.push({ target: s.target, duration: s.duration });
    }
  } else {
    stages.push({ target: t.vus, duration: t.ramp || '1m' });
    stages.push({ target: t.vus, duration: t.duration });
    stages.push({ target: 0, duration: '30s' });
  }
  base.scenarios = {
    ramp: {
      executor: 'ramping-vus',
      stages,
      gracefulStop: '30s',
    },
  };
  return base;
}

// The default VU action: pick an endpoint per the profile weights. Auth
// endpoints are only served by the bounded auth pool (VUs 1..AUTH_VUS).
export function defaultAction(profilePath, tier) {
  const profile = getProfile(profilePath);
  const baseUrl = __ENV.TARGET;
  const weights = profile.endpoints;

  let endpoint;
  if (__VU <= AUTH_VUS) {
    endpoint = pickWeighted(weights, Object.keys(weights));
  } else {
    endpoint = pickWeighted(weights, ANON_NAMES);
  }

  switch (endpoint) {
    case 'health':
      health(baseUrl);
      break;
    case 'feed':
      feed(baseUrl);
      break;
    case 'search':
      search(baseUrl);
      break;
    case 'detail':
      detail(baseUrl);
      break;
    case 'auth_read': {
      const session = ensureSession(baseUrl);
      if (session) {
        authRead(baseUrl, session.token);
      }
      break;
    }
    case 'auth_write': {
      const session = ensureSession(baseUrl);
      if (session) {
        authWrite(baseUrl, session.token);
      }
      break;
    }
    default:
      health(baseUrl);
  }
}
