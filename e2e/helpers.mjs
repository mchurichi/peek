/**
 * Shared test helpers for Playwright Test based e2e tests.
 */

import { spawn } from 'child_process';
import { once } from 'events';
import { setTimeout as delay } from 'timers/promises';
import { basename, dirname, resolve } from 'path';
import { fileURLToPath } from 'url';
import { expect } from '@playwright/test';

const __dirname = dirname(fileURLToPath(import.meta.url));
const PROJECT_ROOT = resolve(__dirname, '..');

const STARTUP_TIMEOUT_MS = 30_000;
const POLL_INTERVAL_MS = 1_000;
const FILE_PORT_OFFSETS = Object.freeze({
  'copy.spec.mjs': 8,
  'datetime.spec.mjs': 0,
  'field-filter-append.spec.mjs': 1,
  'levelless.spec.mjs': 2,
  'resize.spec.mjs': 3,
  'search-caret.spec.mjs': 4,
  'search.spec.mjs': 5,
  'sliding-window.spec.mjs': 6,
  'table.spec.mjs': 7,
  'ui-prefs.spec.mjs': 9,
});

function hashOffset(fileName) {
  let h = 0;
  for (const ch of fileName) {
    h = (h * 31 + ch.charCodeAt(0)) % 90;
  }
  return h;
}

export function portForTestFile(testInfo) {
  const basePort = Number(process.env.PEEK_E2E_BASE_PORT || 9997);
  const workerIndex = testInfo?.workerIndex ?? 0;
  const fileName = basename(testInfo?.file || 'unknown.spec.mjs');
  const fileOffset = FILE_PORT_OFFSETS[fileName] ?? hashOffset(fileName);
  return basePort + workerIndex * 100 + fileOffset;
}

export async function startServer(port, { rows = 40, lines = null } = {}) {
  const testDbPath = `/tmp/peek-e2e-test-${port}`;
  const testBinPath = `/tmp/peek-e2e-bin-${port}`;
  const testLogPath = `/tmp/peek-e2e-${port}.log`;

  let inputCmd = '';
  if (Array.isArray(lines)) {
    const payload = Buffer.from(lines.join('\n'), 'utf8').toString('base64');
    inputCmd = `printf %s '${payload}' | base64 --decode`;
  } else {
    inputCmd = `for i in $(seq 1 ${rows}); do
      printf '{"level":"INFO","msg":"Message %d","time":"2026-02-18T10:%02d:00Z","service":"api","user_id":"user%d","request_id":"req-%d"}\\n' "$i" "$i" "$i" "$i"
    done`;
  }

  const cmd = `
    rm -rf ${testDbPath} ${testBinPath} ${testLogPath} && \
    (command -v go >/dev/null 2>&1 && go build -o ${testBinPath} ./cmd/peek || mise exec -- go build -o ${testBinPath} ./cmd/peek) && \
    ${inputCmd} | ${testBinPath} --port ${port} --no-browser --db-path ${testDbPath} --all --retention-days 0 --retention-size 10GB > ${testLogPath} 2>&1
  `;

  const proc = spawn('sh', ['-c', cmd], { cwd: PROJECT_ROOT });

  const deadline = Date.now() + STARTUP_TIMEOUT_MS;
  while (Date.now() < deadline) {
    try {
      const resp = await fetch(`http://localhost:${port}/health`);
      if (resp.ok) {
        return proc;
      }
    } catch {
      // Server not ready yet.
    }
    await delay(POLL_INTERVAL_MS);
  }

  await stopServer(proc);
  throw new Error(`peek server on port ${port} did not start within ${STARTUP_TIMEOUT_MS / 1000}s`);
}

export async function stopServer(proc) {
  if (!proc || proc.exitCode !== null) {
    return;
  }

  proc.kill('SIGTERM');
  await Promise.race([
    once(proc, 'exit').catch(() => {}),
    delay(1_000),
  ]);

  if (proc.exitCode === null) {
    proc.kill('SIGKILL');
    await Promise.race([
      once(proc, 'exit').catch(() => {}),
      delay(1_000),
    ]);
  }
}

/** Read scrollTop of the log container. */
export async function getScroll(page) {
  return page.evaluate(() =>
    document.querySelector('.log-container')?.scrollTop ?? 0
  );
}

/** Set scrollTop and wait for the repaint to settle. */
export async function setScroll(page, pos) {
  await page.evaluate((p) => {
    const c = document.querySelector('.log-container');
    if (c) c.scrollTop = p;
  }, pos);
  await delay(200);
}

/**
 * Click a `.field-key` whose visible text matches `name`.
 * Returns `true` if the element was found and clicked.
 */
export async function clickFieldKey(page, name) {
  return page.evaluate((n) => {
    const el = Array.from(document.querySelectorAll('.field-key'))
      .find((e) => e.textContent.trim() === n);
    if (!el) return false;
    el.click();
    return true;
  }, name);
}

/**
 * Click the first in-viewport chevron that has no expanded detail row.
 * Returns `true` if a chevron was clicked.
 */
export async function expandRow(page) {
  return page.evaluate(() => {
    const chevrons = document.querySelectorAll('.log-table-body .col-chevron');
    for (const c of chevrons) {
      const r = c.getBoundingClientRect();
      if (r.top > 50 && r.bottom < window.innerHeight) {
        c.click();
        return true;
      }
    }
    chevrons[0]?.click();
    return chevrons.length > 0;
  });
}

