#!/usr/bin/env node
/**
 * screenshot.mjs — Generate a screenshot of peek with realistic data and a Lucene query.
 *
 * Usage: node e2e/screenshot.mjs [--output path/to/screenshot.png]
 */

import { chromium } from 'playwright';
import { spawn } from 'child_process';
import { setTimeout } from 'timers/promises';
import { dirname, resolve } from 'path';
import { fileURLToPath } from 'url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const PROJECT_ROOT = resolve(__dirname, '..');
const PORT = 9996;
const OUTPUT = process.argv.includes('--output')
  ? process.argv[process.argv.indexOf('--output') + 1]
  : resolve(PROJECT_ROOT, 'docs', 'screenshot.png');

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

async function cleanup() {
  if (browser) await browser.close().catch(() => {});
  if (server) server.kill();
}
process.on('SIGINT', cleanup);

try {
  console.log('⏳ Starting peek with realistic log data…');

  const jsonLines = logs.map(l => JSON.stringify(l)).join('\n');
  server = spawn('sh', ['-c', `echo '${jsonLines.replace(/'/g, "\\'")}' | go run ./cmd/peek --port ${PORT} --no-browser`], {
    cwd: PROJECT_ROOT,
    stdio: ['pipe', 'pipe', 'pipe'],
  });

  // Poll until ready
  const deadline = Date.now() + 30_000;
  while (Date.now() < deadline) {
    try {
      const r = await fetch(`http://localhost:${PORT}`);
      if (r.ok) break;
    } catch { /* not ready */ }
    await setTimeout(1000);
  }
  console.log('✅ Server ready');

  browser = await chromium.launch({ headless: true });
  const ctx = await browser.newContext({
    viewport: { width: 1400, height: 800 },
    deviceScaleFactor: 2,          // retina-quality screenshot
    colorScheme: 'dark',
  });
  const page = await ctx.newPage();
  await page.goto(`http://localhost:${PORT}`);
  await setTimeout(2000);

  // Type a Lucene query and execute
  await page.fill('input[type="text"]', 'level:ERROR OR level:WARN');
  await page.click('button:has-text("Search")');
  await setTimeout(1500);

  // Expand the first error row to show fields
  await page.evaluate(() => {
    const chevrons = document.querySelectorAll('.log-table-body .col-chevron');
    for (const c of chevrons) {
      const r = c.getBoundingClientRect();
      if (r.top > 50 && r.bottom < window.innerHeight) {
        c.click();
        return;
      }
    }
  });
  await setTimeout(800);

  // Pin "service" column
  const clicked = await page.evaluate(() => {
    const el = Array.from(document.querySelectorAll('.field-key'))
      .find(e => e.textContent.trim() === 'service');
    if (el) { el.click(); return true; }
    return false;
  });
  if (clicked) await setTimeout(800);

  // Ensure output directory exists
  const { mkdirSync } = await import('fs');
  mkdirSync(resolve(OUTPUT, '..'), { recursive: true });

  await page.screenshot({ path: OUTPUT, fullPage: false });
  console.log(`📸 Screenshot saved: ${OUTPUT}`);

} catch (err) {
  console.error('❌', err);
  process.exit(1);
} finally {
  await cleanup();
}
