/**
 * search.spec.mjs — Lucene syntax highlighting and field autocompletion.
 */

import { test, expect } from '@playwright/test';
import { setTimeout as delay } from 'timers/promises';
import { portForTestFile, startServer, stopServer } from './helpers.mjs';

let server;
let baseURL;

test.describe('search', () => {
  test.beforeAll(async ({}, workerInfo) => {
    const port = portForTestFile(workerInfo);
    server = await startServer(port);
    baseURL = `http://localhost:${port}`;
  });

  test.afterAll(async () => {
    await stopServer(server);
  });

  test('supports field API, highlighting, autocomplete, and query errors', async ({ page }) => {
    await page.goto(baseURL);
    await delay(1000);

    const fieldsResp = await page.evaluate(() => fetch('/fields').then((r) => r.json()));
    const fieldNames = (fieldsResp.fields || []).map((f) => f.name);
    expect(Array.isArray(fieldsResp.fields)).toBeTruthy();
    expect(fieldNames).toEqual(expect.arrayContaining(['service', 'user_id', 'request_id', 'level', 'message', 'timestamp']));

    const serviceField = (fieldsResp.fields || []).find((f) => f.name === 'service');
    expect(Array.isArray(serviceField?.top_values)).toBeTruthy();
    expect(serviceField?.top_values).toContain('api');

    await delay(2000);
    const hlExists = await page.evaluate(() => !!document.querySelector('.search-highlight'));
    const inputExists = await page.evaluate(() => !!document.querySelector('.search-editor-input'));
    expect(hlExists).toBeTruthy();
    expect(inputExists).toBeTruthy();

    const searchInput = page.locator('.search-editor-input');

    await searchInput.fill('level:ERROR');
    await delay(100);
    expect(await page.evaluate(() => !!document.querySelector('.search-highlight .hl-field'))).toBeTruthy();

    await searchInput.fill('level:ERROR AND service:api');
    await delay(100);
    expect(await page.evaluate(() => !!document.querySelector('.search-highlight .hl-op'))).toBeTruthy();

    await searchInput.fill('"connection refused"');
    await delay(100);
    expect(await page.evaluate(() => !!document.querySelector('.search-highlight .hl-quote'))).toBeTruthy();

    await searchInput.fill('message:*timeout*');
    await delay(100);
    expect(await page.evaluate(() => !!document.querySelector('.search-highlight .hl-wildcard'))).toBeTruthy();

    await searchInput.fill('[now-1h TO now]');
    await delay(100);
    expect(await page.evaluate(() => !!document.querySelector('.search-highlight .hl-range'))).toBeTruthy();

    await searchInput.fill('(level:ERROR');
    await delay(100);
    expect(await page.evaluate(() => !!document.querySelector('.search-highlight .hl-error'))).toBeTruthy();

    await searchInput.fill('');
    for (const ch of 'level:INFO') {
      await searchInput.type(ch);
      await delay(20);
    }
    await delay(100);
    const syncedField = await page.evaluate(() => document.querySelector('.search-highlight .hl-field')?.textContent);
    expect(syncedField).toBe('level');

    await searchInput.fill('');
    await searchInput.type('ser');
    await delay(300);

    const dropdownVisible = await page.evaluate(() => {
      const d = document.querySelector('.search-autocomplete');
      return !!(d && d.style.display !== 'none' && d.children.length > 0);
    });
    expect(dropdownVisible).toBeTruthy();

    const hasService = await page.evaluate(() =>
      Array.from(document.querySelectorAll('.search-autocomplete-item'))
        .some((el) => el.textContent.includes('service'))
    );
    expect(hasService).toBeTruthy();

    await searchInput.press('ArrowDown');
    await searchInput.press('Enter');
    await delay(150);
    const inputAfterEnter = await searchInput.inputValue();
    expect(inputAfterEnter).toContain('service:');

    const dropdownAfterEnter = await page.evaluate(() => {
      const d = document.querySelector('.search-autocomplete');
      return !!(d && d.style.display !== 'none');
    });
    expect(dropdownAfterEnter).toBeFalsy();

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
    expect(await searchInput.inputValue()).toBe('lev');

    await searchInput.fill('');
    await searchInput.focus();
    await searchInput.type('l');
    await delay(300);
    const hasLevel = await page.evaluate(() =>
      Array.from(document.querySelectorAll('.search-autocomplete-item'))
        .some((el) => el.textContent.includes('level'))
    );
    expect(hasLevel).toBeTruthy();

    await searchInput.fill('');
    await searchInput.type('level:');
    await delay(300);

    const valueDropdown = await page.evaluate(() => {
      const d = document.querySelector('.search-autocomplete');
      return !!(d && d.style.display !== 'none' && d.children.length > 0);
    });
    expect(valueDropdown).toBeTruthy();

    const hasINFO = await page.evaluate(() =>
      Array.from(document.querySelectorAll('.search-autocomplete-item'))
        .some((el) => el.textContent === 'INFO')
    );
    expect(hasINFO).toBeTruthy();

    await searchInput.press('ArrowDown');
    await searchInput.press('Enter');
    await delay(150);
    expect(await searchInput.inputValue()).toBe('level:INFO');

    await searchInput.fill('');
    await searchInput.type('service:');
    await delay(300);
    const hasApiValue = await page.evaluate(() =>
      Array.from(document.querySelectorAll('.search-autocomplete-item'))
        .some((el) => el.textContent === 'api')
    );
    expect(hasApiValue).toBeTruthy();

    await searchInput.fill('');
    await searchInput.type('level:');
    await delay(300);
    await searchInput.press('ArrowDown');
    await searchInput.press('Enter');
    await delay(150);
    await searchInput.press('Enter');
    await delay(1500);

    const rowCount = await page.evaluate(() => document.querySelectorAll('.log-row').length);
    expect(rowCount).toBeGreaterThan(0);

    await searchInput.fill('');
    await page.evaluate(() => {
      const inp = document.querySelector('.search-editor-input');
      if (inp) inp.dispatchEvent(new Event('input'));
    });
    await delay(100);
    await searchInput.type('message:*timeout*');
    await delay(150);

    const freshField = await page.evaluate(() => !!document.querySelector('.search-highlight .hl-field'));
    expect(freshField).toBeTruthy();

    await searchInput.fill('level:ERROR AND');
    await searchInput.press('Enter');
    await delay(500);

    const statusText = await page.evaluate(() =>
      document.querySelector('.status')?.textContent?.trim() || ''
    );
    expect(statusText.toLowerCase()).toContain('query');

    const clearedRows = await page.evaluate(() => document.querySelectorAll('.log-row').length);
    expect(clearedRows).toBe(0);

    const emptyMessage = await page.evaluate(() =>
      document.querySelector('.log-table-body > div')?.textContent?.trim() || ''
    );
    expect(emptyMessage).toContain(statusText);
  });
});
