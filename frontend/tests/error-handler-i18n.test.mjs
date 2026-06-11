import { readFileSync } from "node:fs";
import test from "node:test";
import assert from "node:assert/strict";

const source = readFileSync(new URL("../lib/error-handler.ts", import.meta.url), "utf8");

test("api error handler uses a translation key for rate limit toasts", () => {
  assert.doesNotMatch(
    source,
    /Too many requests\. Please try again later\./,
    "429 handling must not hardcode user-facing copy",
  );
  assert.match(source, /common\.rateLimited/, "429 handling should use the shared rate limit i18n key");
});
