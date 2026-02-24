/**
 * copy.spec.mjs — Copy-to-clipboard buttons for log rows and field values.
 */

import { test, expect } from '@playwright/test';
import { setTimeout as delay } from 'timers/promises';
import {
  expandRow,
  portForTestFile,
  startServer,
  stopServer,
} from './helpers.mjs';

let server;
let baseURL;

/** Install a clipboard mock that captures writeText calls into window.__clipboardWrites. */
async function mockClipboard(page) {
  await page.addInitScript(() => {
    window.__clipboardWrites = [];
    try {
      Object.defineProperty(navigator, 'clipboard', {
        value: { writeText: async (t) => { window.__clipboardWrites.push(t); } },
        configurable: true,
        writable: true,
      });
    } catch {
      navigator.clipboard = { writeText: async (t) => { window.__clipboardWrites.push(t); } };
    }
  });
}

test.describe('copy', () => {
  test.beforeAll(async ({}, workerInfo) => {
    const port = portForTestFile(workerInfo);
    server = await startServer(port);
    baseURL = `http://localhost:${port}`;
  });

  test.afterAll(async () => {
    await stopServer(server);
  });

  test('row copy button copies raw log line', async ({ page }) => {
    await mockClipboard(page);
    await page.goto(baseURL);

    await expect.poll(
      () => page.evaluate(() => document.querySelectorAll('.log-row').length),
      { timeout: 8_000 },
    ).toBeGreaterThanOrEqual(1);

    // Each row has a copy button attached (hidden by default via CSS opacity)
    await expect(page.locator('.row-copy-btn').first()).toBeAttached();

    // Trigger click via evaluate — bypasses Playwright's visibility check for opacity:0 elements
    await page.evaluate(() => document.querySelector('.row-copy-btn')?.click());

    // Toast appears with correct text
    await expect(page.locator('.toast')).toBeVisible();
    await expect(page.locator('.toast')).toContainText('Copied log line');

    // Clipboard was written with non-empty content
    const writes = await page.evaluate(() => window.__clipboardWrites);
    expect(writes.length).toBeGreaterThan(0);
    expect(writes[0].length).toBeGreaterThan(0);
  });

  test('field copy button copies only the value', async ({ page }) => {
    await mockClipboard(page);
    await page.goto(baseURL);

    await expect.poll(
      () => page.evaluate(() => document.querySelectorAll('.log-row').length),
      { timeout: 8_000 },
    ).toBeGreaterThanOrEqual(1);

    // Expand a row to reveal the fields table
    await expect.poll(() => expandRow(page), { timeout: 8_000 }).toBe(true);
    await expect.poll(
      () => page.evaluate(() => document.querySelectorAll('.field-val').length),
      { timeout: 5_000 },
    ).toBeGreaterThan(0);

    // Each field row has a copy button in its own cell (button is NOT inside .field-val)
    await expect(page.locator('.field-copy-btn').first()).toBeAttached();

    // Capture the field value text — button is in a sibling td so .field-val textContent is clean
    const fieldValue = await page.locator('.field-val').first().evaluate(
      (el) => el.textContent?.trim() ?? '',
    );
    expect(fieldValue.length).toBeGreaterThan(0);

    // Trigger click via evaluate — bypasses Playwright's visibility check for opacity:0 elements
    await page.evaluate(() => document.querySelector('.field-copy-btn')?.click());

    // Toast appears with correct text
    await expect(page.locator('.toast')).toBeVisible();
    await expect(page.locator('.toast')).toContainText('Copied value');

    // Clipboard was written with exactly the field value (no key, no extra chars)
    const writes = await page.evaluate(() => window.__clipboardWrites);
    expect(writes).toHaveLength(1);
    expect(writes[0]).toBe(fieldValue);
  });

  test('clicking field value still filters (copy button does not interfere)', async ({ page }) => {
    await page.goto(baseURL);

    await expect.poll(
      () => page.evaluate(() => document.querySelectorAll('.log-row').length),
      { timeout: 8_000 },
    ).toBeGreaterThanOrEqual(1);

    await expect.poll(() => expandRow(page), { timeout: 8_000 }).toBe(true);
    await expect.poll(
      () => page.evaluate(() => document.querySelectorAll('.field-val').length),
      { timeout: 5_000 },
    ).toBeGreaterThan(0);

    // Click the field-val td itself (not the copy button) — should set query
    await page.evaluate(() => {
      const el = document.querySelector('.field-val');
      if (el) el.click();
    });

    await delay(500);

    const query = await page.evaluate(
      () => document.querySelector('.search-editor-input')?.value ?? '',
    );
    expect(query.length).toBeGreaterThan(0);
  });
});
