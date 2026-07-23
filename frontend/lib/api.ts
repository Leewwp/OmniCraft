// NEXT_PUBLIC_API_URL is intentionally host-reachable from the browser.
// Use || so an empty Compose value does not silently turn requests into
// same-origin /api/v1 calls when no frontend proxy is configured.
const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

export interface ApiError {
  code: string;
  message: string;
}

export class ApiRequestError extends Error {
  constructor(
    public code: string,
    message: string,
    public status: number
  ) {
    super(message);
    this.name = "ApiRequestError";
  }
}

let inMemoryCsrfToken: string | null = null;

async function fetchCSRFToken(forceRefresh = false): Promise<string> {
  if (inMemoryCsrfToken && !forceRefresh) return inMemoryCsrfToken;
  try {
    const res = await fetch(`${API_URL}/api/v1/auth/csrf`, {
      credentials: "include",
    });
    if (res.ok) {
      const data = (await res.json()) as { csrf_token: string };
      if (data.csrf_token) {
        inMemoryCsrfToken = data.csrf_token;
        return inMemoryCsrfToken;
      }
    }
  } catch {}
  return "";
}

function getCSRFTokenFromCookie(): string | null {
  if (typeof document === "undefined") return null;
  const match = document.cookie.match(
    /(?:^|;\s*)(?:__Host-csrf|csrf-token)=([^;]*)/
  );
  return match ? decodeURIComponent(match[1]) : null;
}

const STATE_CHANGING_METHODS = new Set(["POST", "PATCH", "PUT", "DELETE"]);

async function ensureCSRFHeader(
  headers: Record<string, string>,
  forceRefresh = false
) {
  const csrfToken = forceRefresh
    ? await fetchCSRFToken(true)
    : inMemoryCsrfToken || getCSRFTokenFromCookie() || (await fetchCSRFToken());

  if (!csrfToken) {
    throw new ApiRequestError(
      "CSRF_TOKEN_UNAVAILABLE",
      "security token unavailable, please refresh and try again",
      0
    );
  }

  headers["X-CSRF-Token"] = csrfToken;
}

let refreshPromise: Promise<boolean> | null = null;

let inMemoryAccessToken: string | null = null;

export function setAccessToken(token: string | null) {
  inMemoryAccessToken = token;
}

export function getAccessToken(): string | null {
  return inMemoryAccessToken;
}

async function tryRefreshToken(): Promise<boolean> {
  try {
    const csrfToken = await fetchCSRFToken();
    const res = await fetch(`${API_URL}/api/v1/auth/refresh`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        ...(csrfToken ? { "X-CSRF-Token": csrfToken } : {}),
      },
      credentials: "include",
    });
    if (!res.ok) return false;
    const data = (await res.json()) as {
      tokens: { access_token: string };
    };
    inMemoryAccessToken = data.tokens.access_token;
    return true;
  } catch {
    return false;
  }
}

async function doRefreshToken(): Promise<boolean> {
  if (!refreshPromise) {
    refreshPromise = tryRefreshToken().finally(() => {
      refreshPromise = null;
    });
  }
  return refreshPromise;
}

async function request<T>(
  path: string,
  options: RequestInit = {}
): Promise<T> {
  const token = inMemoryAccessToken;

  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...(options.headers as Record<string, string>),
  };

  if (token) {
    headers["Authorization"] = `Bearer ${token}`;
  }

  const method = (options.method ?? "GET").toUpperCase();
  if (STATE_CHANGING_METHODS.has(method)) {
    await ensureCSRFHeader(headers);
  }

  const res = await fetch(`${API_URL}${path}`, {
    ...options,
    headers,
    credentials: "include",
  });

  if (!res.ok) {
    let errBody: ApiError = { code: "UNKNOWN_ERROR", message: res.statusText };
    try {
      errBody = await res.json();
    } catch {}

    if (
      res.status === 403 &&
      errBody.code === "CSRF_TOKEN_INVALID" &&
      STATE_CHANGING_METHODS.has(method)
    ) {
      inMemoryCsrfToken = null;
      await ensureCSRFHeader(headers, true);
      const retryRes = await fetch(`${API_URL}${path}`, {
        ...options,
        headers,
        credentials: "include",
      });
      if (retryRes.ok) {
        if (retryRes.status === 204) return undefined as T;
        return retryRes.json();
      }

      try {
        errBody = await retryRes.json();
      } catch {
        errBody = { code: "UNKNOWN_ERROR", message: retryRes.statusText };
      }
      throw new ApiRequestError(errBody.code, errBody.message, retryRes.status);
    }

    if (res.status === 401 && errBody.code === "TOKEN_EXPIRED" && token) {
      const refreshed = await doRefreshToken();
      if (refreshed) {
        headers["Authorization"] = `Bearer ${inMemoryAccessToken}`;
        const retryRes = await fetch(`${API_URL}${path}`, {
          ...options,
          headers,
          credentials: "include",
        });
        if (retryRes.ok) {
          if (retryRes.status === 204) return undefined as T;
          return retryRes.json();
        }
      } else {
        inMemoryAccessToken = null;
        if (typeof window !== "undefined") {
          window.location.href = "/login";
        }
        throw new ApiRequestError("TOKEN_EXPIRED", "Session expired", 401);
      }
    }

    throw new ApiRequestError(errBody.code, errBody.message, res.status);
  }

  if (res.status === 204) return undefined as T;
  return res.json();
}

export const api = {
  get: <T>(path: string) => request<T>(path),
  post: <T>(path: string, body: unknown) =>
    request<T>(path, {
      method: "POST",
      body: JSON.stringify(body),
    }),
  patch: <T>(path: string, body: unknown) =>
    request<T>(path, {
      method: "PATCH",
      body: JSON.stringify(body),
    }),
  delete: <T>(path: string) => request<T>(path, { method: "DELETE" }),
  deleteWithBody: <T>(path: string, body: unknown) =>
    request<T>(path, {
      method: "DELETE",
      body: JSON.stringify(body),
    }),
  put: <T>(path: string, body: unknown) =>
    request<T>(path, {
      method: "PUT",
      body: JSON.stringify(body),
    }),
  getStatsSummary: () =>
    request<{
      summary: { users: number; ips: number; contents: number };
    }>("/api/v1/stats/summary"),
};
