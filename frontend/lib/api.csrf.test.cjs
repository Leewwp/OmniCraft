const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");
// #967：TS 7（原生重写版）无 TS JS API，ts.transpileModule 不可用——
// TS→CJS 转译改用 esbuild（与 tsx 同源已在依赖树内，显式声明于
// devDependencies）。
const esbuild = require("esbuild");

const apiSourcePath = path.join(__dirname, "api.ts");

function response(status, body) {
  return {
    ok: status >= 200 && status < 300,
    status,
    statusText: String(status),
    async json() {
      return body;
    },
  };
}

function loadApi(fetchImpl) {
  const source = fs.readFileSync(apiSourcePath, "utf8");
  const compiled = esbuild.transformSync(source, {
    loader: "ts",
    format: "cjs",
    target: "es2020",
  }).code;

  const module = { exports: {} };
  const context = vm.createContext({
    module,
    exports: module.exports,
    fetch: fetchImpl,
    // SP-17/T2：api.ts 引入 @/lib/auth-gate（401 刷新失败走事件桥）。
    // 本沙箱只测 CSRF bootstrap 语义，桥以 no-op 替身注入。
    require: (request) => {
      if (request === "@/lib/auth-gate") {
        return { emitAuthRequired: () => {} };
      }
      throw new Error(`unexpected sandbox require: ${request}`);
    },
    process: {
      env: {
        NEXT_PUBLIC_API_URL: "https://api.example.com",
      },
    },
  });

  vm.runInContext(compiled, context, { filename: apiSourcePath });
  return module.exports;
}

async function testBootstrapsBeforeFirstUnsafeRequest() {
  const calls = [];
  const responses = [
    response(200, { csrf_token: "fresh-token" }),
    response(202, { verification_required: true }),
  ];

  const { api } = loadApi(async (url, init = {}) => {
    calls.push({ url, init: { ...init, headers: { ...init.headers } } });
    return responses.shift();
  });

  await api.post("/api/v1/auth/register", { email: "new@example.com" });

  assert.equal(calls.length, 2);
  assert.equal(
    calls[0].url,
    "https://api.example.com/api/v1/auth/csrf"
  );
  assert.equal(calls[0].init.credentials, "include");
  assert.equal(
    calls[1].url,
    "https://api.example.com/api/v1/auth/register"
  );
  assert.equal(calls[1].init.credentials, "include");
  assert.equal(calls[1].init.headers["X-CSRF-Token"], "fresh-token");
}

async function testRefreshesAndRetriesOnceAfterInvalidCsrf() {
  const calls = [];
  const responses = [
    response(200, { csrf_token: "stale-token" }),
    response(403, {
      code: "CSRF_TOKEN_INVALID",
      message: "CSRF token missing or invalid",
    }),
    response(200, { csrf_token: "rotated-token" }),
    response(202, { verification_required: true }),
  ];

  const { api } = loadApi(async (url, init = {}) => {
    calls.push({ url, init: { ...init, headers: { ...init.headers } } });
    return responses.shift();
  });

  await api.post("/api/v1/auth/register", { email: "new@example.com" });

  assert.equal(calls.length, 4);
  assert.equal(calls[0].url, "https://api.example.com/api/v1/auth/csrf");
  assert.equal(calls[1].init.headers["X-CSRF-Token"], "stale-token");
  assert.equal(calls[2].url, "https://api.example.com/api/v1/auth/csrf");
  assert.equal(calls[3].init.headers["X-CSRF-Token"], "rotated-token");
}

async function run() {
  await testBootstrapsBeforeFirstUnsafeRequest();
  await testRefreshesAndRetriesOnceAfterInvalidCsrf();
}

run().catch((err) => {
  console.error(err);
  process.exit(1);
});
