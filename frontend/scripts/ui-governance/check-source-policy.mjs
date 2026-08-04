// Enforced gate: raw backend/exception messages must never be rendered to users.
// The patterns mirror the U-05 audit in
// frontend/tests/user-facing-error-surfaces.test.tsx but are enforced across
// ALL production sources, so newly added files cannot leak messages that the
// audited-surface list would miss.
// Measured reports (not yet gates): hard-coded JSX text, aria-label literals,
// color literals, and compact explicit sizes — these stay informational until
// false positives and intentional exceptions are encoded into allowlists.

// Approximate JSX text-node heuristic for the measured report. Known noisy
// classes: `{a > b && <X/>}` comparisons and `=> <div>` arrow bodies, which are
// false positives until an explicit exception list is encoded.
const HARD_CODED_JSX_TEXT = />[^{<]*[A-Za-z\u4e00-\u9fff][^{<]*</g;
const ARIA_LABEL_LITERAL = /aria-label\s*=\s*["'][A-Za-z\u4e00-\u9fff]/g;
const COLOR_LITERAL = /#[0-9a-fA-F]{3,8}\b|rgba?\(|hsla?\(/g;
const COMPACT_EXPLICIT_SIZE = /\b(?:size|h|w|min-h|min-w)-(\d+)\b/g;

// Tailwind numeric utilities are multiples of the 0.25rem spacing base (4px);
// h-10 is 40px and h-11 is 44px, so only values below 11 stay under the
// 44px coarse-pointer target.
const TOUCH_TARGET_MAX_UNIT = 11;

// Mirrors the raw-rendering patterns audited by U-05 in
// frontend/tests/user-facing-error-surfaces.test.tsx. Order matters for output.
export const RAW_RENDER_PATTERNS = [
  { name: "error-message-union", pattern: /error\.message\s*\|\|/ },
  {
    name: "apirequesterror-instanceof",
    pattern: /instanceof\s+ApiRequestError\s*\?\s*(?:e|err)\.message/,
  },
  {
    name: "code-message-interpolation",
    pattern: /\$\{(?:e|err)\.code\}:\s*\$\{(?:e|err)\.message\}/,
  },
  {
    name: "toast-error-message",
    pattern: /toast\(\s*["']error["']\s*,\s*err\.message\s*\)/,
  },
];

export function checkRawErrorPolicy(sourceByPath) {
  const violations = [];
  for (const [filePath, source] of Object.entries(sourceByPath)) {
    for (const { name, pattern } of RAW_RENDER_PATTERNS) {
      if (pattern.test(source)) violations.push({ filePath, pattern: name });
    }
  }
  return violations;
}

export function measureSourcePolicy(sourceByPath) {
  const report = {
    hardCodedText: { count: 0, files: 0 },
    ariaLabels: { count: 0, files: 0 },
    colors: { count: 0, files: 0 },
    touchTargets: { count: 0, files: 0 },
  };

  for (const source of Object.values(sourceByPath)) {
    const textCount = countMatches(source, HARD_CODED_JSX_TEXT);
    const ariaCount = countMatches(source, ARIA_LABEL_LITERAL);
    const colorCount = countMatches(source, COLOR_LITERAL);
    const touchCount = [...source.matchAll(COMPACT_EXPLICIT_SIZE)].filter(
      (match) => Number(match[1]) < TOUCH_TARGET_MAX_UNIT,
    ).length;

    report.hardCodedText.count += textCount;
    report.ariaLabels.count += ariaCount;
    report.colors.count += colorCount;
    report.touchTargets.count += touchCount;
    if (textCount > 0) report.hardCodedText.files += 1;
    if (ariaCount > 0) report.ariaLabels.files += 1;
    if (colorCount > 0) report.colors.files += 1;
    if (touchCount > 0) report.touchTargets.files += 1;
  }

  return report;
}

function countMatches(source, pattern) {
  pattern.lastIndex = 0;
  return (source.match(pattern) ?? []).length;
}
