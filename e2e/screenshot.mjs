#!/usr/bin/env node
/**
 * screenshot.mjs — Generate a deterministic screenshot of peek with realistic data.
 *
 * Usage: mise exec -- node e2e/screenshot.mjs [--output path/to/screenshot.png]
 */

import { chromium } from 'playwright';
import { spawn } from 'child_process';
import { setTimeout } from 'timers/promises';
import { dirname, resolve } from 'path';
import { fileURLToPath } from 'url';
import { rmSync } from 'fs';

const __dirname = dirname(fileURLToPath(import.meta.url));
const PROJECT_ROOT = resolve(__dirname, '..');
const PORT = 9996;
const OUTPUT = process.argv.includes('--output')
  ? process.argv[process.argv.indexOf('--output') + 1]
  : resolve(PROJECT_ROOT, 'docs', 'screenshot.png');
const RUN_ID = `${Date.now()}-${process.pid}`;
const DB_PATH = `/tmp/peek-screenshot-db-${RUN_ID}`;
const BIN_PATH = `/tmp/peek-screenshot-bin-${RUN_ID}`;
const STARTUP_TIMEOUT_MS = 30_000;

// Realistic microservice log lines
const logs = [
  { level: "INFO",  msg: "Server started on :8080",                    time: "2026-02-18T09:00:01Z", service: "api-gateway",    request_id: "req-001", region: "us-west-2" },
  { level: "INFO",  msg: "Connected to PostgreSQL",                    time: "2026-02-18T09:00:02Z", service: "user-service",    request_id: "req-002", region: "us-west-2" },
  { level: "INFO",  msg: "Cache warmed: 12,847 keys loaded",           time: "2026-02-18T09:00:03Z", service: "cache-service",   request_id: "req-003", region: "us-west-2" },
  { level: "INFO",  msg: "GET /api/v1/users 200 12ms",                 time: "2026-02-18T09:01:15Z", service: "api-gateway",    request_id: "req-100", user_id: "usr-42",  region: "us-west-2" },
  { level: "INFO",  msg: "JWT token validated for usr-42",             time: "2026-02-18T09:01:15Z", service: "auth-service",    request_id: "req-100", user_id: "usr-42",  region: "us-west-2" },
  { level: "WARN",  msg: "Slow query: SELECT * FROM orders (347ms)",   time: "2026-02-18T09:02:30Z", service: "order-service",   request_id: "req-201", user_id: "usr-88",  region: "us-west-2" },
  { level: "INFO",  msg: "POST /api/v1/orders 201 54ms",               time: "2026-02-18T09:02:31Z", service: "api-gateway",    request_id: "req-202", user_id: "usr-88",  region: "us-west-2" },
  { level: "ERROR", msg: "Connection refused: payment-provider:443",    time: "2026-02-18T09:03:00Z", service: "payment-service", request_id: "req-300", user_id: "usr-12",  region: "us-east-1" },
  { level: "ERROR", msg: "POST /api/v1/payments 502 timeout after 30s",time: "2026-02-18T09:03:01Z", service: "api-gateway",    request_id: "req-300", user_id: "usr-12",  region: "us-east-1" },
  { level: "WARN",  msg: "Retry 1/3 for payment-provider:443",         time: "2026-02-18T09:03:05Z", service: "payment-service", request_id: "req-300", user_id: "usr-12",  region: "us-east-1" },
  { level: "WARN",  msg: "Retry 2/3 for payment-provider:443",         time: "2026-02-18T09:03:10Z", service: "payment-service", request_id: "req-300", user_id: "usr-12",  region: "us-east-1" },
  { level: "ERROR", msg: "All retries exhausted for payment-provider",  time: "2026-02-18T09:03:15Z", service: "payment-service", request_id: "req-300", user_id: "usr-12",  region: "us-east-1" },
  { level: "INFO",  msg: "GET /api/v1/products 200 8ms",               time: "2026-02-18T09:04:00Z", service: "api-gateway",    request_id: "req-400", user_id: "usr-55",  region: "us-west-2" },
  { level: "INFO",  msg: "Cache HIT for products:featured",            time: "2026-02-18T09:04:00Z", service: "cache-service",   request_id: "req-400", region: "us-west-2" },
  { level: "DEBUG", msg: "Heartbeat OK",                               time: "2026-02-18T09:04:30Z", service: "api-gateway",    region: "us-west-2" },
  { level: "INFO",  msg: "User usr-42 updated profile",                time: "2026-02-18T09:05:12Z", service: "user-service",    request_id: "req-500", user_id: "usr-42",  region: "us-west-2" },
  { level: "WARN",  msg: "Rate limit approaching: 847/1000 rpm",       time: "2026-02-18T09:05:30Z", service: "api-gateway",    region: "us-west-2" },
  { level: "ERROR", msg: "Kafka consumer lag exceeded threshold: 15k",  time: "2026-02-18T09:06:00Z", service: "event-processor", region: "us-east-1" },
  { level: "INFO",  msg: "Auto-scaling triggered: 3 → 5 replicas",     time: "2026-02-18T09:06:05Z", service: "event-processor", region: "us-east-1" },
  { level: "INFO",  msg: "GET /api/v1/orders/ord-991 200 22ms",        time: "2026-02-18T09:07:00Z", service: "api-gateway",    request_id: "req-600", user_id: "usr-12",  region: "us-west-2" },
  { level: "INFO",  msg: "Order ord-991 status: processing",           time: "2026-02-18T09:07:01Z", service: "order-service",   request_id: "req-600", user_id: "usr-12",  region: "us-west-2" },
  { level: "ERROR", msg: "TLS handshake timeout with vault:8200",      time: "2026-02-18T09:08:00Z", service: "auth-service",    region: "us-east-1" },
  { level: "WARN",  msg: "Falling back to cached secrets",             time: "2026-02-18T09:08:01Z", service: "auth-service",    region: "us-east-1" },
  { level: "INFO",  msg: "Batch job completed: 2,341 emails sent",     time: "2026-02-18T09:09:00Z", service: "notification-svc", region: "us-west-2" },
  { level: "INFO",  msg: "GET /healthz 200 1ms",                       time: "2026-02-18T09:09:30Z", service: "api-gateway",    region: "us-west-2" },
  { level: "WARN",  msg: "Disk usage at 82%: /data/badger",            time: "2026-02-18T09:10:00Z", service: "storage-monitor",  region: "us-west-2" },
  { level: "INFO",  msg: "Compaction completed: reclaimed 1.2GB",      time: "2026-02-18T09:10:30Z", service: "storage-monitor",  region: "us-west-2" },
  { level: "ERROR", msg: "DNS resolution failed for analytics.internal",time: "2026-02-18T09:11:00Z", service: "analytics-svc",   region: "us-east-1" },
  { level: "INFO",  msg: "DELETE /api/v1/sessions/expired 200 340ms",  time: "2026-02-18T09:11:30Z", service: "api-gateway",    request_id: "req-700", region: "us-west-2" },
  { level: "INFO",  msg: "Purged 4,892 expired sessions",              time: "2026-02-18T09:11:31Z", service: "auth-service",    request_id: "req-700", region: "us-west-2" },
  { level: "WARN",  msg: "gRPC deadline exceeded: inventory-service",  time: "2026-02-18T09:12:00Z", service: "order-service",   request_id: "req-801", region: "us-west-2" },
  { level: "ERROR", msg: "Failed to reserve stock for SKU-4412",       time: "2026-02-18T09:12:01Z", service: "order-service",   request_id: "req-801", user_id: "usr-33",  region: "us-west-2" },
  { level: "INFO",  msg: "WebSocket clients connected: 24",            time: "2026-02-18T09:12:30Z", service: "api-gateway",    region: "us-west-2" },
  { level: "INFO",  msg: "Metrics pushed to Prometheus",               time: "2026-02-18T09:13:00Z", service: "metrics-exporter", region: "us-west-2" },
  { level: "INFO",  msg: "GET /api/v1/users/usr-42/orders 200 18ms",   time: "2026-02-18T09:13:15Z", service: "api-gateway",    request_id: "req-900", user_id: "usr-42",  region: "us-west-2" },
];

