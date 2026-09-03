import assert from "node:assert/strict";
import test from "node:test";

import { applyTypeOrder } from "@/components/studio/ContentTypeGrid";
import { uploadMaxMBForType, type PublicConfig } from "@/lib/public-config";

const baseConfig = {
  features: {} as PublicConfig["features"],
  captcha: {} as PublicConfig["captcha"],
  client: {} as PublicConfig["client"],
  legal: {} as PublicConfig["legal"],
  upload: {} as PublicConfig["upload"],
  collaboration: {} as PublicConfig["collaboration"],
  oss_domain: "",
};

test("applyTypeOrder reorders and filters by the configured order (T25)", () => {
  const types = [
    { value: "image", icon: "🖼️" },
    { value: "video", icon: "🎬" },
    { value: "article", icon: "📝" },
    { value: "prompt", icon: "🤖" },
  ] as const;

  assert.deepEqual(
    applyTypeOrder(types, ["article", "image", "video"]).map((t) => t.value),
    ["article", "image", "video"],
    "configured order wins and unlisted types are dropped",
  );

  assert.deepEqual(
    applyTypeOrder(types, null).map((t) => t.value),
    ["image", "video", "article", "prompt"],
    "missing config falls back to the built-in list",
  );

  assert.deepEqual(
    applyTypeOrder(types, []).map((t) => t.value),
    ["image", "video", "article", "prompt"],
    "empty config falls back to the built-in list",
  );
});

test("uploadMaxMBForType consumes runtime limits and falls back safely (T25)", () => {
  const config = {
    ...baseConfig,
    limits: { video_max_mb: 120, image_max_mb: 6, text_max_mb: 8, mod_max_mb: 500, sheet_music_max_mb: 50 },
  } as PublicConfig;

  assert.equal(uploadMaxMBForType(config, "video"), 120, "configured video cap wins");
  assert.equal(uploadMaxMBForType(config, "image"), 6, "configured image cap wins");
  assert.equal(uploadMaxMBForType(config, "text"), 8, "configured text cap wins");

  assert.equal(uploadMaxMBForType(null, "video"), 300, "no config → baseline fallback");
  assert.equal(uploadMaxMBForType({ ...baseConfig } as PublicConfig, "mod"), 500, "missing limits → baseline fallback");
  assert.equal(uploadMaxMBForType({ ...baseConfig, limits: { video_max_mb: 0 } } as PublicConfig, "video"), 300, "zero cap is treated as unset");
  assert.equal(uploadMaxMBForType({ ...baseConfig } as PublicConfig, "audio"), 20, "unmapped type keeps the generic 20MB default");
});
