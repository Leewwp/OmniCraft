import test from "node:test";
import assert from "node:assert/strict";
import { handleApiError } from "@/lib/error-handler";
import { ApiRequestError } from "@/lib/api";

test("429 response uses common.rateLimited translation key", () => {
  const toastCalls: Array<{ type: "error" | "warning"; msg: string }> = [];
  const translateCalls: string[] = [];

  const error = new ApiRequestError("RATE_LIMITED", "Too many requests", 429);

  handleApiError(error, { component: "Test", action: "rateLimit" }, {
    toast: (type, msg) => { toastCalls.push({ type, msg }); },
    translate: (key) => { translateCalls.push(key); return `[${key}]`; },
  });

  assert.deepEqual(translateCalls, ["common.rateLimited"], "429 should call translate with common.rateLimited key");
  assert.equal(toastCalls.length, 1, "429 should call toast once");
  assert.equal(toastCalls[0].type, "warning", "429 should use warning toast type");
  assert.equal(toastCalls[0].msg, "[common.rateLimited]", "429 toast should receive the translated message");
});

test("429 without translate function falls back to key string", () => {
  const toastCalls: Array<{ type: "error" | "warning"; msg: string }> = [];

  const error = new ApiRequestError("RATE_LIMITED", "Too many requests", 429);

  handleApiError(error, { component: "Test", action: "rateLimit" }, {
    toast: (type, msg) => { toastCalls.push({ type, msg }); },
  });

  assert.equal(toastCalls.length, 1, "429 should call toast once even without translate");
  assert.equal(toastCalls[0].msg, "common.rateLimited", "429 without translate should use key as fallback message");
});

test("401 response does not call toast (handled by auto-refresh)", () => {
  const toastCalls: Array<{ type: "error" | "warning"; msg: string }> = [];

  const error = new ApiRequestError("UNAUTHORIZED", "Unauthorized", 401);

  handleApiError(error, { component: "Test", action: "auth" }, {
    toast: (type, msg) => { toastCalls.push({ type, msg }); },
  });

  assert.equal(toastCalls.length, 0, "401 should not call toast");
});

test("500 response calls translate with common.serverError key", () => {
  const toastCalls: Array<{ type: "error" | "warning"; msg: string }> = [];
  const translateCalls: string[] = [];

  const error = new ApiRequestError("INTERNAL_ERROR", "Internal server error", 500);

  handleApiError(error, { component: "Test", action: "serverError" }, {
    toast: (type, msg) => { toastCalls.push({ type, msg }); },
    translate: (key) => { translateCalls.push(key); return `[${key}]`; },
  });

  assert.deepEqual(translateCalls, ["common.serverError"], "500 should call translate with common.serverError key");
  assert.equal(toastCalls.length, 1, "500 should call toast once");
  assert.equal(toastCalls[0].type, "error", "500 should use error toast type");
  assert.equal(toastCalls[0].msg, "[common.serverError]", "500 toast should receive the translated message");
});
