/**
 * search-caret.spec.mjs — Search input/overlay alignment.
 */

import { test, expect } from '@playwright/test';
import { setTimeout as delay } from 'timers/promises';
import { portForTestFile, startServer, stopServer } from './helpers.mjs';

let server;
let baseURL;

test.describe('search-caret', () => {
  test.beforeAll(async ({}, workerInfo) => {
    const port = portForTestFile(workerInfo);
    server = await startServer(port, { rows: 80 });
    baseURL = `http://localhost:${port}`;
  });

  test.afterAll(async () => {
    await stopServer(server);
  });

  test('keeps highlight overlay aligned with input metrics and scroll', async ({ page }) => {
    await page.goto(baseURL);
    await delay(800);

    const input = page.locator('.search-editor-input');
    const longQuery = 'pod:pod-14 AND level:ERROR AND message:*timeout* AND region:us-west-2 AND service:api-gateway AND env:prod';
    await input.fill(longQuery);

    await page.evaluate(() => {
      const inp = document.querySelector('.search-editor-input');
      inp.scrollLeft = inp.scrollWidth;
    });
    await delay(100);

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
    expect(styleMismatch).toEqual([]);

    const scrollDiff = await page.evaluate(() => {
      const inp = document.querySelector('.search-editor-input');
      const hl = document.querySelector('.search-highlight');
      return Math.abs(inp.scrollLeft - hl.scrollLeft);
    });
    expect(scrollDiff).toBeLessThanOrEqual(1);

    const displayMode = await page.evaluate(() =>
      getComputedStyle(document.querySelector('.search-highlight')).display
    );
    expect(displayMode).toBe('block');
  });
});
