import { ApiRequestError } from "./api";

interface ErrorContext {
  component?: string;
  action?: string;
}

export function handleApiError(
  error: unknown,
  context?: ErrorContext,
  options?: { toast?: (type: "error" | "warning", msg: string) => void }
): void {
  if (error instanceof ApiRequestError) {
    if (error.status === 401) {
      // Token issues handled by api.ts auto-refresh
      console.warn(`[auth] 401 in ${context?.component}:${context?.action}`);
      return;
    }
    if (error.status === 403) {
      options?.toast?.("error", "Permission denied");
      return;
    }
    if (error.status >= 500) {
      options?.toast?.("error", "Server busy, please try again later");
      return;
    }
    console.error(`[api-error] ${context?.component}:${context?.action} — ${error.code}: ${error.message}`);
    return;
  }

  if (error instanceof TypeError && error.message === "Failed to fetch") {
    options?.toast?.("error", "Network connection failed, please check your network");
    return;
  }

  console.error(`[error] ${context?.component}:${context?.action} —`, error);
}

export function silentError(error: unknown, context?: ErrorContext): void {
  if (error instanceof ApiRequestError) {
    console.warn(`[silent-api-error] ${context?.component}:${context?.action} — ${error.code}`);
    return;
  }
  console.warn(`[silent-error] ${context?.component}:${context?.action} —`, error);
}
