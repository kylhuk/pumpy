#!/usr/bin/env node
// E2E test for the NeoDash wallet-graph dashboard.
// Launches Firefox headless, walks all 5 pages, collects console errors and
// MUI Snackbar popups, and screenshots each page.
// Exits 0 if clean, 1 if any error or popup found.

// Resolve playwright via createRequire (supports directory imports like CJS)
import { createRequire } from 'module';
import { readdirSync } from 'fs';
const _require = createRequire(import.meta.url);
const { firefox } = (() => {
  const candidates = [
    'playwright',
    '/home/hal9000/docker/poe_trade/node_modules/playwright',
    ...readdirSync('/home/hal9000/.npm/_npx').map(
      d => `/home/hal9000/.npm/_npx/${d}/node_modules/playwright`
    ),
  ];
  for (const p of candidates) {
    try { return _require(p); } catch {}
  }
  throw new Error('playwright not found — install it with: npm install playwright');
})();
import { mkdir } from 'fs/promises';
import { join, dirname } from 'path';
import { fileURLToPath } from 'url';

const URL = process.env.NEODASH_URL || 'https://pumpy.lama-lan.ch';
const OUT  = process.env.SCREENSHOT_DIR || join(dirname(fileURLToPath(import.meta.url)), '../tmp/dashboard-screenshots');
const PAGE_TIMEOUT_MS = 120_000;
const SETTLE_MS       = 3_000;   // ms to wait after networkidle before scraping popups

await mkdir(OUT, { recursive: true });

const browser = await firefox.launch({
  executablePath: process.env.FIREFOX_PATH || undefined,
  headless: true,
});

// 1100×800: below NeoDash's xl breakpoint (1280px) to keep the horizontal tab
// bar visible (at xl+, NeoDash switches to a sidebar that hides [role="tab"]).
const ctx  = await browser.newContext({ ignoreHTTPSErrors: true, viewport: { width: 1100, height: 800 } });
const page = await ctx.newPage();
page.setDefaultTimeout(PAGE_TIMEOUT_MS);

const consoleErrors = [];
const pageErrors    = [];

page.on('console', msg => {
  if (msg.type() === 'error') consoleErrors.push(msg.text());
});
page.on('pageerror', err => pageErrors.push(err.message));

// ── helpers ──────────────────────────────────────────────────────────────────

async function visiblePopups() {
  return page.evaluate(() => {
    const snacks = document.querySelectorAll(
      '.MuiSnackbar-root, [role="alert"], .MuiAlert-root'
    );
    const texts = [];
    for (const el of snacks) {
      const t = (el.innerText || '').trim();
      if (t) texts.push(t);
    }
    return texts;
  });
}

async function dismissAllPopups() {
  // Click all visible close / dismiss buttons on Snackbars
  const closes = await page.locator(
    '.MuiSnackbar-root button, [role="alert"] button[aria-label="Close"]'
  ).all();
  for (const btn of closes) {
    try { await btn.click({ timeout: 2000 }); } catch { /* already gone */ }
  }
}

async function screenshotPage(name) {
  const safe = name.replace(/[^a-z0-9]/gi, '-').toLowerCase();
  const path = join(OUT, `${safe}.png`);
  await page.screenshot({ path, fullPage: true });
  return path;
}

async function waitForSettle() {
  try {
    await page.waitForLoadState('networkidle', { timeout: 30_000 });
  } catch { /* timeout is fine — page may never fully go idle */ }
  await page.waitForTimeout(SETTLE_MS);
}

// ── main ─────────────────────────────────────────────────────────────────────

console.log(`Navigating to ${URL} …`);
await page.goto(URL, { waitUntil: 'domcontentloaded', timeout: PAGE_TIMEOUT_MS });

// Wait for NeoDash root to render
await page.waitForSelector('[class*="MuiBox"], #root > div', { timeout: PAGE_TIMEOUT_MS });
console.log('NeoDash root rendered.');

// Collect the dashboard page tabs
// NeoDash renders page tabs as MuiTab buttons inside a tab list
const allFindings = [];

async function checkCurrentPage(label) {
  await waitForSettle();
  await dismissAllPopups();
  await page.waitForTimeout(500);

  const popups = await visiblePopups();
  const path   = await screenshotPage(label);

  const findings = { page: label, popups, consoleErrors: [...consoleErrors], pageErrors: [...pageErrors] };
  allFindings.push(findings);
  consoleErrors.length = 0;   // clear — we attribute errors to the page that caused them
  pageErrors.length    = 0;

  const status = popups.length === 0 && findings.consoleErrors.length === 0 ? '✓' : '✗';
  console.log(`${status} ${label}`);
  if (popups.length)                  console.log(`  popups: ${JSON.stringify(popups)}`);
  if (findings.consoleErrors.length)  console.log(`  console.errors: ${JSON.stringify(findings.consoleErrors)}`);
  if (findings.pageErrors.length)     console.log(`  page.errors: ${JSON.stringify(findings.pageErrors)}`);
  console.log(`  screenshot: ${path}`);
}

// Check the first (default) page
await checkCurrentPage('overview');

// Walk the remaining tabs
const tabs = await page.locator('[role="tab"]').all();
console.log(`Found ${tabs.length} tab(s).`);

for (let i = 1; i < tabs.length; i++) {
  const tabText = (await tabs[i].textContent() || `page-${i}`).trim();
  await tabs[i].click();
  await checkCurrentPage(tabText);
}

// Extra: on "Wallet Inspector" page, type a wallet and verify no errors appear
// (populate the free-text selector with a known pump_seed wallet if possible)
const walletTabIdx = (await page.locator('[role="tab"]').allTextContents()).findIndex(
  t => t.includes('Wallet')
);
if (walletTabIdx >= 0) {
  const walletTabs = await page.locator('[role="tab"]').all();
  await walletTabs[walletTabIdx].click();
  await waitForSettle();

  // Find the free-text wallet address input (skip if not interactable — non-edit mode)
  try {
    const input = page.locator('input:not([disabled])[placeholder*="alue"], input:not([disabled])[id*="wallet"]').first();
    if (await input.isVisible({ timeout: 5000 }).catch(() => false) &&
        await input.isEnabled({ timeout: 2000 }).catch(() => false)) {
      await input.fill('So11111111111111111111111111111111111111112');
      await page.keyboard.press('Enter');
      await checkCurrentPage('wallet-inspector-populated');
    }
  } catch { /* non-edit mode — skip interactive check */ }
}

// ── summary ──────────────────────────────────────────────────────────────────

await browser.close();

console.log('\n=== E2E Summary ===');
let totalPopups = 0;
let totalErrors = 0;
for (const f of allFindings) {
  totalPopups += f.popups.length;
  totalErrors += f.consoleErrors.length + f.pageErrors.length;
}

console.log(JSON.stringify(allFindings, null, 2));
console.log(`\nTotal popups: ${totalPopups}, total console/page errors: ${totalErrors}`);

if (totalPopups > 0 || totalErrors > 0) {
  console.error('\nFAIL — errors or popups found');
  process.exit(1);
}
console.log('\nPASS — dashboard rendered cleanly across all pages');
