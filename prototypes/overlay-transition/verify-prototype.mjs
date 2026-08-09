// OmniCraft #67 — prototype verification driver (throwaway)
// Runs the overlay-transition prototype through all four acceptance criteria
// and saves screenshots into <repo-root>/screenshots/.
//
// Resolution: use this checkout's frontend/node_modules, then normal package
// resolution as a fallback.
import { createRequire } from 'node:module';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { existsSync, readFileSync } from 'node:fs';

const require = createRequire(import.meta.url);
const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), '../..');
let pw = null;
const candidates = [resolve(repoRoot, 'frontend/node_modules/playwright-core'), 'playwright-core'];
const gitFile = resolve(repoRoot, '.git');
if (existsSync(gitFile)) {
  const gitDirLine = readFileSync(gitFile, 'utf8').trim();
  const gitDir = gitDirLine.startsWith('gitdir: ') ? gitDirLine.slice('gitdir: '.length) : '';
  const worktreesSegment = `${process.platform === 'win32' ? '\\' : '/'}worktrees${process.platform === 'win32' ? '\\' : '/'}`;
  const splitAt = gitDir.lastIndexOf(worktreesSegment);
  if (splitAt >= 0) {
    const primaryRoot = resolve(gitDir.slice(0, splitAt), '..');
    candidates.push(resolve(primaryRoot, 'frontend/node_modules/playwright-core'));
  }
}
for (const candidate of candidates) {
  try {
    pw = require(candidate);
    break;
  } catch {
    /* try next */
  }
}
if (!pw) {
  console.error('playwright-core not found — install it or check the fallback path');
  process.exit(2);
}

const { chromium } = pw;
const SHOT_DIR = resolve(repoRoot, 'screenshots');
const PAGE_URL = 'file://' + resolve(repoRoot, 'prototypes/overlay-transition/index.html');

const results = [];
function check(name, ok, detail = '') {
  results.push({ name, ok, detail });
  console.log(`${ok ? 'PASS' : 'FAIL'}  ${name}${detail ? '  — ' + detail : ''}`);
}

function note(name, detail = '') {
  console.log(`INFO  ${name}${detail ? '  — ' + detail : ''}`);
}

async function logLines(page) {
  return page.$$eval('#log li', (lis) => lis.map((li) => li.textContent.trim()));
}
async function waitSettled(page, open) {
  await page.waitForFunction(
    (wantOpen) => {
      const ov = document.getElementById('overlay');
      const body = document.getElementById('overlay-body');
      return (
        ov.classList.contains('open') === wantOpen &&
        (wantOpen ? getComputedStyle(body).opacity === '1' : !ov.classList.contains('open'))
      );
    },
    open,
    { timeout: 8000 }
  );
}
async function midShot(page, name, delayMs) {
  await page.waitForTimeout(delayMs);
  await page.screenshot({ path: resolve(SHOT_DIR, name), fullPage: false });
}
async function waitForLog(page, substr, timeout = 8000) {
  await page.waitForFunction(
    (needle) =>
      Array.from(document.querySelectorAll('#log li')).some((li) => li.textContent.includes(needle)),
    substr,
    { timeout }
  );
}

const browser = await chromium.launch();
const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });

