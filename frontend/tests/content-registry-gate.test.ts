import assert from "node:assert/strict";
import test from "node:test";
import { readFileSync, readdirSync, statSync } from "node:fs";
import path from "node:path";

/**
 * #687 source-contract gate：允许清单之外，前端源码不得重新枚举完整
 * content_type 词表——分类学单一真源 = /config/public 注册表投影 +
 * lib/public-config.ts 兜底。检测形态：25 行滑动窗口内以字符串字面量
 * 出现 ≥8/9 个规范类型名（数组/对象键表/switch case 均覆盖）。
 * 新增豁免须在 PR 的 census 账表（迁/留裁决）中给出理由。
 */
const ALLOWLIST: Record<string, string> = {
  "lib/public-config.ts": "内置兜底注册表 = 全前端唯一被豁免的完整枚举点（仅旧后端/离线时生效，与后端 DefaultContentRegistry 同基线）",
  "app/(protected)/(headered)/history/page.tsx": "浏览历史筛选为独立 UI 展示序（article 居首），非任何配置序可表达；注册表投影只承载成员关系，展示序属 UI 决策",
  "app/(protected)/(headered)/judge/exam/page.tsx": "判官域独立类型空间（含 comment 伪类型、无 mod），非平台分类学轴",
  "app/(protected)/(headered)/studio/publish/fanwork/page.tsx": "发布类型格声明性产物（key→emoji 图标 + 文案键推导）；排序已由 type_order 配置驱动（ContentTypeGrid.applyTypeOrder）",
  "app/(protected)/(headered)/studio/publish/original/page.tsx": "同上：声明性图标/文案映射，非分类学枚举",
  "components/content/ContentDetail.tsx": "详情页按类型渲染模式/图标/标签的业务分支（markdown 正文、乐谱 viewer、agent 入口等），本批注册表不承载 UI 声明性产物",
  "components/content/ContentTypeFilter.tsx": "合集筛选为独立 UI 展示序（sheet_music/mod/prompt 位序与注册表声明序不同）；成员关系与注册表一致",
  "components/ip/hub/IPShareTab.tsx": "IP 分享 tab 业务子集（无 template）+ 自定义分组语义，非分类学全集",
  "lib/coverPlaceholder.ts": "逐类型封面占位 SVG 的声明性资产映射",
};

const CANONICAL_NAMES = ["image", "article", "video", "audio", "template", "sheet_music", "mod", "prompt", "other"];
const SCAN_ROOTS = ["app", "components", "lib", "contexts"];

function listSourceFiles(root: string, base: string): string[] {
  const out: string[] = [];
  for (const entry of readdirSync(root)) {
    if (entry === ".next" || entry === "node_modules") continue;
    const full = path.join(root, entry);
    if (statSync(full).isDirectory()) {
      out.push(...listSourceFiles(full, base));
      continue;
    }
    if (!/\.(ts|tsx)$/.test(entry) || /\.test\.(ts|tsx)$/.test(entry)) continue;
    out.push(path.relative(base, full));
  }
  return out;
}

function windowContainsEnumeration(source: string): boolean {
  const lines = source.split("\n");
  for (let start = 0; start < lines.length; start += 1) {
    const window = lines.slice(start, start + 25);
    const found = new Set<string>();
    for (const line of window) {
      for (const name of CANONICAL_NAMES) {
        const pattern = new RegExp(`["'\`]${name}["'\`]`);
        if (pattern.test(line)) found.add(name);
      }
    }
    if (found.size >= 8) return true;
  }
  return false;
}

test("#687 source-contract: no full content-type enumeration outside the allowlist", () => {
  const projectRoot = process.cwd();
  const files = SCAN_ROOTS.flatMap((root) => listSourceFiles(path.join(projectRoot, root), projectRoot));
  const violations: string[] = [];

  for (const rel of files) {
    const normalized = rel.replaceAll("\\", "/");
    if (normalized in ALLOWLIST) continue;
    const source = readFileSync(path.join(projectRoot, rel), "utf-8");
    if (windowContainsEnumeration(source)) violations.push(normalized);
  }

  assert.deepEqual(
    violations,
    [],
    `发现允许清单外的完整类型枚举——改消费注册表投影（lib/public-config.ts 读取层）或在 census 账表补充豁免理由: ${violations.join(", ")}`,
  );
});

test("#687 source-contract: every allowlist entry still exists and still trips the detector", () => {
  const projectRoot = process.cwd();
  for (const rel of Object.keys(ALLOWLIST)) {
    const full = path.join(projectRoot, rel);
    assert.ok(existsSyncSafe(full), `allowlist entry ${rel} no longer exists — prune it`);
    const source = readFileSync(full, "utf-8");
    assert.ok(
      windowContainsEnumeration(source),
      `allowlist entry ${rel} no longer contains a full enumeration — it was migrated; prune the allowlist entry`,
    );
  }
});

function existsSyncSafe(full: string): boolean {
  try {
    statSync(full);
    return true;
  } catch {
    return false;
  }
}
