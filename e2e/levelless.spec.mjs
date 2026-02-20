#!/usr/bin/env node
/**
 * levelless.spec.mjs — Levelless log entry rendering
 *
 * Pipes log lines without a level field and verifies that:
 * - Rows appear in the table
 * - Level cells show an em dash (—) instead of blank or "INFO"
 * - Level cells carry the level-NONE CSS class
 * - level:ERROR queries do not match levelless entries
 */

import { chromium } from 'playwright';
import { setTimeout } from 'timers/promises';
import { spawn } from 'child_process';
import { dirname, resolve } from 'path';
import { fileURLToPath } from 'url';
import { assert, printSummary } from './helpers.mjs';

const __dirname = dirname(fileURLToPath(import.meta.url));
const PROJECT_ROOT = resolve(__dirname, '..');

const PORT = 9997;
const BASE_URL = `http://localhost:${PORT}`;
const STARTUP_TIMEOUT_MS = 30_000;
const POLL_INTERVAL_MS = 1_000;

let server = null;
let browser = null;

async function cleanup() {
  if (browser) await browser.close().catch(() => {});
  if (server) server.kill();
}
process.on('SIGINT', cleanup);

try {
  // ── Setup: pipe log lines without a level field ───────────────
  console.log('⏳ Starting peek server with levelless logs…');

  const testDbPath = `/tmp/peek-e2e-levelless-${PORT}`;
  const cmd = `
    rm -rf ${testDbPath} && \
    go build -o /tmp/peek-e2e-bin ./cmd/peek && \
    for i in $(seq 1 10); do
      printf '{"msg":"plain message %d","time":"2026-02-18T10:%02d:00Z","service":"svc"}\\n' "$i" "$i"
    done | /tmp/peek-e2e-bin --port ${PORT} --no-browser --db-path ${testDbPath} --all --retention-days 0 --retention-size 10GB > /tmp/peek-e2e-levelless-${PORT}.log 2>&1
  `;
  server = spawn('sh', ['-c', cmd], { cwd: PROJECT_ROOT });

  const deadline = Date.now() + STARTUP_TIMEOUT_MS;
  while (Date.now() < deadline) {
    try {
      const resp = await fetch(BASE_URL);
      if (resp.ok) break;
    } catch { /* not ready yet */ }
    await setTimeout(POLL_INTERVAL_MS);
  }
  if (Date.now() >= deadline) {
    server.kill();
    throw new Error(`peek server did not start within ${STARTUP_TIMEOUT_MS / 1000}s`);
  }
  console.log('✅ Server ready');

  const headless = !!(process.env.CI || process.env.HEADLESS);
  browser = await chromium.launch({ headless, ...(headless ? {} : { slowMo: 80 }) });
  const ctx = await browser.newContext({ viewport: { width: 1400, height: 900 } });
  const page = await ctx.newPage();
  page.on('pageerror', err => console.error('  PAGE ERROR:', err.message));

  await page.goto(BASE_URL);
  await setTimeout(2000);

  // ── 1. Rows appear ───────────────────────────────────────────
  console.log('\n1️⃣  Levelless rows appear in table');
  const rowCount = await page.evaluate(() =>
    document.querySelectorAll('.log-row').length
  );
  assert('Has log rows', rowCount >= 10, `${rowCount} rows`);

  // ── 2. Level cells show em dash, not "INFO" or blank ─────────
  console.log('\n2️⃣  Level cells show em dash');
  const levelTexts = await page.evaluate(() =>
    Array.from(document.querySelectorAll('.col-level'))
      .map(el => el.textContent.trim())
  );
  const hasEmDash = levelTexts.some(t => t === '\u2014');
  assert('At least one level cell shows em dash', hasEmDash, JSON.stringify(levelTexts.slice(0, 5)));

  const hasINFO = levelTexts.some(t => t === 'INFO');
  assert('No level cell shows synthetic INFO', !hasINFO, JSON.stringify(levelTexts.slice(0, 5)));

  // ── 3. Level cells carry level-NONE class ────────────────────
  console.log('\n3️⃣  Level cells carry level-NONE class');
  const noneCount = await page.evaluate(() =>
    document.querySelectorAll('.col-level.level-NONE').length
  );
  assert('level-NONE cells exist', noneCount >= 10, `${noneCount} cells`);

  // ── 4. level:ERROR query returns 0 results ───────────────────
  console.log('\n4️⃣  level:ERROR query does not match levelless entries');
  const queryRes = await fetch(`${BASE_URL}/query`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ query: 'level:ERROR', limit: 100, offset: 0 }),
  });
  const queryData = await queryRes.json();
  assert('level:ERROR returns 0 results', queryData.total === 0, `total=${queryData.total}`);

  // ── Summary ──────────────────────────────────────────────────
  await page.screenshot({ path: '/tmp/peek-test-levelless.png', fullPage: false });
  console.log('\n📸 Screenshot: /tmp/peek-test-levelless.png');

  const exitCode = printSummary();

  if (exitCode > 0) {
    console.log('\n🔴 Some tests failed. Browser open 15s…');
    await setTimeout(15_000);
  } else {
    console.log('\n🟢 All tests passed!');
    await setTimeout(3_000);
  }

  process.exit(exitCode);

} catch (error) {
  console.error('❌ Error:', error);
  process.exit(1);
} finally {
  await cleanup();
}
