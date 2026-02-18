#!/usr/bin/env node
/**
 * resize.spec.mjs — Column resize behaviour
 *
 * Covers: drag-to-resize on time, level, and message columns, verifying
 * that only the targeted column changes width and others remain stable.
 */

import { chromium } from 'playwright';
import { setTimeout } from 'timers/promises';
import {
  assert, printSummary, startServer,
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

/** Snapshot widths of all header cells. */
async function headerWidths(page) {
  return page.evaluate(() =>
    Array.from(document.querySelectorAll('.log-table-header > div'))
      .map(d => ({ cls: d.className.split(' ')[0], width: d.offsetWidth, text: d.textContent.trim() }))
  );
}

/** Drag a resize handle by `dx` pixels. */
async function dragHandle(page, colClass, dx) {
  const handle = page.locator(`.log-table-header .${colClass} .resize-handle`);
  const box = await handle.boundingBox();
  if (!box) throw new Error(`No resize handle for ${colClass}`);

  const x = box.x + box.width / 2;
  const y = box.y + box.height / 2;
  await page.mouse.move(x, y);
  await page.mouse.down();
  await page.mouse.move(x + dx, y, { steps: 15 });
  await page.mouse.up();
  await setTimeout(300);
}

try {
  console.log('⏳ Starting peek server…');
  server = await startServer(PORT, { rows: 5 });
  console.log('✅ Server ready');

  const headless = !!(process.env.CI || process.env.HEADLESS);
  browser = await chromium.launch({ headless, ...(headless ? {} : { slowMo: 80 }) });
  const ctx = await browser.newContext({ viewport: { width: 1400, height: 900 } });
  const page = await ctx.newPage();
  page.on('pageerror', err => console.error('  PAGE ERROR:', err.message));

  await page.goto(BASE_URL);
  await setTimeout(2000);

  // ── 1. Resize TIME column ──────────────────────
  console.log('\n1️⃣  Resize TIME column +100px');
  const before = await headerWidths(page);
  await dragHandle(page, 'col-time', 100);
  const after = await headerWidths(page);

  const timeBefore = before.find(h => h.cls === 'col-time')?.width ?? 0;
  const timeAfter = after.find(h => h.cls === 'col-time')?.width ?? 0;
  assert('Time column grew', timeAfter > timeBefore + 50, `${timeBefore} → ${timeAfter}px`);

  const levelBefore = before.find(h => h.cls === 'col-level')?.width ?? 0;
  const levelAfter = after.find(h => h.cls === 'col-level')?.width ?? 0;
  assert('Level column unchanged', Math.abs(levelAfter - levelBefore) < 5,
    `${levelBefore} → ${levelAfter}px`);

  // ── 2. Resize LEVEL column ──────────────────────
  console.log('\n2️⃣  Resize LEVEL column +60px');
  const before2 = await headerWidths(page);
  await dragHandle(page, 'col-level', 60);
  const after2 = await headerWidths(page);

  const lvlBefore = before2.find(h => h.cls === 'col-level')?.width ?? 0;
  const lvlAfter = after2.find(h => h.cls === 'col-level')?.width ?? 0;
  assert('Level column grew', lvlAfter > lvlBefore + 30, `${lvlBefore} → ${lvlAfter}px`);

  const timeSame = after2.find(h => h.cls === 'col-time')?.width ?? 0;
  assert('Time column unchanged after level resize', Math.abs(timeSame - timeAfter) < 5,
    `${timeAfter} → ${timeSame}px`);

  // ── 3. Grid template updated ───────────────────
  console.log('\n3️⃣  Grid template');
  const gridTemplate = await page.evaluate(() =>
    document.querySelector('.log-table')?.style.gridTemplateColumns ?? ''
  );
  assert('Grid template is set', gridTemplate.length > 0, gridTemplate.slice(0, 60));

  // ── Summary ─────────────────────────────────────
  await page.screenshot({ path: '/tmp/peek-test-resize.png', fullPage: false });
  console.log('\n📸 Screenshot: /tmp/peek-test-resize.png');

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