let server = null;
let browser = null;

async function waitForServerReady() {
  const deadline = Date.now() + STARTUP_TIMEOUT_MS;
  while (Date.now() < deadline) {
    try {
      const resp = await fetch(`http://localhost:${PORT}/health`);
      if (resp.ok) return;
    } catch {
      // Server not ready yet.
    }
    await setTimeout(250);
  }
  throw new Error(`peek server on port ${PORT} did not start within ${STARTUP_TIMEOUT_MS / 1000}s`);
}

async function waitForQueryCount(expectedMinimum = 1) {
  const deadline = Date.now() + STARTUP_TIMEOUT_MS;
  const body = {
    query: '*',
    limit: 500,
    offset: 0,
  };

  while (Date.now() < deadline) {
    try {
      const resp = await fetch(`http://localhost:${PORT}/query`, {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify(body),
      });
      if (resp.ok) {
        const data = await resp.json();
        const total = Number(data?.total || 0);
        if (total >= expectedMinimum) return;
      }
    } catch {
      // Query API not ready yet.
    }
    await setTimeout(250);
  }
  throw new Error(`timed out waiting for at least ${expectedMinimum} indexed logs`);
}

async function waitForExit(proc, timeoutMs = 5_000) {
  if (!proc || proc.exitCode !== null) return;
  await Promise.race([
    new Promise((resolveExit) => proc.once('exit', resolveExit)),
    setTimeout(timeoutMs),
  ]);
}

