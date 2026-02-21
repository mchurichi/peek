#!/usr/bin/env node
/**
 * search-caret.spec.mjs — Search input/overlay alignment
 *
 * Verifies the syntax-highlight overlay matches input metrics and scroll
 * so the caret position aligns with rendered tokens.
 */

import { chromium } from 'playwright';
import { setTimeout as sleep } from 'timers/promises';
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
  server = await startServer(PORT, { rows: 80 });
  console.log('✅ Server ready');

  const headless = !!(process.env.CI || process.env.HEADLESS);
  browser = await chromium.launch({ headless, ...(headless ? {} : { slowMo: 60 }) });
  const ctx = await browser.newContext({ viewport: { width: 1400, height: 900 } });
  const page = await ctx.newPage();
  await page.goto(BASE_URL);
  await sleep(800);

  const inputSel = '.search-editor-input';
  const hlSel = '.search-highlight';

  const longQuery = 'pod:pod-14 AND level:ERROR AND message:*timeout* AND region:us-west-2 AND service:api-gateway AND env:prod';
  const input = page.locator(inputSel);
  await input.fill(longQuery);

  // Force scroll to the end to test scrollLeft sync
  await page.evaluate(() => {
    const inp = document.querySelector('.search-editor-input');
    inp.scrollLeft = inp.scrollWidth;
  });
  await sleep(100);

  // 1) Style parity between input and highlight
  const styleMismatch = await page.evaluate(() => {
    const inp = document.querySelector('.search-editor-input');
    const hl = document.querySelector('.search-highlight');
    const props = ['fontFamily', 'fontSize', 'lineHeight', 'paddingLeft', 'paddingRight', 'boxSizing'];
    const mismatches = [];
    for (const p of props) {
      const iv = getComputedStyle(inp)[p];
      const hv = getComputedStyle(hl)[p];
      if (iv !== hv) mismatches.push({ prop: p, input: iv, highlight: hv });
    }
    return mismatches;
  });
  assert('Input/highlight styles match', styleMismatch.length === 0, JSON.stringify(styleMismatch));

  // 2) Scroll sync (caret position alignment)
  const scrollDiff = await page.evaluate(() => {
    const inp = document.querySelector('.search-editor-input');
    const hl = document.querySelector('.search-highlight');
    return Math.abs(inp.scrollLeft - hl.scrollLeft);
  });
  assert('Highlight scroll matches input', scrollDiff <= 1, `diff=${scrollDiff}`);

  // 3) No flex centering in highlight (prevents text offset)
  const displayMode = await page.evaluate(() => getComputedStyle(document.querySelector('.search-highlight')).display);
  assert('Highlight display is block', displayMode === 'block', displayMode);

  const exitCode = printSummary();
  await cleanup();
  process.exit(exitCode);

} catch (err) {
  console.error('❌ Error:', err);
  await cleanup();
  process.exit(1);
}
