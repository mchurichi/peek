#!/usr/bin/env node
/**
 * loggen.mjs — Manual test-data generator for Peek.
 *
 * Usage:
 *   node e2e/loggen.mjs [options]
 */

import fs from "fs";
import { once } from "events";
import { setTimeout as sleep } from "timers/promises";

const DEFAULT_COUNT = 200;
const DEFAULT_FORMAT = "mixed";
const DEFAULT_PROFILE = "feature";
const DEFAULT_FOLLOW_RATE = 10;

const HELP_TEXT = `Peek log generator (dev/testing utility)

Usage:
  node e2e/loggen.mjs [options]

Options:
  --count <n>                Number of logs to emit in finite mode (default: ${DEFAULT_COUNT})
  --rate <n>                 Fixed emit rate in logs/sec
  --follow                   Stream continuously until Ctrl+C
  --format <mixed|json|logfmt>
                             Output format (default: ${DEFAULT_FORMAT})
  --profile <feature>        Data profile (default: ${DEFAULT_PROFILE})
  --out <path>               Write output to file (default: stdout)
  --seed <n|string>          Deterministic seed for repeatable datasets
  --help                     Show this help

Examples:
  node e2e/loggen.mjs --count 200
  node e2e/loggen.mjs --count 500 --format json | peek --all
  node e2e/loggen.mjs --count 300 --rate 50 --format mixed
  node e2e/loggen.mjs --follow --rate 20 | peek --all
  node e2e/loggen.mjs --count 1000 --out /tmp/peek-sample.log
`;

function fail(msg) {
  process.stderr.write(`Error: ${msg}\n`);
  process.stderr.write(`Run with --help for usage.\n`);
  process.exit(1);
}

function parseArgs(argv) {
  const opts = {
    count: DEFAULT_COUNT,
    rate: null,
    follow: false,
    format: DEFAULT_FORMAT,
    profile: DEFAULT_PROFILE,
    out: null,
    seed: null,
    help: false,
  };

  for (let i = 0; i < argv.length; i++) {
    const arg = argv[i];

    if (arg === "--help") {
      opts.help = true;
      continue;
    }
    if (arg === "--follow") {
      opts.follow = true;
      continue;
    }

    const readValue = (name) => {
      const v = argv[i + 1];
      if (!v || v.startsWith("--")) fail(`missing value for ${name}`);
      i++;
      return v;
    };

    if (arg === "--count") {
      const v = Number.parseInt(readValue(arg), 10);
      if (!Number.isInteger(v) || v < 0) fail("--count must be an integer >= 0");
      opts.count = v;
      continue;
    }
    if (arg === "--rate") {
      const v = Number.parseFloat(readValue(arg));
      if (!Number.isFinite(v) || v <= 0) fail("--rate must be a number > 0");
      opts.rate = v;
      continue;
    }
    if (arg === "--format") {
      const v = readValue(arg);
      if (!["mixed", "json", "logfmt"].includes(v)) {
        fail("--format must be one of: mixed, json, logfmt");
      }
      opts.format = v;
      continue;
    }
    if (arg === "--profile") {
      const v = readValue(arg);
      if (v !== "feature") fail("--profile must be: feature");
      opts.profile = v;
      continue;
    }
    if (arg === "--out") {
      opts.out = readValue(arg);
      continue;
    }
    if (arg === "--seed") {
      opts.seed = readValue(arg);
      continue;
    }

    fail(`unknown option: ${arg}`);
  }

  if (opts.follow && opts.rate == null) opts.rate = DEFAULT_FOLLOW_RATE;
  return opts;
}

function xmur3(str) {
  let h = 1779033703 ^ str.length;
  for (let i = 0; i < str.length; i++) {
    h = Math.imul(h ^ str.charCodeAt(i), 3432918353);
    h = (h << 13) | (h >>> 19);
  }
  h = Math.imul(h ^ (h >>> 16), 2246822507);
  h = Math.imul(h ^ (h >>> 13), 3266489909);
  return (h ^ (h >>> 16)) >>> 0;
}

function mulberry32(seed) {
  let t = seed >>> 0;
  return function rand() {
    t += 0x6d2b79f5;
    let r = Math.imul(t ^ (t >>> 15), 1 | t);
    r ^= r + Math.imul(r ^ (r >>> 7), 61 | r);
    return ((r ^ (r >>> 14)) >>> 0) / 4294967296;
  };
}

function makeRng(seed) {
  if (seed == null) return Math.random;
  if (/^-?\d+$/.test(seed)) return mulberry32(Number.parseInt(seed, 10) >>> 0);
  return mulberry32(xmur3(String(seed)));
}

function pick(rng, arr) {
  return arr[Math.floor(rng() * arr.length)];
}

function chance(rng, p) {
  return rng() < p;
}

