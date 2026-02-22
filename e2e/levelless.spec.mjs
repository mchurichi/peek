/**
 * levelless.spec.mjs — Levelless log entry rendering.
 */

import { test, expect } from '@playwright/test';
import { setTimeout as delay } from 'timers/promises';
import { portForTestFile, startServer, stopServer } from './helpers.mjs';

let server;
let baseURL;

test.describe('levelless', () => {
  test.beforeAll(async ({}, workerInfo) => {
    const port = portForTestFile(workerInfo);
    const lines = Array.from({ length: 10 }, (_, idx) => {
      const i = idx + 1;
      return JSON.stringify({
        msg: `plain message ${i}`,
        time: `2026-02-18T10:${String(i).padStart(2, '0')}:00Z`,
        service: 'svc',
      });
    });

    server = await startServer(port, { lines });
    baseURL = `http://localhost:${port}`;
  });

  test.afterAll(async () => {
    await stopServer(server);
  });

  test('renders levelless rows and excludes them from level:ERROR query', async ({ page }) => {
    await page.goto(baseURL);
    await delay(2000);

    const rowCount = await page.evaluate(() =>
      document.querySelectorAll('.log-row').length
    );
    expect(rowCount).toBeGreaterThanOrEqual(10);

    const levelTexts = await page.evaluate(() =>
      Array.from(document.querySelectorAll('.col-level'))
        .map((el) => el.textContent.trim())
    );

    const hasEmDash = levelTexts.some((t) => t === '\u2014');
    expect(hasEmDash).toBeTruthy();

    const hasINFO = levelTexts.some((t) => t === 'INFO');
    expect(hasINFO).toBeFalsy();

    const noneCount = await page.evaluate(() =>
      document.querySelectorAll('.col-level.level-NONE').length
    );
    expect(noneCount).toBeGreaterThanOrEqual(10);

    const queryRes = await fetch(`${baseURL}/query`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ query: 'level:ERROR', limit: 100, offset: 0 }),
    });
    const queryData = await queryRes.json();
    expect(queryData.total).toBe(0);
  });
});
