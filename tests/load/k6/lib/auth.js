import http from 'k6/http';
import { check } from 'k6';

// OmniCraft auth helper: obtains a CSRF token (cookie + header) and logs in
// with the given identity, returning the bearer access token.
//
// The CSRF flow: GET /api/v1/auth/csrf sets the `csrf-token` cookie and
// returns the token; POST /auth/login must echo it via X-CSRF-Token. The
// login response itself does not carry a csrf_token field (it returns
// user/capabilities/tokens only), so the token from the initial GET is
// returned; the cookie jar keeps the matching cookie for the same VU.
// Returns { token } or null on failure.

export function login(baseUrl, email, password) {
  const csrfRes = http.get(`${baseUrl}/api/v1/auth/csrf`, {
    tags: { name: 'auth_csrf' },
  });
  const csrfToken = csrfRes.json('csrf_token');
  if (!csrfToken) {
    return null;
  }

  const res = http.post(
    `${baseUrl}/api/v1/auth/login`,
    JSON.stringify({ email, password }),
    {
      headers: {
        'Content-Type': 'application/json',
        'X-CSRF-Token': csrfToken,
      },
      tags: { name: 'auth_login' },
    }
  );
  const ok =
    res.status === 200 &&
    check(res, { 'login returns tokens': (r) => r.json('tokens.access_token') !== undefined });
  if (!ok) {
    return null;
  }
  return { token: res.json('tokens.access_token') };
}
