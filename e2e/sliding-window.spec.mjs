#!/usr/bin/env node
/**
 * sliding-window.spec.mjs — Relative time presets slide without /query polling
 *
 * Covers:
 * - Relative preset ("15m") prunes stale rows via client timer
 * - Sliding does not trigger periodic /query re-polling
 * - Fixed preset ("today") does not apply sliding prune
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
  console.log('⏳ Starting peek server…');
  server = await startServer(PORT);
  console.log('✅ Server ready');

  browser = await chromium.launch({ headless: !!(process.env.CI || process.env.HEADLESS) });
  const ctx = await browser.newContext({ viewport: { width: 1400, height: 900 } });
  const page = await ctx.newPage();
  page.on('pageerror', err => console.error('  PAGE ERROR:', err.message));

  // Freeze WS behavior for this spec so /query drives deterministic state.
  await page.addInitScript(() => {
    const realNow = Date.now.bind(Date);
    window.__peekNowOffsetMs = 0;
    Date.now = () => realNow() + (window.__peekNowOffsetMs || 0);

    class FakeWebSocket {
      static OPEN = 1;
      static CLOSED = 3;

      constructor() {
        this.readyState = FakeWebSocket.OPEN;
        setTimeout(() => {
          if (typeof this.onopen === 'function') this.onopen();
        }, 0);
      }

      send() {}

      close() {
        this.readyState = FakeWebSocket.CLOSED;
        if (typeof this.onclose === 'function') this.onclose();
      }
    }

    window.WebSocket = FakeWebSocket;
  });

  let queryCount = 0;
  await page.route('**/query', async route => {
    queryCount++;
    const now = Date.now();
    const logs = [
      {
        id: `sw-1-${queryCount}`,
        timestamp: new Date(now - 10 * 60 * 1000).toISOString(),
        level: 'INFO',
        message: 'within-window',
        fields: { service: 'api' },
        raw: '{"msg":"within-window"}',
      },
      {
        id: `sw-2-${queryCount}`,
        timestamp: new Date(now - 14 * 60 * 1000).toISOString(),
        level: 'WARN',
        message: 'near-edge',
        fields: { service: 'api' },
        raw: '{"msg":"near-edge"}',
      },
    ];

    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ logs, total: logs.length, took_ms: 1 }),
    });
  });

  await page.goto(BASE_URL);
  await page.waitForSelector('[data-testid="time-preset"]', { timeout: 15000 });
  await setTimeout(400);

  const baselineCount = queryCount;

  // ── 1. Sliding prune works on relative preset ─────────────────
  console.log('\n1️⃣  Relative preset prunes stale rows without re-query');
  await page.evaluate(() => {
    const s = document.querySelector('[data-testid="time-preset"]');
    s.value = '15m';
    s.dispatchEvent(new Event('change', { bubbles: true }));
  });
  await setTimeout(700);

  const beforeRows = await page.evaluate(() => document.querySelectorAll('.log-row').length);
  assert('Rows visible after selecting 15m', beforeRows >= 2, `${beforeRows} rows`);

  await page.evaluate(() => { window.__peekNowOffsetMs = 20 * 60 * 1000; });
  await setTimeout(1400); // > SLIDING_TICK_MS

  const afterRows = await page.evaluate(() => document.querySelectorAll('.log-row').length);
  assert('Rows pruned after +20m time jump', afterRows === 0, `${afterRows} rows`);

  // ── 2. No periodic /query polling ─────────────────────────────
  console.log('\n2️⃣  No periodic /query polling while sliding');
  const afterSelectCount = queryCount;
  await setTimeout(6100);
  assert(
    'No extra /query calls over ~6s',
    queryCount === afterSelectCount,
    `count ${afterSelectCount} -> ${queryCount}`,
  );

  // ── 3. Fixed preset does not slide ────────────────────────────
  console.log('\n3️⃣  Fixed preset remains fixed');
  await page.evaluate(() => { window.__peekNowOffsetMs = 0; });
  await page.evaluate(() => {
    const s = document.querySelector('[data-testid="time-preset"]');
    s.value = 'today';
    s.dispatchEvent(new Event('change', { bubbles: true }));
  });
  await setTimeout(700);

  const todayRowsBefore = await page.evaluate(() => document.querySelectorAll('.log-row').length);
  assert('Rows visible for fixed preset', todayRowsBefore >= 2, `${todayRowsBefore} rows`);

  await page.evaluate(() => { window.__peekNowOffsetMs = 20 * 60 * 1000; });
  await setTimeout(1400);

  const todayRowsAfter = await page.evaluate(() => document.querySelectorAll('.log-row').length);
  assert(
    'Rows unchanged for fixed preset after +20m',
    todayRowsAfter === todayRowsBefore,
    `${todayRowsBefore} -> ${todayRowsAfter}`,
  );

  assert('At least one /query call happened after page load', queryCount > baselineCount, `${baselineCount} -> ${queryCount}`);

  const exitCode = printSummary();
  process.exit(exitCode);
} catch (err) {
  console.error('❌ Error:', err);
  process.exit(1);
} finally {
  await cleanup();
}