function weightedPick(rng, entries) {
  let sum = 0;
  for (const e of entries) sum += e.weight;
  let n = rng() * sum;
  for (const e of entries) {
    n -= e.weight;
    if (n <= 0) return e.value;
  }
  return entries[entries.length - 1].value;
}

const SERVICES = [
  "api-gateway",
  "auth-service",
  "order-service",
  "payment-service",
  "user-service",
  "inventory-service",
  "cache-service",
  "analytics-svc",
  "notification-svc",
  "event-processor",
];
const REGIONS = ["us-east-1", "us-west-2", "eu-central-1"];
const ENVS = ["dev", "staging", "prod"];
const METHODS = ["GET", "POST", "PUT", "PATCH", "DELETE"];
const PATHS = [
  "/api/v1/users",
  "/api/v1/orders",
  "/api/v1/payments",
  "/api/v1/inventory",
  "/healthz",
  "/metrics",
  "/internal/cache/warmup",
];
const ERROR_CODES = [
  "ETIMEDOUT",
  "ECONNREFUSED",
  "EHOSTUNREACH",
  "ERATE_LIMIT",
  "EAUTH",
  "EUPSTREAM",
];

function makeMessage(level, record, rng) {
  if (!level) {
    return weightedPick(rng, [
      { weight: 4, value: "plain text line without explicit level" },
      { weight: 3, value: "processing request without level field" },
      { weight: 3, value: `service emitted unclassified event for ${record.path}` },
    ]);
  }

  const ms = record.duration_ms;
  const status = record.status;
  const method = record.method;
  const path = record.path;
  const service = record.service;

  if (level === "TRACE") {
    return weightedPick(rng, [
      { weight: 4, value: `trace span=${record.trace_id} ${method} ${path}` },
      { weight: 3, value: `trace cache key lookup service=${service}` },
      { weight: 3, value: `trace retry=${record.retry} status=${status}` },
    ]);
  }
  if (level === "DEBUG") {
    return weightedPick(rng, [
      { weight: 4, value: `debug cache hit for ${path}` },
      { weight: 3, value: `debug SQL rows=${Math.max(1, Math.floor(record.bytes / 128))}` },
      { weight: 3, value: `debug request context user=${record.user_id ?? "anonymous"}` },
    ]);
  }
  if (level === "INFO") {
    return weightedPick(rng, [
      { weight: 4, value: `${method} ${path} ${status} ${ms}ms` },
      { weight: 3, value: `request completed service=${service} duration=${ms}ms` },
      { weight: 2, value: `cache hit for key ${record.request_id}` },
      { weight: 1, value: "metrics flush completed" },
    ]);
  }
  if (level === "WARN") {
    return weightedPick(rng, [
      { weight: 4, value: `rate limit nearing threshold for ${service}` },
      { weight: 3, value: `slow request ${method} ${path} took ${ms}ms` },
      { weight: 3, value: `retry ${record.retry}/3 after upstream timeout` },
    ]);
  }
  if (level === "ERROR") {
    return weightedPick(rng, [
      { weight: 4, value: `connection refused to upstream (${record.error_code})` },
      { weight: 3, value: `timeout after ${ms}ms calling dependency` },
      { weight: 2, value: `auth failed for user ${record.user_id ?? "unknown"}` },
      { weight: 1, value: `request failed ${method} ${path} status=${status}` },
    ]);
  }

  // FATAL
  return weightedPick(rng, [
    { weight: 4, value: `fatal: unrecoverable panic in ${service}` },
    { weight: 3, value: `fatal: dependency unavailable (${record.error_code})` },
    { weight: 3, value: "fatal: shutting down worker after repeated failures" },
  ]);
}

