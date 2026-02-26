/**
 * ui-prefs.spec.mjs — Persistent UI preferences (columns, widths, time preset).
 */

import { test, expect } from '@playwright/test';
import { setTimeout as delay } from 'timers/promises';
import {
  clickFieldKey,
  clickResetPreferences,
  expandCollapsedRow,
  expandRow,
  getHeaders,
  getTimePresetValue,
  portForTestFile,
  readJSONLocalStorage,
  selectTimePreset,
  startServer,
  stopServer,
} from './helpers.mjs';

let server;
let baseURL;

test.describe('ui-prefs', () => {
  test.beforeAll(async ({}, workerInfo) => {
    const port = portForTestFile(workerInfo);
    server = await startServer(port);
    baseURL = `http://localhost:${port}`;
  });

  test.afterAll(async () => {
    await stopServer(server);
  });

  test('restores pinned columns after reload', async ({ page }) => {
    await page.addInitScript(() => {
      const key = '__peek_e2e_uiprefs_pin_reset__';
      if (sessionStorage.getItem(key)) return;
      localStorage.removeItem('peek.uiPrefs.v1');
      sessionStorage.setItem(key, '1');
    });
    await page.goto(baseURL);
    await expect(page.locator('.log-row').first()).toBeVisible({ timeout: 10_000 });

    // Expand first row and pin 'service'
    await expect.poll(() => expandRow(page), { timeout: 8_000 }).toBe(true);
    await delay(300);
    await expect.poll(() => clickFieldKey(page, 'service'), { timeout: 5_000 }).toBe(true);
    await delay(300);

    // Expand a different collapsed row and pin 'user_id'
    await expect.poll(() => expandCollapsedRow(page), { timeout: 8_000 }).toBe(true);
    await delay(300);
    await expect.poll(() => clickFieldKey(page, 'user_id'), { timeout: 5_000 }).toBe(true);
    await delay(600); // wait for debounced save

    const headersBefore = await getHeaders(page);
    expect(headersBefore).toContain('service');
    expect(headersBefore).toContain('user_id');
    expect(headersBefore.indexOf('service')).toBeLessThan(headersBefore.indexOf('user_id'));

    await page.reload();
    await expect(page.locator('.log-row').first()).toBeVisible({ timeout: 10_000 });
    await delay(300);

    const headersAfter = await getHeaders(page);
    expect(headersAfter).toContain('service');
    expect(headersAfter).toContain('user_id');
    expect(headersAfter.indexOf('service')).toBeLessThan(headersAfter.indexOf('user_id'));
  });

  test('restores column widths after reload', async ({ page }) => {
    await page.addInitScript(() => {
      const key = '__peek_e2e_uiprefs_width_reset__';
      if (sessionStorage.getItem(key)) return;
      localStorage.removeItem('peek.uiPrefs.v1');
      sessionStorage.setItem(key, '1');
    });
    await page.goto(baseURL);
    await expect(page.locator('.log-row').first()).toBeVisible({ timeout: 10_000 });
    await delay(500);

    // Drag the time column resize handle to make it wider
    const handle = page.locator('.log-table-header .col-time .resize-handle');
    const box = await handle.boundingBox();
    expect(box).not.toBeNull();

    const x = box.x + box.width / 2;
    const y = box.y + box.height / 2;
    await page.mouse.move(x, y);
    await page.mouse.down();
    await page.mouse.move(x + 80, y, { steps: 15 });
    await page.mouse.up();
    await delay(600); // wait for debounced save

    const widthAfterResize = await page.evaluate(() =>
      document.querySelector('.log-table-header .col-time')?.offsetWidth ?? 0
    );
    expect(widthAfterResize).toBeGreaterThan(0);

    await page.reload();
    await expect(page.locator('.log-row').first()).toBeVisible({ timeout: 10_000 });
    await delay(300);

    const widthAfterReload = await page.evaluate(() =>
      document.querySelector('.log-table-header .col-time')?.offsetWidth ?? 0
    );
    expect(Math.abs(widthAfterReload - widthAfterResize)).toBeLessThan(10);
  });

  test('restores time preset after reload', async ({ page }) => {
    await page.addInitScript(() => {
      const key = '__peek_e2e_uiprefs_preset_reset__';
      if (sessionStorage.getItem(key)) return;
      localStorage.removeItem('peek.uiPrefs.v1');
      sessionStorage.setItem(key, '1');
    });
    await page.goto(baseURL);
    await expect(page.locator('[data-testid="time-preset"]')).toBeVisible({ timeout: 10_000 });

    await selectTimePreset(page, '7d');
    await delay(600); // wait for debounced save

    await page.reload();
    await expect(page.locator('[data-testid="time-preset"]')).toBeVisible({ timeout: 10_000 });
    await delay(300);

    const selectedValue = await getTimePresetValue(page);
    expect(selectedValue).toBe('7d');
  });

  test('restores custom time range after reload', async ({ page }) => {
    await page.addInitScript(() => {
      const key = '__peek_e2e_uiprefs_custom_reset__';
      if (sessionStorage.getItem(key)) return;
      localStorage.removeItem('peek.uiPrefs.v1');
      sessionStorage.setItem(key, '1');
    });
    await page.goto(baseURL);
    await expect(page.locator('[data-testid="time-preset"]')).toBeVisible({ timeout: 10_000 });

    await selectTimePreset(page, 'custom');
    await delay(300);

    const customStart = '2026-01-01T10:00';
    const customEnd = '2026-01-02T10:00';
    await page.fill('[data-testid="time-from"]', customStart);
    await page.fill('[data-testid="time-to"]', customEnd);
    await delay(600); // wait for debounced save

    await page.reload();
    await expect(page.locator('[data-testid="time-preset"]')).toBeVisible({ timeout: 10_000 });
    await delay(300);

    const selectedPreset = await getTimePresetValue(page);
    expect(selectedPreset).toBe('custom');

    // Custom range should be visible
    const customVisible = await page.evaluate(() => {
      const el = document.querySelector('.time-custom-row');
      return !!(el && el.style.display !== 'none');
    });
    expect(customVisible).toBeTruthy();

    const fromVal = await page.evaluate(() =>
      document.querySelector('[data-testid="time-from"]')?.value ?? ''
    );
    const toVal = await page.evaluate(() =>
      document.querySelector('[data-testid="time-to"]')?.value ?? ''
    );
    expect(fromVal).toBe(customStart);
    expect(toVal).toBe(customEnd);
  });

  test('malformed localStorage falls back safely to defaults', async ({ page }) => {
    await page.addInitScript(() => {
      localStorage.setItem('peek.uiPrefs.v1', 'not valid json {{{');
    });
    await page.goto(baseURL);
    await expect(page.locator('.log-row').first()).toBeVisible({ timeout: 10_000 });

    const selectedPreset = await getTimePresetValue(page);
    expect(selectedPreset).toBe('all');

    const headers = await getHeaders(page);
    expect(headers).toContain('time');
    expect(headers).toContain('level');
    expect(headers).toContain('message');
  });

  test('reset clears only UI prefs and preserves query history', async ({ page }) => {
    await page.addInitScript(() => {
      const key = '__peek_e2e_uiprefs_reset_seed__';
      if (sessionStorage.getItem(key)) return;
      localStorage.setItem('peek.uiPrefs.v1', JSON.stringify({
        pinnedColumns: ['service'],
        columnWidths: {},
        timeRange: { preset: '1h', customStart: null, customEnd: null },
      }));
      localStorage.setItem('peek.queryHistory.v1', JSON.stringify([
        { query: 'level:ERROR', lastUsedAt: '2026-01-01T00:00:00Z', useCount: 1 },
      ]));
      sessionStorage.setItem(key, '1');
    });
    await page.goto(baseURL);
    await expect(page.locator('[data-testid="settings-btn"]')).toBeVisible({ timeout: 10_000 });
    // Wait for the table header to appear (rows may be empty with 1h preset on old data)
    await expect(page.locator('.log-table-header')).toBeVisible({ timeout: 10_000 });

    // Verify prefs were applied: 'service' column and '1h' preset
    const headersBefore = await getHeaders(page);
    expect(headersBefore).toContain('service');

    const presetBefore = await getTimePresetValue(page);
    expect(presetBefore).toBe('1h');

    // Click reset via settings dropdown
    await clickResetPreferences(page);
    await delay(500);

    // UI prefs key should be gone (or empty)
    const uiPrefs = await page.evaluate(() => localStorage.getItem('peek.uiPrefs.v1'));
    expect(uiPrefs).toBeNull();

    // Pinned columns cleared
    const headersAfter = await getHeaders(page);
    expect(headersAfter).not.toContain('service');

    // Time preset reset to 'all'
    const presetAfter = await getTimePresetValue(page);
    expect(presetAfter).toBe('all');

    // Query history is preserved
    const history = await readJSONLocalStorage(page, 'peek.queryHistory.v1', []);
    expect(history.length).toBeGreaterThan(0);
    expect(history[0].query).toBe('level:ERROR');
  });
});
