import assert from "node:assert/strict";
import test from "node:test";

import { isProviderDegradation } from "@/lib/agent";

/* #684 B+ 裁决：provider 降级判定条件收口为全仓唯一真源 isProviderDegradation
 * （三消费方 = normalizeAgentEvent / agent-turn applyError / AgentWorkspace
 * handleStreamEvent）。本文件锚定谓词三态语义，以及归一化点对 unknown 的
 * 严格窄化（=== true，拒绝脏值——归一化后事件 degraded ∈ {undefined, true}，
 * 谓词与 truthy 判定行为一致）。 */

test("isProviderDegradation 三态：正常降级 / 非 provider_error 降级 / 非降级", () => {
  assert.equal(
    isProviderDegradation({ degraded: true, degraded_reason: "provider_error" }),
    true,
    "正常降级",
  );
  assert.equal(isProviderDegradation({ degraded: true }), false, "降级但缺 provider_error 归因");
  assert.equal(
    isProviderDegradation({ degraded: true, degraded_reason: "moderation" }),
    false,
    "非 provider_error 降级",
  );
  assert.equal(
    isProviderDegradation({ degraded: false, degraded_reason: "provider_error" }),
    false,
    "非降级",
  );
  assert.equal(isProviderDegradation({}), false, "两字段全缺 = 非降级");
});

test("isProviderDegradation 对归一化前的脏值严格窄化（=== true 语义）", () => {
  assert.equal(isProviderDegradation({ degraded: "yes", degraded_reason: "provider_error" }), false);
  assert.equal(isProviderDegradation({ degraded: 1, degraded_reason: "provider_error" }), false);
  assert.equal(isProviderDegradation({ degraded: "true", degraded_reason: "provider_error" }), false);
});
