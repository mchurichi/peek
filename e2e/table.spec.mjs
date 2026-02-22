/**
 * table.spec.mjs — Core table behaviour.
 */

import { test, expect } from '@playwright/test';
import { setTimeout as delay } from 'timers/promises';
import {
  clickFieldKey,
  expandCollapsedRow,
  expandRow,
  getHeaders,
  getScroll,
  portForTestFile,
  setScroll,
  startServer,
  stopServer,
} from './helpers.mjs';

let server;
let baseURL;

test.describe('table', () => {
  test.beforeAll(async ({}, workerInfo) => {
    const port = portForTestFile(workerInfo);
    server = await startServer(port);
    baseURL = `http://localhost:${port}`;
  });

  test.afterAll(async () => {
    await stopServer(server);
  });

  test('renders rows and preserves scroll on row/column changes', async ({ page }) => {
    await page.goto(baseURL);
    await delay(2000);

    const rowCount = await page.evaluate(() => document.querySelectorAll('.log-row').length);
    expect(rowCount).toBeGreaterThanOrEqual(30);

    const headers = await getHeaders(page);
    expect(headers.length).toBe(3);
    expect(headers).toEqual(expect.arrayContaining(['time', 'level', 'message']));

    await setScroll(page, 400);
    const scrollBefore = await getScroll(page);
    expect(scrollBefore).toBeGreaterThan(100);

    await page.evaluate((s) => { window.scrollPreserveValue = s; }, scrollBefore);
    expect(await expandRow(page)).toBeTruthy();
    await delay(800);

    const scrollAfterExpand = await getScroll(page);
    const expandDrift = Math.abs(scrollAfterExpand - scrollBefore);
    expect(expandDrift).toBeLessThanOrEqual(30);

    const detailVisible = await page.evaluate(() =>
      document.querySelectorAll('.detail-row.visible').length > 0
    );
    expect(detailVisible).toBeTruthy();

    const scrollBeforeCol = await getScroll(page);
    expect(await clickFieldKey(page, 'service')).toBeTruthy();
    await delay(800);

    const headersAfter = await getHeaders(page);
    expect(headersAfter).toContain('service');

    const pinnedCount = await page.evaluate(() =>
      document.querySelectorAll('.pinned-val').length
    );
    expect(pinnedCount).toBeGreaterThanOrEqual(30);

    const colDrift = Math.abs((await getScroll(page)) - scrollBeforeCol);
    expect(colDrift).toBeLessThanOrEqual(50);

    await expandCollapsedRow(page);
    await delay(500);
    expect(await clickFieldKey(page, 'user_id')).toBeTruthy();
    await delay(800);

    const headers2 = await getHeaders(page);
    expect(headers2).toContain('user_id');

    const hasDash = await page.evaluate(() =>
      Array.from(document.querySelectorAll('.pinned-val'))
        .some((c) => c.textContent.trim() === '-')
    );
    expect(hasDash).toBeFalsy();

    const handles = await page.evaluate(() =>
      document.querySelectorAll('.resize-handle').length
    );
    expect(handles).toBeGreaterThan(0);

    await setScroll(page, 999_999);
    await delay(300);
    const stickyMetrics = await page.evaluate(() => {
      const h = document.querySelector('.log-table-header');
      const c = document.querySelector('.log-container');
      if (!h || !c) return null;
      const r = h.getBoundingClientRect();
      const cr = c.getBoundingClientRect();
      return { headerTop: r.top, containerTop: cr.top };
    });

    expect(stickyMetrics).not.toBeNull();
    const drift = Math.abs(stickyMetrics.headerTop - stickyMetrics.containerTop);
    expect(drift).toBeLessThanOrEqual(3);
  });
});
