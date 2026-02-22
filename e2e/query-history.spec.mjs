/**
 * query-history.spec.mjs — Query history and starred queries.
 */

import { test, expect } from '@playwright/test';
import {
  portForTestFile,
  readJSONLocalStorage,
  startServer,
  stopServer,
  waitForHistoryEntry,
} from './helpers.mjs';

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
    await page.addInitScript(() => {
      const resetKey = '__peek_e2e_history_reset_v1__';
      if (sessionStorage.getItem(resetKey)) return;
      localStorage.removeItem('peek.queryHistory.v1');
      localStorage.removeItem('peek.starredQueries.v1');
      sessionStorage.setItem(resetKey, '1');
    });
    await page.goto(baseURL);

    const searchInput = page.locator('.search-editor-input');
    const searchBtn = page.locator('button:has-text("Search")');
    await expect(searchInput).toBeVisible();
    await expect(searchBtn).toBeVisible();
    const readHistory = () => readJSONLocalStorage(page, 'peek.queryHistory.v1', []);
    const readStarred = () => readJSONLocalStorage(page, 'peek.starredQueries.v1', []);
    const executeQuery = async (query) => {
      await searchInput.fill(query);
      await searchBtn.click();
      await waitForHistoryEntry(
        page,
        query,
        (entry) => typeof entry.useCount === 'number' && entry.useCount >= 1
      );
    };

    // Build and verify history.
    await executeQuery('level:ERROR');
    await executeQuery('service:api');
    await executeQuery('level:INFO');

    let history = await readHistory();
    expect(history.length).toBe(3);
    expect(history[0]?.query).toBe('level:INFO');
    expect(history[1]?.query).toBe('service:api');
    expect(history[2]?.query).toBe('level:ERROR');

    await page.reload();
    await expect(searchInput).toBeVisible();
    await waitForHistoryEntry(page, 'level:INFO', (_entry, fullHistory) => fullHistory.length === 3);
    history = await readHistory();
    expect(history.length).toBe(3);
    expect(history[0]?.query).toBe('level:INFO');

    const beforeRepeat = history.find((h) => h.query === 'level:ERROR');
    expect(beforeRepeat).toBeTruthy();
    const beforeUseCount = beforeRepeat?.useCount ?? 0;
    const beforeTimestamp = beforeRepeat?.lastUsedAt ?? '';

    await executeQuery('level:ERROR');
    await waitForHistoryEntry(
      page,
      'level:ERROR',
      (entry) =>
        entry.useCount === beforeUseCount + 1 &&
        typeof entry.lastUsedAt === 'string' &&
        entry.lastUsedAt !== beforeTimestamp
    );
    history = await readHistory();
    expect(history[0]?.query).toBe('level:ERROR');

    // History dropdown.
    const histBtn = page.locator('[data-testid="history-btn"]');
    await expect(histBtn).toBeVisible();
    await histBtn.click();
    await expect(page.locator('.query-panel-item .qp-text').first()).toBeVisible();
    const histPanelItems = await page.locator('.query-panel-item .qp-text').allTextContents();
    expect(histPanelItems.length).toBeGreaterThan(0);
    expect(histPanelItems[0]).toBe('level:ERROR');

    await page.evaluate(() => {
      document.querySelector('.query-panel-item')?.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }));
    });
    await expect(searchInput).toHaveValue('level:ERROR');

    // Star/unstar persistence.
    const starBtn = page.locator('[data-testid="star-btn"]');
    await expect(starBtn).toBeVisible();
    await starBtn.click();
    await expect.poll(async () => {
      const starred = await readStarred();
      return starred.includes('level:ERROR');
    }).toBe(true);

    await page.reload();
    await expect(searchInput).toBeVisible();
    await searchInput.fill('level:ERROR');
    const reloadedStarClass = await page.evaluate(() =>
      document.querySelector('[data-testid="star-btn"]')?.className || ''
    );
    expect(reloadedStarClass.includes('starred')).toBeTruthy();

    await page.keyboard.press('Alt+s');
    await expect.poll(async () => {
      const starred = await readStarred();
      return starred.includes('level:ERROR');
    }).toBe(false);

    // Alt+C copy shortcut.
    await page.evaluate(() => {
      window._clipboardText = '';
      Object.defineProperty(navigator, 'clipboard', {
        get: () => ({ writeText: async (t) => { window._clipboardText = t; } }),
        configurable: true,
      });
    });
    await searchInput.fill('level:DEBUG');
    await page.keyboard.press('Alt+c');
    await expect.poll(async () => page.evaluate(() => window._clipboardText)).toBe('level:DEBUG');

    // Autocomplete still works (Enter/Tab/Escape).
    await searchInput.fill('');
    await searchInput.type('lev');
    await expect(page.locator('.search-autocomplete-item').first()).toBeVisible();
    await searchInput.press('ArrowDown');
    await searchInput.press('Enter');
    await expect(searchInput).toHaveValue(/level/);

    await searchInput.fill('');
    await searchInput.type('ser');
    await expect(page.locator('.search-autocomplete-item').first()).toBeVisible();
    await searchInput.press('Tab');
    await expect(searchInput).toHaveValue(/service:/);

    await searchInput.fill('');
    await searchInput.type('lev');
    await expect(page.locator('.search-autocomplete-item').first()).toBeVisible();
    await searchInput.press('Escape');
    await expect(page.locator('.search-autocomplete-item').first()).toBeHidden();
  });
});
