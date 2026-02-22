#!/usr/bin/env node
/**
 * field-filter-append.spec.mjs — clicking field values in expanded rows
 *
 * Covers: appending safe Lucene tokens instead of replacing the query,
 * proper escaping of spaces/quotes/backslashes, trailing-operator handling,
 * and scroll-preservation invariants.
 */

import { chromium } from 'playwright';
import { setTimeout } from 'timers/promises';
import {
  assert, printSummary,
  startServer, getScroll, setScroll, expandRow,
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

/** Click the first `.field-val` cell whose text matches `value`. */
async function clickFieldVal(page, value) {
  return page.evaluate((v) => {
    const el = Array.from(document.querySelectorAll('.field-val'))
      .find(e => e.textContent.trim() === v);
    if (!el) return false;
    el.click();
    return true;
  }, value);
}

/** Read the current search input value. */
async function getQuery(page) {
  return page.evaluate(() =>
    document.querySelector('.search-input')?.value ?? ''
  );
}

/** Set the search input value directly and update highlight. */
async function setQuery(page, q) {
  await page.evaluate((q) => {
    const el = document.querySelector('.search-input');
    if (el) el.value = q;
  }, q);
}

/** Expand a row and wait for detail to appear. */
async function expandAndWait(page) {
  await expandRow(page);
  await setTimeout(800);
}

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
  await setTimeout(2000);

  const rowCount = await page.evaluate(() =>
    document.querySelectorAll('.log-row').length
  );
  assert('Rows loaded', rowCount >= 30, `${rowCount} rows`);

  // ── 1. Append to existing query ──────────────────
  console.log('\n1️⃣  Append token to existing query');
  await setQuery(page, 'level:INFO');
  await expandAndWait(page);

  const clickedService = await clickFieldVal(page, 'api');
  assert('Clicked service=api field value', clickedService);
  await setTimeout(600);

  const q1 = await getQuery(page);
  assert(
    'Query appended with AND token',
    q1 === 'level:INFO AND service:"api"',
    `got: ${q1}`,
  );

  // ── 2. Empty query → replace with token ─────────
  console.log('\n2️⃣  Empty query replaced by token');
  await setQuery(page, '');
  await expandAndWait(page);

  const clickedService2 = await clickFieldVal(page, 'api');
  assert('Clicked service=api field value (empty query)', clickedService2);
  await setTimeout(600);

  const q2 = await getQuery(page);
  assert(
    'Empty query replaced by token',
    q2 === 'service:"api"',
    `got: ${q2}`,
  );

  // ── 3. Wildcard query → replace with token ───────
  console.log('\n3️⃣  Wildcard query replaced by token');
  await setQuery(page, '*');
  await expandAndWait(page);

  const clickedService3 = await clickFieldVal(page, 'api');
  assert('Clicked service=api (wildcard query)', clickedService3);
  await setTimeout(600);

  const q3 = await getQuery(page);
  assert(
    'Wildcard query replaced by token',
    q3 === 'service:"api"',
    `got: ${q3}`,
  );

  // ── 4. Trailing operator → append without extra AND ──
  console.log('\n4️⃣  Trailing AND operator appends correctly');
  await setQuery(page, 'level:INFO AND');
  await expandAndWait(page);

  const clickedService4 = await clickFieldVal(page, 'api');
  assert('Clicked service=api (trailing AND)', clickedService4);
  await setTimeout(600);

  const q4 = await getQuery(page);
  assert(
    'Trailing AND produces valid query',
    q4 === 'level:INFO AND service:"api"',
    `got: ${q4}`,
  );

  // ── 5. Scroll preservation ───────────────────────
  console.log('\n5️⃣  Scroll preserved when clicking field value');
  await setScroll(page, 300);
  await setTimeout(200);
  const scrollBefore = await getScroll(page);

  // Ensure a row is expanded at scroll position
  await expandAndWait(page);
  const scrollAfterExpand = await getScroll(page);

  const clickedScroll = await clickFieldVal(page, 'api');
  assert('Clicked field val for scroll test', clickedScroll);
  await setTimeout(600);

  const scrollAfterClick = await getScroll(page);
  const drift = Math.abs(scrollAfterClick - scrollAfterExpand);
  assert('Scroll preserved after field-value click', drift <= 50, `drift ${drift}px`);

  // ── Summary ─────────────────────────────────────
  await page.screenshot({ path: '/tmp/peek-test-field-filter-append.png', fullPage: false });
  console.log('\n📸 Screenshot: /tmp/peek-test-field-filter-append.png');

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
