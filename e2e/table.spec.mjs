#!/usr/bin/env node
/**
 * table.spec.mjs — Core table behaviour
 *
 * Covers: row rendering, expand/collapse with scroll preservation,
 * adding/removing pinned columns, empty cell rendering, resize handles,
 * and sticky header.
 */

import { chromium } from 'playwright';
import { setTimeout } from 'timers/promises';
import {
  assert, printSummary,
  startServer, getScroll, setScroll,
  clickFieldKey, expandRow, expandCollapsedRow, getHeaders,
} from './helpers.mjs';

const PORT = 9997;
const BASE_URL = `http://localhost:${PORT}`;

let server = null;
let browser = null;

async function cleanup() {
  if (browser) await browser.close().catch(() => {});
  if (server) server.kill();
}
process.on('SIGINT', cleanup);

try {
  // ── Setup ────────────────────────────────────────
  console.log('⏳ Starting peek server (go run)…');
  server = await startServer(PORT);
  console.log('✅ Server ready');

  const headless = !!(process.env.CI || process.env.HEADLESS);
  browser = await chromium.launch({ headless, ...(headless ? {} : { slowMo: 80 }) });
  const ctx = await browser.newContext({ viewport: { width: 1400, height: 900 } });
  const page = await ctx.newPage();
  page.on('pageerror', err => console.error('  PAGE ERROR:', err.message));

  await page.goto(BASE_URL);
  await setTimeout(2000); // let VanJS render + observer init

  // ── 1. Table renders rows ───────────────────────
  console.log('\n1️⃣  Table renders rows');
  const rowCount = await page.evaluate(() =>
    document.querySelectorAll('.log-row').length
  );
  assert('Has log rows', rowCount >= 30, `${rowCount} rows`);

  const headers = await getHeaders(page);
  assert(
    'Default headers: time, level, message',
    headers.length === 3 && headers.includes('time') && headers.includes('level') && headers.includes('message'),
  );

  // ── 2. Expand row preserves scroll ──────────────
  console.log('\n2️⃣  Expand row preserves scroll');
  await setScroll(page, 400);
  const scrollBefore = await getScroll(page);
  assert('Scrolled to target', scrollBefore > 100, `${scrollBefore}px`);

  // Store scroll in window property so the app can restore it
  await page.evaluate(s => { window.scrollPreserveValue = s; }, scrollBefore);
  await expandRow(page);
  await setTimeout(800);

  const scrollAfterExpand = await getScroll(page);
  const expandDrift = Math.abs(scrollAfterExpand - scrollBefore);
  assert('Scroll preserved after expand', expandDrift <= 30, `drift ${expandDrift}px`);

  const detailVisible = await page.evaluate(() =>
    document.querySelectorAll('.detail-row.visible').length > 0
  );
  assert('Detail row is visible', detailVisible);

  // ── 3. Add column "service" ─────────────────────
  console.log('\n3️⃣  Add column "service"');
  const scrollBeforeCol = await getScroll(page);

  const clicked = await clickFieldKey(page, 'service');
  assert('Clicked service field', clicked);
  await setTimeout(800);

  const headersAfter = await getHeaders(page);
  assert('Header includes "service"', headersAfter.includes('service'), headersAfter.join(', '));

  const pinnedCount = await page.evaluate(() =>
    document.querySelectorAll('.pinned-val').length
  );
  assert('Pinned value cells exist', pinnedCount >= 30, `${pinnedCount} cells`);

  const colDrift = Math.abs((await getScroll(page)) - scrollBeforeCol);
  assert('Scroll stable after add column', colDrift <= 50, `drift ${colDrift}px`);

  // ── 4. Add second column "user_id" ──────────────
  console.log('\n4️⃣  Add column "user_id"');
  await expandCollapsedRow(page);
  await setTimeout(500);

  const clicked2 = await clickFieldKey(page, 'user_id');
  assert('Clicked user_id field', clicked2);
  await setTimeout(800);

  const headers2 = await getHeaders(page);
  assert('Header includes "user_id"', headers2.includes('user_id'), headers2.join(', '));

  // ── 5. No "-" in empty cells ────────────────────
  console.log('\n5️⃣  Empty cell rendering');
  const hasDash = await page.evaluate(() =>
    Array.from(document.querySelectorAll('.pinned-val'))
      .some(c => c.textContent.trim() === '-')
  );
  assert('No "-" in pinned cells', !hasDash);

  // ── 6. Resize handles exist ─────────────────────
  console.log('\n6️⃣  Column resize');
  const handles = await page.evaluate(() =>
    document.querySelectorAll('.resize-handle').length
  );
  assert('Resize handles present', handles > 0, `${handles} handles`);

  // ── 7. Sticky header ───────────────────────────
  console.log('\n7️⃣  Sticky header');
  await setScroll(page, 999_999);
  await setTimeout(300);
  const headerVisible = await page.evaluate(() => {
    const h = document.querySelector('.log-table-header');
    if (!h) return false;
    const r = h.getBoundingClientRect();
    return r.top >= 0 && r.top < 200;
  });
  assert('Header stays visible at bottom', headerVisible);

  // ── Summary ─────────────────────────────────────
  await page.screenshot({ path: '/tmp/peek-test-table.png', fullPage: false });
  console.log('\n📸 Screenshot: /tmp/peek-test-table.png');

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