function createRecord(index, rng) {
  const service = pick(rng, SERVICES);
  const method = pick(rng, METHODS);
  const path = pick(rng, PATHS);
  const region = pick(rng, REGIONS);
  const env = weightedPick(rng, [
    { weight: 2, value: ENVS[0] },
    { weight: 2, value: ENVS[1] },
    { weight: 6, value: ENVS[2] },
  ]);

  const levelless = chance(rng, 0.1);
  const level = levelless
    ? null
    : weightedPick(rng, [
        { weight: 5, value: "TRACE" },
        { weight: 10, value: "DEBUG" },
        { weight: 45, value: "INFO" },
        { weight: 20, value: "WARN" },
        { weight: 15, value: "ERROR" },
        { weight: 5, value: "FATAL" },
      ]);

  const status = weightedPick(rng, [
    { weight: 40, value: 200 },
    { weight: 8, value: 201 },
    { weight: 5, value: 204 },
    { weight: 7, value: 400 },
    { weight: 5, value: 401 },
    { weight: 5, value: 403 },
    { weight: 5, value: 404 },
    { weight: 8, value: 429 },
    { weight: 7, value: 500 },
    { weight: 5, value: 502 },
    { weight: 5, value: 503 },
  ]);

  const durationBase = level === "ERROR" || level === "FATAL" ? 500 : level === "WARN" ? 200 : 40;
  const duration_ms = durationBase + Math.floor(rng() * 1200);

  const record = {
    time: new Date().toISOString(),
    service,
    region,
    env,
    method,
    path,
    status,
    duration_ms,
    bytes: 200 + Math.floor(rng() * 2_000_000),
    retry: level === "WARN" || level === "ERROR" || level === "FATAL" ? 1 + Math.floor(rng() * 3) : 0,
    request_id: `req-${String(index + 1).padStart(6, "0")}`,
    trace_id: `trc-${Math.floor(rng() * 1_000_000_000).toString(16)}`,
    pod: `pod-${Math.floor(rng() * 24) + 1}`,
    host: `node-${Math.floor(rng() * 16) + 1}`,
    error_code: pick(rng, ERROR_CODES),
  };

  if (chance(rng, 0.75)) {
    record.user_id = `usr-${Math.floor(rng() * 9000) + 1000}`;
  }

  if (chance(rng, 0.3)) {
    record.session_id = `sess-${Math.floor(rng() * 1_000_000).toString(36)}`;
  }

  if (chance(rng, 0.15)) {
    record.feature_flag = pick(rng, ["checkout_v2", "search_v3", "billing_v1"]);
  }

  record.msg = makeMessage(level, record, rng);
  if (level) record.level = level;

  // Sparse optional fields to exercise empty-cell behavior in UI.
  if (!chance(rng, 0.4)) delete record.pod;
  if (!chance(rng, 0.35)) delete record.host;
  if (!chance(rng, 0.25)) delete record.error_code;
  if (!chance(rng, 0.2)) delete record.feature_flag;

  return record;
}

function logfmtEscape(value, forceQuote = false) {
  if (value === null || value === undefined) return "";
  const raw = String(value);
  if (!forceQuote && raw !== "" && /^[^\s"=]+$/.test(raw)) return raw;
  return `"${raw.replace(/\\/g, "\\\\").replace(/"/g, '\\"')}"`;
}

function toLogfmt(record) {
  const preferredOrder = [
    "time",
    "level",
    "msg",
    "service",
    "region",
    "env",
    "method",
    "path",
    "status",
    "duration_ms",
    "bytes",
    "retry",
    "request_id",
    "user_id",
    "trace_id",
    "session_id",
    "error_code",
    "feature_flag",
    "pod",
    "host",
  ];

  const keys = [
    ...preferredOrder.filter((k) => Object.prototype.hasOwnProperty.call(record, k)),
    ...Object.keys(record)
      .filter((k) => !preferredOrder.includes(k))
      .sort(),
  ];

  const parts = [];
  for (const key of keys) {
    const value = record[key];
    if (value === null || value === undefined) continue;
    const forceQuote = key === "msg";
    parts.push(`${key}=${logfmtEscape(value, forceQuote)}`);
  }
  return parts.join(" ");
}

function resolveFormat(baseFormat, rng) {
  if (baseFormat !== "mixed") return baseFormat;
  return chance(rng, 0.7) ? "json" : "logfmt";
}

async function writeLine(stream, line) {
  if (!stream.write(`${line}\n`)) {
    await once(stream, "drain");
  }
}

async function main() {
  const opts = parseArgs(process.argv.slice(2));
  if (opts.help) {
    process.stdout.write(HELP_TEXT);
    return;
  }

  const rng = makeRng(opts.seed);
  const stream = opts.out ? fs.createWriteStream(opts.out, { encoding: "utf8", flags: "w" }) : process.stdout;

  let running = true;
  process.on("SIGINT", () => {
    running = false;
  });
  process.on("SIGTERM", () => {
    running = false;
  });

  const rate = opts.rate;
  const intervalMs = rate ? 1000 / rate : 0;
  let nextTick = Date.now();
  let emitted = 0;

  while (running && (opts.follow || emitted < opts.count)) {
    const record = createRecord(emitted, rng);
    const format = resolveFormat(opts.format, rng);
    const line = format === "json" ? JSON.stringify(record) : toLogfmt(record);
    await writeLine(stream, line);
    emitted++;

    if (intervalMs > 0) {
      nextTick += intervalMs;
      const waitMs = Math.max(0, nextTick - Date.now());
      if (waitMs > 0) await sleep(waitMs);
      else nextTick = Date.now();
    }
  }

  if (stream !== process.stdout) {
    stream.end();
    await once(stream, "finish");
  }
}

main().catch((err) => {
  process.stderr.write(`Fatal: ${err?.message ?? String(err)}\n`);
  process.exit(1);
});
