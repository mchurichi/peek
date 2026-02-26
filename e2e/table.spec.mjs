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

  test('preserves scroll position when dragging column to reorder', async ({ page }) => {
    await page.goto(baseURL);
    await delay(2000);

    // First, pin two extra columns so we have something to drag
    await expandRow(page);
    await delay(500);
    expect(await clickFieldKey(page, 'service')).toBeTruthy();
    await delay(500);

    await expandCollapsedRow(page);
    await delay(500);
    expect(await clickFieldKey(page, 'user_id')).toBeTruthy();
    await delay(500);

    const headers = await getHeaders(page);
    expect(headers).toContain('service');
    expect(headers).toContain('user_id');

    // Scroll down to middle of log table
    await setScroll(page, 400);
    await delay(300);
    const scrollBefore = await getScroll(page);
    expect(scrollBefore).toBeGreaterThan(100);

    // Find the "service" and "user_id" pinned column headers for drag operation
    const serviceHeader = page.locator('.log-table-header .col-pinned[data-col="service"]');
    const userIdHeader = page.locator('.log-table-header .col-pinned[data-col="user_id"]');

    const serviceBox = await serviceHeader.boundingBox();
    const userIdBox = await userIdHeader.boundingBox();
    expect(serviceBox).not.toBeNull();
    expect(userIdBox).not.toBeNull();

    // Perform a slow drag from service column to user_id column
    // Drag ABOVE the column headers to trigger browser auto-scroll behavior
    const startX = serviceBox.x + serviceBox.width / 2;
    const startY = serviceBox.y + serviceBox.height / 2;
    const endX = userIdBox.x + userIdBox.width / 2;
    const endY = userIdBox.y - 10; // ABOVE the header to trigger auto-scroll toward top

    await page.mouse.move(startX, startY);
    await page.mouse.down();
    // Drag slowly with many steps, moving upward to trigger auto-scroll
    await page.mouse.move(endX, endY, { steps: 30 });
    // Hold above the column for 500ms to trigger auto-scroll
    await delay(500);
    await page.mouse.up();
    await delay(500);

    // Verify scroll position is preserved (strict — scroll should be actively locked during drag)
    const scrollAfter = await getScroll(page);
    const scrollDrift = Math.abs(scrollAfter - scrollBefore);
    expect(scrollDrift).toBeLessThanOrEqual(5);
  });
});
