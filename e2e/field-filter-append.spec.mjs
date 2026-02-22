/**
 * field-filter-append.spec.mjs — clicking field values in expanded rows.
 */

import { test, expect } from '@playwright/test';
import { setTimeout as delay } from 'timers/promises';
import {
  expandRow,
  getScroll,
  portForTestFile,
  setScroll,
  startServer,
  stopServer,
} from './helpers.mjs';

let server;
let baseURL;

test.describe('field-filter-append', () => {
  test.beforeAll(async ({}, workerInfo) => {
    const port = portForTestFile(workerInfo);
    server = await startServer(port);
    baseURL = `http://localhost:${port}`;
  });

  test.afterAll(async () => {
    await stopServer(server);
  });

  test('appends safe field tokens and preserves scroll', async ({ page }) => {
    const clickFieldVal = async (value) => page.evaluate((v) => {
      const el = Array.from(document.querySelectorAll('.field-val'))
        .find((e) => e.textContent.trim() === v);
      if (!el) return false;
      el.click();
      return true;
    }, value);

    const getQuery = async () => page.evaluate(() =>
      document.querySelector('.search-editor-input')?.value ?? ''
    );

    const setQuery = async (q) => {
      await page.evaluate((query) => {
        const el = document.querySelector('.search-editor-input');
        if (!el) return;
        el.value = query;
        el.dispatchEvent(new Event('input', { bubbles: true }));
      }, q);
    };

    const expandAndWait = async () => {
      await expect.poll(async () => expandRow(page), {
        timeout: 8_000,
        intervals: [100, 200, 300, 500],
      }).toBe(true);
      await expect.poll(async () => page.evaluate(() =>
        document.querySelectorAll('.field-val').length
      ), {
        timeout: 5_000,
        intervals: [100, 200, 300],
      }).toBeGreaterThan(0);
    };

    const expectQueryValue = async (value) => {
      await expect.poll(async () => getQuery(), {
        timeout: 5_000,
        intervals: [100, 200, 300],
      }).toBe(value);
    };

    await page.goto(baseURL);
    await expect(page.locator('.search-editor-input')).toBeVisible();

    await expect.poll(async () => page.evaluate(() => document.querySelectorAll('.log-row').length), {
      timeout: 8_000,
      intervals: [100, 200, 300, 500],
    }).toBeGreaterThanOrEqual(30);

    await setQuery('level:INFO');
    await expandAndWait();
    expect(await clickFieldVal('api')).toBeTruthy();
    await expectQueryValue('level:INFO AND service:"api"');

    await setQuery('');
    await expandAndWait();
    expect(await clickFieldVal('api')).toBeTruthy();
    await expectQueryValue('service:"api"');

    await setQuery('*');
    await expandAndWait();
    expect(await clickFieldVal('api')).toBeTruthy();
    await expectQueryValue('service:"api"');

    await setQuery('level:INFO AND');
    await expandAndWait();
    expect(await clickFieldVal('api')).toBeTruthy();
    await expectQueryValue('level:INFO AND service:"api"');

    await setScroll(page, 300);
    await delay(200);
    const scrollBefore = await getScroll(page);
    expect(scrollBefore).toBeGreaterThan(0);

    await expandAndWait();
    const scrollAfterExpand = await getScroll(page);
    expect(await clickFieldVal('api')).toBeTruthy();

    const scrollAfterClick = await getScroll(page);
    const drift = Math.abs(scrollAfterClick - scrollAfterExpand);
    expect(drift).toBeLessThanOrEqual(50);
  });
});
