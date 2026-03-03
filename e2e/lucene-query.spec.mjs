/**
 * lucene-query.spec.mjs — Lucene-style query feature tests.
 *
 * Covers: field existence (field:*), regex (field:/regex/), FTS (bare keyword
 * and quoted phrase), required/prohibited (+/-), and wildcard queries.
 */

import { test, expect } from '@playwright/test';
import { portForTestFile, startServer, stopServer, postJSON } from './helpers.mjs';

let server;
let baseURL;

/** Logs with varied fields and messages for query testing. */
const TEST_LINES = [
  // logs with request_id
  JSON.stringify({ level: 'INFO',  msg: 'connection timeout',  time: '2026-02-18T10:01:00Z', service: 'api-gateway',  request_id: 'req-001' }),
  JSON.stringify({ level: 'INFO',  msg: 'connection refused',  time: '2026-02-18T10:02:00Z', service: 'api-edge',     request_id: 'req-002' }),
  // logs without request_id
  JSON.stringify({ level: 'WARN',  msg: 'all good',            time: '2026-02-18T10:03:00Z', service: 'auth-service' }),
  JSON.stringify({ level: 'ERROR', msg: 'internal error',      time: '2026-02-18T10:04:00Z', service: 'auth-service' }),
  JSON.stringify({ level: 'ERROR', msg: 'gateway error',       time: '2026-02-18T10:05:00Z', service: 'api-gateway',  request_id: 'req-005' }),
  JSON.stringify({ level: 'DEBUG', msg: 'debug trace',         time: '2026-02-18T10:06:00Z', service: 'api-edge' }),
];

test.describe('lucene-query', () => {
  test.beforeAll(async ({}, workerInfo) => {
    const port = portForTestFile(workerInfo);
    server = await startServer(port, { lines: TEST_LINES });
    baseURL = `http://localhost:${port}`;
  });

  test.afterAll(async () => {
    await stopServer(server);
  });

  test('field existence: request_id:* returns only logs with that field', async ({ page }) => {
    await page.goto(baseURL);
    const result = await postJSON(page, '/query', { query: 'request_id:*', limit: 100 });
    expect(result.status).toBe(200);
    const logs = result.body.logs;
    expect(logs.length).toBeGreaterThan(0);
    // Every returned log must have request_id
    for (const log of logs) {
      expect(log.fields).toHaveProperty('request_id');
    }
    // Logs without request_id must not appear
    const withoutRequestId = logs.filter((l) => !l.fields?.request_id);
    expect(withoutRequestId).toHaveLength(0);
  });

  test('field existence: non-existent field returns no results', async ({ page }) => {
    await page.goto(baseURL);
    const result = await postJSON(page, '/query', { query: 'nonexistent_field:*', limit: 100 });
    expect(result.status).toBe(200);
    expect(result.body.logs).toHaveLength(0);
  });

  test('regex: service:/^api-(gateway|edge)$/ matches only api services', async ({ page }) => {
    await page.goto(baseURL);
    const result = await postJSON(page, '/query', {
      query: 'service:/^api-(gateway|edge)$/',
      limit: 100,
    });
    expect(result.status).toBe(200);
    const logs = result.body.logs;
    expect(logs.length).toBeGreaterThan(0);
    for (const log of logs) {
      expect(['api-gateway', 'api-edge']).toContain(log.fields?.service);
    }
    // auth-service must not appear
    const authLogs = logs.filter((l) => l.fields?.service === 'auth-service');
    expect(authLogs).toHaveLength(0);
  });

  test('FTS: bare keyword "timeout" matches message containing word', async ({ page }) => {
    await page.goto(baseURL);
    const result = await postJSON(page, '/query', { query: 'timeout', limit: 100 });
    expect(result.status).toBe(200);
    const logs = result.body.logs;
    expect(logs.length).toBeGreaterThan(0);
    for (const log of logs) {
      expect(log.message.toLowerCase()).toContain('timeout');
    }
  });

  test('FTS: quoted phrase "connection refused" matches the exact phrase', async ({ page }) => {
    await page.goto(baseURL);
    const result = await postJSON(page, '/query', {
      query: '"connection refused"',
      limit: 100,
    });
    expect(result.status).toBe(200);
    const logs = result.body.logs;
    expect(logs.length).toBeGreaterThan(0);
    for (const log of logs) {
      expect(log.message.toLowerCase()).toContain('connection refused');
    }
    // "connection timeout" must NOT appear
    const wrongLogs = logs.filter((l) => l.message.toLowerCase().includes('timeout') && !l.message.toLowerCase().includes('refused'));
    expect(wrongLogs).toHaveLength(0);
  });

  test('required/prohibited: +level:ERROR -service:auth returns only non-auth ERRORs', async ({ page }) => {
    await page.goto(baseURL);
    const result = await postJSON(page, '/query', {
      query: '+level:ERROR -service:auth',
      limit: 100,
    });
    expect(result.status).toBe(200);
    const logs = result.body.logs;
    expect(logs.length).toBeGreaterThan(0);
    for (const log of logs) {
      expect(log.level).toBe('ERROR');
      expect(log.fields?.service).not.toContain('auth');
    }
  });

  test('wildcard: service:api* matches all api-prefixed services', async ({ page }) => {
    await page.goto(baseURL);
    const result = await postJSON(page, '/query', { query: 'service:api*', limit: 100 });
    expect(result.status).toBe(200);
    const logs = result.body.logs;
    expect(logs.length).toBeGreaterThan(0);
    for (const log of logs) {
      expect(log.fields?.service).toMatch(/^api/);
    }
    // auth-service must not appear
    const authLogs = logs.filter((l) => l.fields?.service === 'auth-service');
    expect(authLogs).toHaveLength(0);
  });

  test('UI: regex and +/- queries highlighted correctly', async ({ page }) => {
    await page.goto(baseURL);
    const searchInput = page.locator('.search-editor-input');
    await expect(searchInput).toBeVisible();

    // Regex literal should get hl-regex class
    await searchInput.fill('service:/^api-(gateway|edge)$/');
    await page.waitForTimeout(100);
    const hasRegexHighlight = await page.evaluate(
      () => !!document.querySelector('.search-highlight .hl-regex'),
    );
    expect(hasRegexHighlight).toBeTruthy();

    // +/- prefix operators should get hl-op class
    await searchInput.fill('+level:ERROR -service:auth');
    await page.waitForTimeout(100);
    const opSpans = await page.evaluate(
      () => Array.from(document.querySelectorAll('.search-highlight .hl-op')).map((e) => e.textContent),
    );
    expect(opSpans).toContain('+');
    expect(opSpans).toContain('-');

    // ? wildcard should get hl-wildcard class
    await searchInput.fill('service:api-?');
    await page.waitForTimeout(100);
    const hasWildcardHighlight = await page.evaluate(
      () => !!document.querySelector('.search-highlight .hl-wildcard'),
    );
    expect(hasWildcardHighlight).toBeTruthy();
  });
});