/**
 * Expand a row that is NOT already expanded (for adding a second column
 * from a different detail panel).
 */
export async function expandCollapsedRow(page) {
  return page.evaluate(() => {
    const chevrons = document.querySelectorAll('.col-chevron');
    for (const c of chevrons) {
      const detail = c.parentElement?.nextElementSibling;
      if (detail && !detail.classList.contains('visible')) {
        const r = c.getBoundingClientRect();
        if (r.top > 0 && r.bottom < window.innerHeight) {
          c.click();
          return true;
        }
      }
    }
    return false;
  });
}

/** Return visible header labels (strip remove-button chars). */
export async function getHeaders(page) {
  return page.evaluate(() =>
    Array.from(document.querySelectorAll('.log-table-header > div'))
      .map((d) => d.textContent.replace(/[✕×]/g, '').trim())
      .filter((t) => t.length > 0)
  );
}

export async function readJSONLocalStorage(page, key, fallback = []) {
  return page.evaluate(([storageKey, defaultValue]) => {
    try {
      return JSON.parse(localStorage.getItem(storageKey) || JSON.stringify(defaultValue));
    } catch {
      return defaultValue;
    }
  }, [key, fallback]);
}

export async function waitForHistoryEntry(page, query, predicate, { timeout = 5_000 } = {}) {
  await expect.poll(async () => {
    const history = await readJSONLocalStorage(page, 'peek.queryHistory.v1', []);
    const entry = history.find((item) => item.query === query);
    return Boolean(entry && predicate(entry, history));
  }, { timeout }).toBe(true);
}

export async function waitForFields(page, { timeout = 10_000, interval = 200 } = {}) {
  const deadline = Date.now() + timeout;
  let lastError = null;
  const fieldsURL = new URL('/fields', page.url()).toString();

  while (Date.now() < deadline) {
    try {
      const resp = await page.request.get(fieldsURL);
      if (!resp.ok()) throw new Error(`fields status ${resp.status()}`);
      const data = await resp.json();
      if (Array.isArray(data?.fields)) return data;
      lastError = new Error('fields payload missing array');
    } catch (err) {
      lastError = err;
    }
    await delay(interval);
  }

  throw new Error(`Timed out waiting for /fields: ${lastError?.message || 'unknown error'}`);
}

export async function postJSON(page, path, payload) {
  const url = new URL(path, page.url()).toString();
  const resp = await page.request.post(url, { data: payload });
  let body = null;
  try {
    body = await resp.json();
  } catch {
    body = null;
  }
  return { status: resp.status(), body };
}

export async function waitForQuery(page, payload, { timeout = 10_000, interval = 200 } = {}) {
  const deadline = Date.now() + timeout;
  let lastError = null;

  while (Date.now() < deadline) {
    try {
      const result = await postJSON(page, '/query', payload);
      if (result.status === 200 && result.body && Array.isArray(result.body.logs)) return result;
      lastError = new Error(`query status ${result.status}`);
    } catch (err) {
      lastError = err;
    }
    await delay(interval);
  }

  throw new Error(`Timed out waiting for /query: ${lastError?.message || 'unknown error'}`);
}

/**
 * Map of preset values to their display labels in the time-range dropdown.
 */
const PRESET_LABELS = {
  all: 'All time',
  '15m': 'Last 15 minutes',
  '1h': 'Last 1 hour',
  '6h': 'Last 6 hours',
  '24h': 'Last 24 hours',
  '7d': 'Last 7 days',
  today: 'Today',
  yesterday: 'Yesterday',
  custom: 'Custom range\u2026',
};

/**
 * Select a time preset by opening the dropdown portal and clicking the item.
 * @param {import('@playwright/test').Page} page
 * @param {string} value - preset value key (e.g. '1h', '7d', 'custom', 'all')
 */
export async function selectTimePreset(page, value) {
  const label = PRESET_LABELS[value];
  if (!label) throw new Error(`Unknown time preset value: ${value}`);
  await page.click('[data-testid="time-preset"]');
  await page.waitForSelector('.dropdown-portal', { timeout: 3_000 });
  await page.locator('.dropdown-portal .dp-item', { hasText: label }).click();
  // Portal auto-closes on item click; wait briefly for DOM cleanup
  await delay(150);
}

/**
 * Read the current time-preset value from the button's data-value attribute.
 * @param {import('@playwright/test').Page} page
 * @returns {Promise<string>} Preset value (e.g. 'all', '1h', 'custom')
 */
export async function getTimePresetValue(page) {
  return page.evaluate(() =>
    document.querySelector('[data-testid="time-preset"]')?.dataset.value ?? ''
  );
}

/**
 * Execute a search query by filling the input and pressing Enter.
 * @param {import('@playwright/test').Page} page
 * @param {string} query
 */
export async function executeSearch(page, query) {
  const searchInput = page.locator('.search-editor-input');
  await searchInput.fill(query);
  await searchInput.press('Enter');
}

/**
 * Open the settings dropdown and click the reset preferences button.
 * @param {import('@playwright/test').Page} page
 */
export async function clickResetPreferences(page) {
  await page.click('[data-testid="settings-btn"]');
  await page.waitForSelector('.dropdown-portal', { timeout: 3_000 });
  await page.click('[data-testid="reset-ui-prefs-btn"]');
  await delay(150);
}
