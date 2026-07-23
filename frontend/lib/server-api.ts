/**
 * API base URL for Next.js server-side rendering.
 *
 * The browser-facing URL and the Docker-internal URL are intentionally
 * separate: a browser cannot resolve the Compose service name `backend`,
 * while the Next.js container cannot reach the host through its own
 * `localhost`.
 */
export function getServerApiBase(): string {
  const internalURL = process.env.INTERNAL_API_URL;
  const publicURL = process.env.NEXT_PUBLIC_API_URL;
  const raw = internalURL ?? publicURL ?? "http://localhost:8080";
  if (!raw) {
    return process.env.NEXT_PHASE === "phase-production-build"
      ? "http://localhost:8080/api/v1"
      : "/api/v1";
  }
  return `${raw.replace(/\/$/, "")}/api/v1`;
}

/**
 * API base URL that is safe to pass to a client component.
 * SSR may use the Docker-only service name `backend`; browsers must use a
 * host-reachable public URL instead.
 */
export function getBrowserApiBase(): string {
  const configured = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
  const base = configured.replace(/\/+$/, "");
  return base.endsWith("/api/v1") ? base : `${base}/api/v1`;
}