async function cleanup() {
  if (browser) await browser.close().catch(() => {});
  if (server && server.exitCode === null) {
    server.kill('SIGTERM');
    await waitForExit(server, 1_500);
    if (server.exitCode === null) {
      server.kill('SIGKILL');
      await waitForExit(server, 1_500);
    }
  }
  rmSync(DB_PATH, { recursive: true, force: true });
  rmSync(BIN_PATH, { force: true });
}
process.on('SIGINT', cleanup);
process.on('SIGTERM', cleanup);

try {
  rmSync(DB_PATH, { recursive: true, force: true });
  rmSync(BIN_PATH, { force: true });

  console.log('Building peek binary...');
  await new Promise((resolveBuild, rejectBuild) => {
    const build = spawn('mise', ['exec', '--', 'go', 'build', '-o', BIN_PATH, './cmd/peek'], {
      cwd: PROJECT_ROOT,
      stdio: 'inherit',
    });
    build.once('error', rejectBuild);
    build.once('exit', (code) => {
      if (code === 0) resolveBuild();
      else rejectBuild(new Error(`go build failed with exit code ${code}`));
    });
  });

  console.log('Starting peek server with isolated db...');
  server = spawn(BIN_PATH, ['--port', String(PORT), '--no-browser', '--db-path', DB_PATH, '--all'], {
    cwd: PROJECT_ROOT,
    stdio: ['pipe', 'pipe', 'pipe'],
  });
  server.stdout.on('data', (chunk) => process.stdout.write(chunk));
  server.stderr.on('data', (chunk) => process.stderr.write(chunk));
  server.once('error', (err) => {
    console.error('peek process error:', err);
  });

  const payload = logs.map((l) => `${JSON.stringify(l)}\n`).join('');
  server.stdin.write(payload);
  server.stdin.end();

  await waitForServerReady();
  await waitForQueryCount(logs.length);
  console.log(`Server ready with ${logs.length} logs`);

  browser = await chromium.launch({ headless: true });
  const ctx = await browser.newContext({
    viewport: { width: 1120, height: 640 },
    deviceScaleFactor: 2, // retina-quality screenshot
    colorScheme: 'dark',
  });
  const page = await ctx.newPage();
  await page.goto(`http://localhost:${PORT}`);
  await page.waitForSelector('.log-table-body .col-chevron');

  // Type a Lucene query and execute
  await page.fill('.search-editor-input', 'level:ERROR OR level:WARN');
  await page.press('.search-editor-input', 'Enter');
  await page.waitForTimeout(500);
  await page.waitForSelector('.log-table-body .col-chevron');

  // Expand the 5th row to show fields
  await page.evaluate(() => {
    const chevrons = document.querySelectorAll('.log-table-body .col-chevron');
    if (chevrons[4]) chevrons[4].click();
  });
  await page.waitForTimeout(300);

  // Pin "service" column
  const clicked = await page.evaluate(() => {
    const el = Array.from(document.querySelectorAll('.field-key'))
      .find(e => e.textContent.trim() === 'service');
    if (el) { el.click(); return true; }
    return false;
  });
  if (clicked) await page.waitForTimeout(300);

  // Ensure output directory exists
  const { mkdirSync } = await import('fs');
  mkdirSync(resolve(OUTPUT, '..'), { recursive: true });

  await page.screenshot({ path: OUTPUT, fullPage: false });
  console.log(`Screenshot saved: ${OUTPUT}`);

} catch (err) {
  console.error(err);
  process.exitCode = 1;
} finally {
  await cleanup();
}
