/**
 * datetime.spec.mjs — Datetime range picker UI and API integration.
 */

import { test, expect } from '@playwright/test';
import { setTimeout as delay } from 'timers/promises';
import { portForTestFile, postJSON, startServer, stopServer, waitForFields, waitForQuery } from './helpers.mjs';

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
    expect(rawHtml).toContain('time-range-picker');
    expect(rawHtml).toContain('time-preset');
    expect(rawHtml).toContain('time-custom-range');
    expect(rawHtml).toContain('DateRangePicker');
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

    const hasPresetSelect = await page.evaluate(() =>
      !!document.querySelector('[data-testid="time-preset"]')
    );
    expect(hasPresetSelect).toBeTruthy();

    if (hasPresetSelect) {
      const optionValues = await page.evaluate(() => {
        const s = document.querySelector('[data-testid="time-preset"]');
        return Array.from(s.options).map((o) => o.value);
      });
      expect(optionValues).toEqual(expect.arrayContaining(['all', '1h', '7d', 'custom']));

      const customHidden = await page.evaluate(() => {
        const el = document.querySelector('.time-custom-range');
        return !el || el.style.display === 'none';
      });
      expect(customHidden).toBeTruthy();

      await page.evaluate(() => {
        const s = document.querySelector('[data-testid="time-preset"]');
        s.value = 'custom';
        s.dispatchEvent(new Event('change', { bubbles: true }));
      });
      await delay(400);

      const customVisible = await page.evaluate(() => {
        const el = document.querySelector('.time-custom-range');
        return !!(el && el.style.display !== 'none');
      });
      expect(customVisible).toBeTruthy();

      let capturedBody = null;
      await page.route('**/query', async (route) => {
        capturedBody = JSON.parse(route.request().postData() || '{}');
        await route.continue();
      });

      await page.evaluate(() => {
        const s = document.querySelector('[data-testid="time-preset"]');
        s.value = '1h';
        s.dispatchEvent(new Event('change', { bubbles: true }));
      });
      await delay(1000);

      expect(capturedBody?.start).not.toBeNull();
    }
  });
});