try {
  await page.goto(PAGE_URL);
  await page.waitForSelector('#grid .card');
  const vtNative = await page.evaluate(() => typeof document.startViewTransition === 'function');
  note('VT capability detected', `startViewTransition=${vtNative}`);
  await page.screenshot({ path: resolve(SHOT_DIR, 'overlay-transition-feed-baseline.png') });

  /* ---------- AC1/AC2: FLIP core with VT forced off ---------- */
  await page.check('#chk-no-vt');
  await page.click('#btn-open-card');
  await page.waitForTimeout(140); // 300ms 转场中段
  const midTransform = await page.$eval('#overlay-cover', (el) => getComputedStyle(el).transform);
  check('FLIP open mid-flight has active transform', midTransform !== 'none', midTransform);
  await page.screenshot({ path: resolve(SHOT_DIR, 'overlay-transition-flip-open-mid.png') });
  await waitSettled(page, true);
  const settledTransform = await page.$eval('#overlay-cover', (el) => getComputedStyle(el).transform);
  check('FLIP open settled transform is none', settledTransform === 'none', settledTransform);
  await page.screenshot({ path: resolve(SHOT_DIR, 'overlay-transition-flip-open-settled.png') });
  let logs = await logLines(page);
  check('FLIP open path logged', logs.some((l) => l.includes('打开完成（FLIP）')), logs[0]);

  // close: reversible regression
  await page.keyboard.press('Escape');
  await page.waitForTimeout(120); // 240ms 回归动画中段
  await page.screenshot({ path: resolve(SHOT_DIR, 'overlay-transition-close-flip-mid.png') });
  await waitForLog(page, '关闭完成（FLIP 反向回归）');
  await waitSettled(page, false);
  const focusBack = await page.evaluate(() => document.activeElement && document.activeElement.classList.contains('card'));
  logs = await logLines(page);
  check('FLIP close reversible + path logged', logs.some((l) => l.includes('关闭完成（FLIP 反向回归）')), logs[0]);
  check('focus returns to trigger card', focusBack);

  /* ---------- AC2: View Transition path (progressive enhancement) ---------- */
  await page.uncheck('#chk-no-vt');
  await page.click('#btn-open-card');
  await midShot(page, 'overlay-transition-vt-open-mid.png', 140);
  await waitSettled(page, true);
  await page.screenshot({ path: resolve(SHOT_DIR, 'overlay-transition-vt-open-settled.png') });
  logs = await logLines(page);
  const vtPathOk = vtNative
    ? logs.some((l) => l.includes('打开完成（VT 共享元素）'))
    : logs.some((l) => l.includes('打开完成（FLIP）'));
  check('VT path open (or FLIP fallback when VT unsupported)', vtPathOk, logs[0]);
  await page.keyboard.press('Escape');
  const vtCloseNeedle = vtNative ? '关闭完成（VT 反向回归）' : '关闭完成（FLIP 反向回归）';
  await waitForLog(page, vtCloseNeedle);
  await waitSettled(page, false);
  logs = await logLines(page);
  const vtCloseOk = logs.some((l) => l.includes(vtCloseNeedle));
  check('VT path close reversible', vtCloseOk, logs[0]);

  // A supported browser may still reject t.finished (interruption/abort).
  // Emulate that asynchronous failure and prove the actual path is reported
  // as FLIP rather than a misleading VT success.
  if (vtNative) {
    await page.evaluate(() => {
      window.__prototypeNativeVT = document.startViewTransition;
      document.startViewTransition = (update) => {
        update();
        return { finished: Promise.reject(new DOMException('forced rejection', 'AbortError')) };
      };
    });
    await page.click('#btn-open-card');
    await waitForLog(page, '打开完成（VT 失败 → FLIP 兜底）');
    await waitSettled(page, true);
    await page.keyboard.press('Escape');
    await waitForLog(page, '关闭完成（VT 失败 → FLIP 兜底回归）');
    await waitSettled(page, false);
    logs = await logLines(page);
    check(
      'rejected VT reports and completes through FLIP fallback',
      logs.some((l) => l.includes('打开完成（VT 失败 → FLIP 兜底）')) &&
        logs.some((l) => l.includes('关闭完成（VT 失败 → FLIP 兜底回归）')),
      logs.slice(0, 4).join(' | '),
    );
    await page.evaluate(() => {
      document.startViewTransition = window.__prototypeNativeVT;
      delete window.__prototypeNativeVT;
    });
  } else {
    note('rejected VT fallback test skipped', 'native VT unavailable; forced-off FLIP path already covered');
  }

  /* ---------- AC3: fallback scenarios (centered scale+fade) ---------- */
  await page.check('#chk-no-vt');
  // programmatic open without source
  await page.click('#btn-open-programmatic');
  await midShot(page, 'overlay-transition-fallback-open-mid.png', 140);
  await waitSettled(page, true);
  await page.screenshot({ path: resolve(SHOT_DIR, 'overlay-transition-fallback-open-settled.png') });
  logs = await logLines(page);
  check('fallback open (no source) logged', logs.some((l) => l.includes('降级打开完成') && l.includes('无 source')), logs[0]);
  await page.keyboard.press('Escape');
  await waitSettled(page, false);

  // offscreen source
  await page.click('#btn-open-offscreen');
  await waitSettled(page, true);
  logs = await logLines(page);
  check('fallback open (offscreen source) logged', logs.some((l) => l.includes('降级打开完成') && l.includes('离屏')), logs[0]);
  await page.keyboard.press('Escape');
  await waitSettled(page, false);

  // detached source
  await page.click('#btn-open-detached');
  await waitSettled(page, true);
  logs = await logLines(page);
  check('fallback open (detached source) logged', logs.some((l) => l.includes('降级打开完成') && l.includes('已卸载')), logs[0]);
  await page.keyboard.press('Escape');
  await waitSettled(page, false);

  // unmeasurable source
  await page.click('#btn-open-unmeasurable');
  await waitSettled(page, true);
  logs = await logLines(page);
  check('fallback open (unmeasurable source) logged', logs.some((l) => l.includes('降级打开完成') && l.includes('无法测量')), logs[0]);
  await page.keyboard.press('Escape');
  await waitSettled(page, false);

  // source leaves viewport before close → close falls back (no flight offscreen)
  await page.click('#btn-close-offscreen');
  await waitSettled(page, false);
  await waitForLog(page, '关闭降级（居中缩淡）');
  logs = await logLines(page);
  check('close falls back when source left viewport', logs.some((l) => l.includes('关闭降级（居中缩淡）') && l.includes('离屏')), logs[0]);
  await page.screenshot({ path: resolve(SHOT_DIR, 'overlay-transition-close-fallback-settled.png') });

  /* ---------- AC4: reduced-motion (real emulation + force) ---------- */
  await page.emulateMedia({ reducedMotion: 'reduce' });
  await page.click('#btn-open-card');
  await midShot(page, 'overlay-transition-reduced-open-mid.png', 50); // 100ms 中段
  await waitSettled(page, true);
  const rmTransform = await page.$eval('#overlay-cover', (el) => getComputedStyle(el).transform);
  check('reduced-motion: no transform during open', rmTransform === 'none', rmTransform);
  await page.screenshot({ path: resolve(SHOT_DIR, 'overlay-transition-reduced-open-settled.png') });
  await waitForLog(page, '打开完成（reduced-motion'); // fade path logs after its transition settles
  logs = await logLines(page);
  check('reduced-motion open path logged (fade)', logs.some((l) => l.includes('打开完成（reduced-motion')), logs[0]);
  await page.keyboard.press('Escape');
  await waitSettled(page, false);
  await waitForLog(page, '关闭完成（reduced-motion');
  logs = await logLines(page);
  check('reduced-motion close path logged (fade)', logs.some((l) => l.includes('关闭完成（reduced-motion')), logs[0]);
  await page.emulateMedia({ reducedMotion: 'no-preference' });

  /* ---------- interruption: rapid open→close→open recovers ---------- */
  await page.uncheck('#chk-no-vt');
  await page.click('#btn-rapid');
  await page.waitForTimeout(1000);
  const openAfterRapid = await page.$eval('#overlay', (el) => el.classList.contains('open'));
  await waitSettled(page, true);
  await page.screenshot({ path: resolve(SHOT_DIR, 'overlay-transition-interrupt-recovered.png') });
  logs = await logLines(page);
  check('rapid open/close/open recovers to settled open', openAfterRapid && logs.some((l) => l.includes('中断')), logs.slice(0, 2).join(' | '));

  await browser.close();
} catch (err) {
  await browser.close();
  console.error('Script error:', err);
  process.exit(1);
}

const failed = results.filter((r) => !r.ok);
console.log(`\n== ${results.length - failed.length}/${results.length} checks passed ==`);
if (failed.length) {
  console.log('Failed:', failed.map((f) => f.name).join(', '));
  process.exit(1);
}
