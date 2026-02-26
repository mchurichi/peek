/**
 * datetime.spec.mjs — Datetime range picker UI and API integration.
 */

import { test, expect } from '@playwright/test';
import { setTimeout as delay } from 'timers/promises';
import { getTimePresetValue, portForTestFile, postJSON, selectTimePreset, startServer, stopServer, waitForFields, waitForQuery } from './helpers.mjs';

let server;
let baseURL;

test.describe('datetime', () => {
  test.beforeAll(async ({}, workerInfo) => {
    const port = portForTestFile(workerInfo);
    server = await startServer(port);
    baseURL = `http://localhost:${port}`;
  });

  test.afterAll(async () => {
    await stopServer(server);
  });

  test('supports datetime presets and /query time filters', async ({ page }) => {
    await page.goto(baseURL);
    await expect(page.locator('.search-editor-input')).toBeVisible();
    await waitForFields(page);

    const rawHtml = await page.evaluate(() => document.documentElement.outerHTML);
    expect(rawHtml).toContain('time-range-btn');
    expect(rawHtml).toContain('time-preset');
    expect(rawHtml).toContain('time-custom-row');
    expect(rawHtml).toContain('time-custom-input');
    expect(rawHtml).toContain('getTimeRange');

    const nowISO = new Date().toISOString();
    const hourAgo = new Date(Date.now() - 60 * 60 * 1000).toISOString();

    const qResp = await waitForQuery(page, { query: '*', limit: 100, offset: 0, start: hourAgo, end: nowISO });

    expect(qResp.status).toBe(200);
    expect(Array.isArray(qResp.body.logs)).toBeTruthy();

    const futureResp = await postJSON(page, '/query', {
      query: '*', limit: 100, offset: 0,
      start: '2099-01-01T00:00:00Z',
      end: '2099-12-31T23:59:59Z',
    });
    expect(futureResp.status).toBe(200);
    expect(futureResp.body?.total).toBe(0);

    const pastResp = await postJSON(page, '/query', {
      query: '*', limit: 100, offset: 0,
      start: '2000-01-01T00:00:00Z',
      end: '2000-12-31T23:59:59Z',
    });
    expect(pastResp.status).toBe(200);
    expect(pastResp.body?.total).toBe(0);

    const allResp = await postJSON(page, '/query', { query: '*', limit: 100, offset: 0 });
    expect(allResp.status).toBe(200);
    expect(allResp.body?.total).toBeGreaterThan(0);

    const hasPresetBtn = await page.evaluate(() =>
      !!document.querySelector('[data-testid="time-preset"]')
    );
    expect(hasPresetBtn).toBeTruthy();

    if (hasPresetBtn) {
      // Verify dropdown contains expected preset values by opening it
      await page.click('[data-testid="time-preset"]');
      await page.waitForSelector('.dropdown-portal', { timeout: 3_000 });
      const itemTexts = await page.evaluate(() =>
        Array.from(document.querySelectorAll('.dropdown-portal .dp-item'))
          .map(el => el.textContent.trim())
      );
      expect(itemTexts).toEqual(expect.arrayContaining(['All time', 'Last 1 hour', 'Last 7 days', 'Custom range\u2026']));
      // Close dropdown by pressing Escape
      await page.keyboard.press('Escape');
      await delay(200);

      const customHidden = await page.evaluate(() => {
        const el = document.querySelector('.time-custom-row');
        return !el || el.style.display === 'none';
      });
      expect(customHidden).toBeTruthy();

      await selectTimePreset(page, 'custom');
      await delay(400);

      const customVisible = await page.evaluate(() => {
        const el = document.querySelector('.time-custom-row');
        return !!(el && el.style.display !== 'none');
      });
      expect(customVisible).toBeTruthy();

      let capturedBody = null;
      await page.route('**/query', async (route) => {
        capturedBody = JSON.parse(route.request().postData() || '{}');
        await route.continue();
      });

      await selectTimePreset(page, '1h');
      await delay(1000);

      expect(capturedBody?.start).not.toBeNull();
    }
  });
});
