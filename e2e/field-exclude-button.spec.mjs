/**
 * field-exclude-button.spec.mjs — quick exclude ('-') button on field values.
 *
 * Each field value in the expanded log detail panel has an exclude button that
 * appends `AND NOT field:"value"` (or `NOT field:"value"` when the query is
 * empty) to the search query and re-executes it.
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

test.describe('field-exclude-button', () => {
  test.beforeAll(async ({}, workerInfo) => {
    const port = portForTestFile(workerInfo);
    server = await startServer(port);
    baseURL = `http://localhost:${port}`;
  });

  test.afterAll(async () => {
    await stopServer(server);
  });

  /** Click the exclude button for the field-val-cell whose text matches `value`. */
  const clickExcludeForVal = async (page, value) => page.evaluate((v) => {
    const cells = document.querySelectorAll('.field-val-cell');
    for (const cell of cells) {
      const valEl = cell.querySelector('.field-val');
      if (valEl && valEl.textContent.trim() === v) {
        const btn = cell.querySelector('.field-exclude-btn');
        if (!btn) return false;
        btn.click();
        return true;
      }
    }
    return false;
  }, value);

  const getQuery = async (page) => page.evaluate(() =>
    document.querySelector('.search-editor-input')?.value ?? ''
  );

  const setQuery = async (page, q) => {
    await page.evaluate((query) => {
      const el = document.querySelector('.search-editor-input');
      if (!el) return;
      el.value = query;
      el.dispatchEvent(new Event('input', { bubbles: true }));
    }, q);
  };

  const expandAndWait = async (page) => {
    await expect.poll(async () => expandRow(page), {
      timeout: 8_000,
      intervals: [100, 200, 300, 500],
    }).toBe(true);
    await expect.poll(async () => page.evaluate(() =>
      document.querySelectorAll('.field-exclude-btn').length
    ), {
      timeout: 5_000,
      intervals: [100, 200, 300],
    }).toBeGreaterThan(0);
  };

  const expectQuery = async (page, value) => {
    await expect.poll(async () => getQuery(page), {
      timeout: 5_000,
      intervals: [100, 200, 300],
    }).toBe(value);
  };

  test('exclude from empty query produces NOT token', async ({ page }) => {
    await page.goto(baseURL);
    await expect(page.locator('.search-editor-input')).toBeVisible();
    await expect.poll(
      async () => page.evaluate(() => document.querySelectorAll('.log-row').length),
      { timeout: 8_000, intervals: [100, 200, 300, 500] }
    ).toBeGreaterThanOrEqual(30);

    await setQuery(page, '');
    await expandAndWait(page);
    expect(await clickExcludeForVal(page, 'api')).toBe(true);
    await expectQuery(page, 'NOT service:"api"');
  });

  test('exclude from wildcard query replaces * with NOT token', async ({ page }) => {
    await page.goto(baseURL);
    await expect(page.locator('.search-editor-input')).toBeVisible();
    await expect.poll(
      async () => page.evaluate(() => document.querySelectorAll('.log-row').length),
      { timeout: 8_000, intervals: [100, 200, 300, 500] }
    ).toBeGreaterThanOrEqual(30);

    await setQuery(page, '*');
    await expandAndWait(page);
    expect(await clickExcludeForVal(page, 'api')).toBe(true);
    await expectQuery(page, 'NOT service:"api"');
  });

  test('exclude appends AND NOT to existing query', async ({ page }) => {
    await page.goto(baseURL);
    await expect(page.locator('.search-editor-input')).toBeVisible();
    await expect.poll(
      async () => page.evaluate(() => document.querySelectorAll('.log-row').length),
      { timeout: 8_000, intervals: [100, 200, 300, 500] }
    ).toBeGreaterThanOrEqual(30);

    await setQuery(page, 'level:INFO');
    await expandAndWait(page);
    expect(await clickExcludeForVal(page, 'api')).toBe(true);
    await expectQuery(page, 'level:INFO AND NOT service:"api"');
  });

  test('exclude preserves scroll position', async ({ page }) => {
    await page.goto(baseURL);
    await expect(page.locator('.search-editor-input')).toBeVisible();
    await expect.poll(
      async () => page.evaluate(() => document.querySelectorAll('.log-row').length),
      { timeout: 8_000, intervals: [100, 200, 300, 500] }
    ).toBeGreaterThanOrEqual(30);

    await setScroll(page, 300);
    await delay(200);
    const scrollBefore = await getScroll(page);
    expect(scrollBefore).toBeGreaterThan(0);

    await expandAndWait(page);

    // Read a unique field value (user_id) from the expanded row's detail grid.
    // The grid alternates: keyEl at even indices, valCell at odd indices.
    // Excluding a unique user_id leaves 39 rows so scroll can be preserved.
    const uniqueVal = await page.evaluate(() => {
      const firstCell = document.querySelector('.field-val-cell');
      const grid = firstCell?.parentElement;
      if (!grid) return null;
      const children = Array.from(grid.children);
      for (let i = 1; i < children.length; i += 2) {
        if (children[i - 1]?.textContent.trim() === 'user_id') {
          return children[i].querySelector('.field-val')?.textContent.trim() ?? null;
        }
      }
      return null;
    });
    expect(uniqueVal).toBeTruthy();

    const scrollAfterExpand = await getScroll(page);
    expect(await clickExcludeForVal(page, uniqueVal)).toBe(true);

    const scrollAfterClick = await getScroll(page);
    const drift = Math.abs(scrollAfterClick - scrollAfterExpand);
    expect(drift).toBeLessThanOrEqual(50);
  });

  test('exclude button is present for every field value in expanded row', async ({ page }) => {
    await page.goto(baseURL);
    await expect(page.locator('.search-editor-input')).toBeVisible();
    await expect.poll(
      async () => page.evaluate(() => document.querySelectorAll('.log-row').length),
      { timeout: 8_000, intervals: [100, 200, 300, 500] }
    ).toBeGreaterThanOrEqual(30);

    await expandAndWait(page);

    const result = await page.evaluate(() => {
      const cells = document.querySelectorAll('.field-val-cell');
      let total = 0;
      let withBtn = 0;
      for (const cell of cells) {
        total++;
        if (cell.querySelector('.field-exclude-btn')) withBtn++;
      }
      return { total, withBtn };
    });

    expect(result.total).toBeGreaterThan(0);
    expect(result.withBtn).toBe(result.total);
  });
});
