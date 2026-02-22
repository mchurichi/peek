/**
 * query-history.spec.mjs — Query history and starred queries.
 */

import { test, expect } from '@playwright/test';
import { setTimeout as delay } from 'timers/promises';
import { portForTestFile, startServer, stopServer } from './helpers.mjs';

let server;
let baseURL;

test.describe('query-history', () => {
  test.beforeAll(async ({}, workerInfo) => {
    const port = portForTestFile(workerInfo);
    server = await startServer(port);
    baseURL = `http://localhost:${port}`;
  });

  test.afterAll(async () => {
    await stopServer(server);
  });

  test('persists history/starred queries and keeps shortcuts/autocomplete working', async ({ page }) => {
    await page.goto(baseURL);
    await delay(1500);

    await page.evaluate(() => {
      localStorage.removeItem('peek.queryHistory.v1');
      localStorage.removeItem('peek.starredQueries.v1');
    });
    await page.reload();
    await delay(1500);

    const searchInput = page.locator('.search-editor-input');

    // Build and verify history.
    await searchInput.fill('level:ERROR');
    await searchInput.press('Enter');
    await delay(800);
    await searchInput.fill('service:api');
    await searchInput.press('Enter');
    await delay(800);
    await searchInput.fill('level:INFO');
    await searchInput.press('Enter');
    await delay(800);

    let history = await page.evaluate(() => {
      try { return JSON.parse(localStorage.getItem('peek.queryHistory.v1') || '[]'); } catch { return []; }
    });
    expect(history.length).toBe(3);
    expect(history[0]?.query).toBe('level:INFO');
    expect(history[1]?.query).toBe('service:api');
    expect(history[2]?.query).toBe('level:ERROR');

    await page.reload();
    await delay(1500);
    history = await page.evaluate(() => {
      try { return JSON.parse(localStorage.getItem('peek.queryHistory.v1') || '[]'); } catch { return []; }
    });
    expect(history.length).toBe(3);
    expect(history[0]?.query).toBe('level:INFO');

    await searchInput.fill('level:ERROR');
    await searchInput.press('Enter');
    await delay(800);
    history = await page.evaluate(() => {
      try { return JSON.parse(localStorage.getItem('peek.queryHistory.v1') || '[]'); } catch { return []; }
    });
    expect(history[0]?.query).toBe('level:ERROR');
    expect(history[0]?.useCount).toBe(2);

    // History dropdown.
    const histBtn = page.locator('[data-testid="history-btn"]');
    await expect(histBtn).toBeVisible();
    await histBtn.click();
    await delay(200);

    const histPanelItems = await page.evaluate(() =>
      Array.from(document.querySelectorAll('.query-panel-item .qp-text')).map((el) => el.textContent)
    );
    expect(histPanelItems.length).toBeGreaterThan(0);
    expect(histPanelItems[0]).toBe('level:ERROR');

    await page.evaluate(() => {
      document.querySelector('.query-panel-item')?.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }));
    });
    await delay(800);
    await expect(searchInput).toHaveValue('level:ERROR');

    // Star/unstar persistence.
    const starBtn = page.locator('[data-testid="star-btn"]');
    await expect(starBtn).toBeVisible();
    await starBtn.click();
    await delay(200);

    let starred = await page.evaluate(() => {
      try { return JSON.parse(localStorage.getItem('peek.starredQueries.v1') || '[]'); } catch { return []; }
    });
    expect(starred).toContain('level:ERROR');

    await page.reload();
    await delay(1500);
    await searchInput.fill('level:ERROR');
    await delay(200);
    const reloadedStarClass = await page.evaluate(() =>
      document.querySelector('[data-testid="star-btn"]')?.className || ''
    );
    expect(reloadedStarClass.includes('starred')).toBeTruthy();

    await page.keyboard.press('Alt+s');
    await delay(200);
    starred = await page.evaluate(() => {
      try { return JSON.parse(localStorage.getItem('peek.starredQueries.v1') || '[]'); } catch { return []; }
    });
    expect(starred.includes('level:ERROR')).toBeFalsy();

    // Alt+C copy shortcut.
    await page.evaluate(() => {
      window._clipboardText = '';
      Object.defineProperty(navigator, 'clipboard', {
        get: () => ({ writeText: async (t) => { window._clipboardText = t; } }),
        configurable: true,
      });
    });
    await searchInput.fill('level:DEBUG');
    await delay(100);
    await page.keyboard.press('Alt+c');
    await delay(300);
    const copiedText = await page.evaluate(() => window._clipboardText);
    expect(copiedText).toBe('level:DEBUG');

    // Autocomplete still works (Enter/Tab/Escape).
    await searchInput.fill('');
    await searchInput.type('lev');
    await delay(300);
    await searchInput.press('ArrowDown');
    await searchInput.press('Enter');
    await delay(200);
    await expect(searchInput).toHaveValue(/level/);

    await searchInput.fill('');
    await searchInput.type('ser');
    await delay(300);
    await searchInput.press('Tab');
    await delay(100);
    await expect(searchInput).toHaveValue(/service:/);

    await searchInput.fill('');
    await searchInput.type('lev');
    await delay(300);
    await searchInput.press('Escape');
    await delay(100);
    const dropdownAfterEsc = await page.evaluate(() => {
      const d = document.querySelector('.search-autocomplete');
      return !!(d && d.style.display !== 'none');
    });
    expect(dropdownAfterEsc).toBeFalsy();
  });
});
