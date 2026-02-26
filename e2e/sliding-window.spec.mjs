/**
 * sliding-window.spec.mjs — Relative time presets slide without /query polling.
 */

import { test, expect } from '@playwright/test';
import { setTimeout as delay } from 'timers/promises';
import { portForTestFile, selectTimePreset, startServer, stopServer } from './helpers.mjs';

let server;
let baseURL;

test.describe('sliding-window', () => {
  test.beforeAll(async ({}, workerInfo) => {
    const port = portForTestFile(workerInfo);
    server = await startServer(port);
    baseURL = `http://localhost:${port}`;
  });

  test.afterAll(async () => {
    await stopServer(server);
  });

  test('slides relative windows without polling and keeps fixed window static', async ({ page }) => {
    await page.addInitScript(() => {
      const realNow = Date.now.bind(Date);
      window.__peekNowOffsetMs = 0;
      Date.now = () => realNow() + (window.__peekNowOffsetMs || 0);

      class FakeWebSocket {
        static OPEN = 1;
        static CLOSED = 3;

        constructor() {
          this.readyState = FakeWebSocket.OPEN;
          setTimeout(() => {
            if (typeof this.onopen === 'function') this.onopen();
          }, 0);
        }

        send() {}

        close() {
          this.readyState = FakeWebSocket.CLOSED;
          if (typeof this.onclose === 'function') this.onclose();
        }
      }

      window.WebSocket = FakeWebSocket;
    });

    let queryCount = 0;
    await page.route('**/query', async (route) => {
      queryCount++;
      const now = Date.now();
      const logs = [
        {
          id: `sw-1-${queryCount}`,
          timestamp: new Date(now - 10 * 60 * 1000).toISOString(),
          level: 'INFO',
          message: 'within-window',
          fields: { service: 'api' },
          raw: '{"msg":"within-window"}',
        },
        {
          id: `sw-2-${queryCount}`,
          timestamp: new Date(now - 14 * 60 * 1000).toISOString(),
          level: 'WARN',
          message: 'near-edge',
          fields: { service: 'api' },
          raw: '{"msg":"near-edge"}',
        },
      ];

      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ logs, total: logs.length, took_ms: 1 }),
      });
    });

    await page.goto(baseURL);
    await page.waitForSelector('[data-testid="time-preset"]', { timeout: 15_000 });
    await delay(400);

    const baselineCount = queryCount;

    await selectTimePreset(page, '15m');
    await delay(700);

    const beforeRows = await page.evaluate(() => document.querySelectorAll('.log-row').length);
    expect(beforeRows).toBeGreaterThanOrEqual(2);

    await page.evaluate(() => { window.__peekNowOffsetMs = 20 * 60 * 1000; });
    await delay(1400);

    const afterRows = await page.evaluate(() => document.querySelectorAll('.log-row').length);
    expect(afterRows).toBe(0);

    const afterSelectCount = queryCount;
    await delay(6100);
    expect(queryCount).toBe(afterSelectCount);

    await page.evaluate(() => { window.__peekNowOffsetMs = 0; });
    await selectTimePreset(page, 'today');
    await delay(700);

    const todayRowsBefore = await page.evaluate(() => document.querySelectorAll('.log-row').length);
    expect(todayRowsBefore).toBeGreaterThanOrEqual(2);

    await page.evaluate(() => { window.__peekNowOffsetMs = 20 * 60 * 1000; });
    await delay(1400);

    const todayRowsAfter = await page.evaluate(() => document.querySelectorAll('.log-row').length);
    expect(todayRowsAfter).toBe(todayRowsBefore);

    expect(queryCount).toBeGreaterThan(baselineCount);
  });
});
