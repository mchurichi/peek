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

test.describe('copy', () => {
  test.beforeAll(async ({}, workerInfo) => {
    const port = portForTestFile(workerInfo);
    server = await startServer(port);
    baseURL = `http://localhost:${port}`;
  });

  test.afterAll(async () => {
    await stopServer(server);
  });

  test('row copy button is visible on hover and copies raw log line', async ({ page, context }) => {
    await context.grantPermissions(['clipboard-read', 'clipboard-write']);
    await page.goto(baseURL);

    await expect.poll(
      () => page.evaluate(() => document.querySelectorAll('.log-row').length),
      { timeout: 8_000 },
    ).toBeGreaterThanOrEqual(1);

    // Copy button hidden by default
    const copyBtn = page.locator('.row-copy-btn').first();
    await expect(copyBtn).toHaveCSS('opacity', '0');

    // Hover message cell — button becomes visible
    await page.locator('.log-table-body .col-msg').first().hover();
    await expect(copyBtn).toHaveCSS('opacity', '1');

    // Click copy button (force: true because it's a hover-revealed element)
    await copyBtn.click({ force: true });

    // Toast appears with correct text
    await expect(page.locator('.toast')).toBeVisible();
    await expect(page.locator('.toast')).toContainText('Copied log line');

    // Clipboard contains non-empty content
    const clipText = await page.evaluate(() => navigator.clipboard.readText());
    expect(clipText.length).toBeGreaterThan(0);
  });

  test('field copy button copies only the value', async ({ page, context }) => {
    await context.grantPermissions(['clipboard-read', 'clipboard-write']);
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

    // Field copy button hidden by default
    const fieldCopyBtn = page.locator('.field-copy-btn').first();
    await expect(fieldCopyBtn).toHaveCSS('opacity', '0');

    // Hover the first field row — button becomes visible
    await page.locator('.fields-table tr').first().hover({ force: true });
    await expect(fieldCopyBtn).toHaveCSS('opacity', '1');

    // Capture the field value text before clicking copy.
    // field-val td has a text node as firstChild (the value) followed by the copy button.
    const fieldValue = await page.locator('.field-val').first().evaluate((el) => {
      // Text content includes the button text; strip it
      return el.childNodes[0]?.textContent?.trim() ?? '';
    });

    await fieldCopyBtn.click({ force: true });

    // Toast appears
    await expect(page.locator('.toast')).toBeVisible();
    await expect(page.locator('.toast')).toContainText('Copied value');

    // Clipboard contains the field value (not the key, not extra chars)
    const clipText = await page.evaluate(() => navigator.clipboard.readText());
    expect(clipText).toBe(fieldValue);
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
