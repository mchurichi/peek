#!/usr/bin/env node
/**
 * datetime.spec.mjs — Datetime range picker UI and API integration
 *
 * Covers: /query API accepts start/end params, preset dropdown in HTML source,
 * and (when VanJS loads) custom range input visibility.
 */

import { chromium } from 'playwright';
import { setTimeout } from 'timers/promises';
import { assert, printSummary, startServer } from './helpers.mjs';

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
  console.log('⏳ Starting peek server…');
  server = await startServer(PORT);
  console.log('✅ Server ready');

  const headless = !!(process.env.CI || process.env.HEADLESS);
  browser = await chromium.launch({ headless, ...(headless ? {} : { slowMo: 80 }) });
  const ctx = await browser.newContext({ viewport: { width: 1400, height: 900 } });
  const page = await ctx.newPage();
  page.on('pageerror', err => console.error('  PAGE ERROR:', err.message));

  await page.goto(BASE_URL);
  await setTimeout(2000); // Wait for VanJS (needs CDN access in CI)

  // ── 1. HTML source contains datetime picker ────────
  console.log('\n1️⃣  HTML source contains datetime range picker');
  const rawHtml = await page.evaluate(() => document.documentElement.outerHTML);
  assert('HTML contains time-range-picker class',  rawHtml.includes('time-range-picker'),  'HTML source');
  assert('HTML contains time-preset select',        rawHtml.includes('time-preset'),         'HTML source');
  assert('HTML contains time-custom-range class',   rawHtml.includes('time-custom-range'),   'HTML source');
  assert('HTML contains DateRangePicker function',  rawHtml.includes('DateRangePicker'),     'HTML source');
  assert('HTML contains getTimeRange function',     rawHtml.includes('getTimeRange'),        'HTML source');

  // ── 2. POST /query accepts start/end params ────────
  console.log('\n2️⃣  POST /query with start/end parameters');
  const nowISO  = new Date().toISOString();
  const hourAgo = new Date(Date.now() - 60 * 60 * 1000).toISOString();

  const qResp = await page.evaluate(async ({ start, end }) => {
    const r = await fetch('/query', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ query: '*', limit: 100, offset: 0, start, end }),
    });
    const body = await r.json();
    return { status: r.status, body };
  }, { start: hourAgo, end: nowISO });

  assert('/query with start/end returns 200', qResp.status === 200, `status=${qResp.status}`);
  assert('/query with start/end returns logs array', Array.isArray(qResp.body.logs), JSON.stringify(qResp.body).slice(0, 100));

  // ── 3. /query filters by time range ───────────────
  console.log('\n3️⃣  /query time range filters results');

  // Query with far-future range — should return no logs
  const futureResp = await page.evaluate(async () => {
    const r = await fetch('/query', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        query: '*', limit: 100, offset: 0,
        start: '2099-01-01T00:00:00Z',
        end:   '2099-12-31T23:59:59Z',
      }),
    });
    return r.json();
  });
  assert('/query far-future range returns 0 logs', futureResp.total === 0,
    `total=${futureResp.total}`);

  // Query with far-past range — should return no logs
  const pastResp = await page.evaluate(async () => {
    const r = await fetch('/query', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        query: '*', limit: 100, offset: 0,
        start: '2000-01-01T00:00:00Z',
        end:   '2000-12-31T23:59:59Z',
      }),
    });
    return r.json();
  });
  assert('/query far-past range returns 0 logs', pastResp.total === 0,
    `total=${pastResp.total}`);

  // Query with no time range — should return all logs
  const allResp = await page.evaluate(async () => {
    const r = await fetch('/query', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ query: '*', limit: 100, offset: 0 }),
    });
    return r.json();
  });
  assert('/query without time range returns all logs', allResp.total > 0,
    `total=${allResp.total}`);

  // ── 4. DOM-based tests (require VanJS / CDN access) ──
  console.log('\n4️⃣  Datetime picker DOM (requires CDN)');

  const hasPresetSelect = await page.evaluate(() =>
    !!document.querySelector('[data-testid="time-preset"]')
  );
  assert('preset select rendered in DOM', hasPresetSelect, '(requires VanJS CDN)');

  if (hasPresetSelect) {
    const optionValues = await page.evaluate(() => {
      const s = document.querySelector('[data-testid="time-preset"]');
      return Array.from(s.options).map(o => o.value);
    });
    assert('preset has "all" option',  optionValues.includes('all'),  optionValues.join(', '));
    assert('preset has "1h" option',   optionValues.includes('1h'),   optionValues.join(', '));
    assert('preset has "7d" option',   optionValues.includes('7d'),   optionValues.join(', '));
    assert('preset has "custom" option', optionValues.includes('custom'), optionValues.join(', '));

    // Custom range hidden by default
    const customHidden = await page.evaluate(() => {
      const el = document.querySelector('.time-custom-range');
      return !el || el.style.display === 'none';
    });
    assert('custom range hidden by default', customHidden);

    // Select "custom" — custom range should show
    await page.evaluate(() => {
      const s = document.querySelector('[data-testid="time-preset"]');
      s.value = 'custom';
      s.dispatchEvent(new Event('change', { bubbles: true }));
    });
    await setTimeout(400);

    const customVisible = await page.evaluate(() => {
      const el = document.querySelector('.time-custom-range');
      return el && el.style.display !== 'none';
    });
    assert('custom range visible after selecting "custom"', customVisible);

    // Verify preset query sends start param
    let capturedBody = null;
    await page.route('/query', async route => {
      capturedBody = JSON.parse(route.request().postData() || '{}');
      await route.continue();
    });
    await page.evaluate(() => {
      const s = document.querySelector('[data-testid="time-preset"]');
      s.value = '1h';
      s.dispatchEvent(new Event('change', { bubbles: true }));
    });
    await setTimeout(1000);
    assert('preset "1h" sends start param', capturedBody?.start != null,
      JSON.stringify(capturedBody));
  }

  // ── Screenshot ────────────────────────────────────
  await page.screenshot({ path: '/tmp/peek-test-datetime.png', fullPage: false });
  console.log('\n📸 Screenshot saved to /tmp/peek-test-datetime.png');

} finally {
  await cleanup();
  const code = printSummary();
  process.exit(code);
}

