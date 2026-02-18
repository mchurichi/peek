/**
 * Shared test helpers for peek e2e tests.
 *
 * Provides server lifecycle, common assertions, and DOM utilities
 * so individual spec files stay focused on behavior.
 */

import { spawn } from 'child_process';
import { setTimeout } from 'timers/promises';
import { dirname, resolve } from 'path';
import { fileURLToPath } from 'url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const PROJECT_ROOT = resolve(__dirname, '..');

// ── Test runner state ────────────────────────────

let _passed = 0;
let _failed = 0;

export function assert(label, condition, detail = '') {
  if (condition) {
    console.log(`  ✅ ${label}${detail ? ` (${detail})` : ''}`);
    _passed++;
  } else {
    console.log(`  ❌ ${label}${detail ? ` (${detail})` : ''}`);
    _failed++;
  }
}

export function results() {
  return { passed: _passed, failed: _failed };
}

export function resetCounters() {
  _passed = 0;
  _failed = 0;
}

// ── Server lifecycle ─────────────────────────────

const DEFAULT_PORT = 9997;
const STARTUP_TIMEOUT_MS = 30_000;
const POLL_INTERVAL_MS = 1_000;

/**
 * Start peek with piped test data and wait until it responds on `port`.
 * Returns the child process handle.
 */
export async function startServer(port = DEFAULT_PORT, { rows = 40 } = {}) {
  const proc = spawn('sh', ['-c', `
    for i in $(seq 1 ${rows}); do
      echo "{\\"level\\":\\"INFO\\",\\"msg\\":\\"Message $i\\",\\"time\\":\\"2026-02-18T10:$(printf '%02d' $i):00Z\\",\\"service\\":\\"api\\",\\"user_id\\":\\"user$i\\",\\"request_id\\":\\"req-$i\\"}"
    done | go run ./cmd/peek --port ${port} --no-browser
  `], { cwd: PROJECT_ROOT, stdio: ['pipe', 'pipe', 'pipe'] });

  // Poll until the HTTP server is ready
  const deadline = Date.now() + STARTUP_TIMEOUT_MS;
  while (Date.now() < deadline) {
    try {
      const resp = await fetch(`http://localhost:${port}`);
      if (resp.ok) return proc;
    } catch { /* not ready yet */ }
    await setTimeout(POLL_INTERVAL_MS);
  }

  proc.kill();
  throw new Error(`peek server did not start within ${STARTUP_TIMEOUT_MS / 1000}s`);
}

// ── DOM helpers ──────────────────────────────────

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
  await setTimeout(200);
}

/**
 * Click a `.field-key` whose visible text matches `name`.
 * Returns `true` if the element was found and clicked.
 */
export async function clickFieldKey(page, name) {
  return page.evaluate((n) => {
    const el = Array.from(document.querySelectorAll('.field-key'))
      .find(e => e.textContent.trim() === n);
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
      .map(d => d.textContent.replace(/[✕×]/g, '').trim())
      .filter(t => t.length > 0)
  );
}

// ── Report / exit ────────────────────────────────

/**
 * Print final results and return the exit code.
 * Call this at the end of a spec's `finally` block.
 */
export function printSummary() {
  const { passed, failed } = results();
  console.log(`\n${'═'.repeat(40)}`);
  console.log(`Results: ${passed} passed, ${failed} failed`);
  console.log(`${'═'.repeat(40)}`);
  return failed > 0 ? 1 : 0;
}
