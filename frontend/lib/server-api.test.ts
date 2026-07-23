import assert from "node:assert/strict";
import test from "node:test";

import { getBrowserApiBase, getServerApiBase } from "./server-api";

const originalInternal = process.env.INTERNAL_API_URL;
const originalPublic = process.env.NEXT_PUBLIC_API_URL;

function restoreEnvironment() {
  if (originalInternal === undefined) delete process.env.INTERNAL_API_URL;
  else process.env.INTERNAL_API_URL = originalInternal;
  if (originalPublic === undefined) delete process.env.NEXT_PUBLIC_API_URL;
  else process.env.NEXT_PUBLIC_API_URL = originalPublic;
}

test("server rendering prefers the Docker-internal API URL", () => {
  process.env.INTERNAL_API_URL = "http://backend:8080/";
  process.env.NEXT_PUBLIC_API_URL = "http://localhost:8080";
  assert.equal(getServerApiBase(), "http://backend:8080/api/v1");
  restoreEnvironment();
});

test("server rendering keeps same-origin mode when public API URL is empty", () => {
  delete process.env.INTERNAL_API_URL;
  process.env.NEXT_PUBLIC_API_URL = "";
  assert.equal(getServerApiBase(), "/api/v1");
  restoreEnvironment();
});

test("local development falls back to the host API", () => {
  delete process.env.INTERNAL_API_URL;
  delete process.env.NEXT_PUBLIC_API_URL;
  assert.equal(getServerApiBase(), "http://localhost:8080/api/v1");
  restoreEnvironment();
});

    
test("browser API base never exposes the Docker-internal API host", () => {
  delete process.env.INTERNAL_API_URL;
  process.env.NEXT_PUBLIC_API_URL = "";
  assert.equal(getBrowserApiBase(), "http://localhost:8080/api/v1");

  process.env.NEXT_PUBLIC_API_URL = "https://api.example.com/";
  assert.equal(getBrowserApiBase(), "https://api.example.com/api/v1");
  restoreEnvironment();
});
