const AUTH_KEYS = {
  USER: "user",
} as const;

let inMemoryAccessToken: string | null = null;

export function saveTokens(accessToken: string, _refreshToken?: string) {
  inMemoryAccessToken = accessToken;
}

export function getAccessToken(): string | null {
  return inMemoryAccessToken;
}

export function getRefreshToken(): string | null {
  return null;
}

export function clearTokens() {
  inMemoryAccessToken = null;
  if (typeof window !== "undefined") {
    localStorage.removeItem(AUTH_KEYS.USER);
  }
}

export function isTokenExpired(token: string): boolean {
  try {
    const payload = JSON.parse(atob(token.split(".")[1]));
    return payload.exp * 1000 < Date.now();
  } catch {
    return true;
  }
}
