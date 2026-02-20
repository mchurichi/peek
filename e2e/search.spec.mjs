#!/usr/bin/env node
/**
 * search.spec.mjs — Lucene syntax highlighting and field autocompletion
 *
 * Covers: /fields API, highlight overlay, token CSS classes,
 * autocomplete dropdown, keyboard navigation, and query execution.
 */

import { chromium } from 'playwright';
import { setTimeout } from 'timers/promises';
import { assert, printSummary, startServer } from './helpers.mjs';

const PORT = 9997;
const BASE_URL = `http://localhost:${PORT}`;

let server = null;
let browser = null;

async function cleanup() {
  if (browser) await browser.close().catch(() => {});
  if (server) server.kill();
}
process.on('SIGINT', cleanup);

try {
  // ── Setup ────────────────────────────────────────
  console.log('⏳ Starting peek server…');
  server = await startServer(PORT);
  console.log('✅ Server ready');

  const headless = !!(process.env.CI || process.env.HEADLESS);
  browser = await chromium.launch({ headless, ...(headless ? {} : { slowMo: 80 }) });
  const ctx = await browser.newContext({ viewport: { width: 1400, height: 900 } });
  const page = await ctx.newPage();
  page.on('pageerror', err => console.error('  PAGE ERROR:', err.message));

  // ── 1. /fields API ──────────────────────────────
  console.log('\n1️⃣  /fields API');
  const fieldsResp = await page.evaluate(() =>
    fetch('/fields').then(r => r.json())
  );
  const fieldNames = (fieldsResp.fields || []).map(f => f.name);
  assert('/fields returns fields array', Array.isArray(fieldsResp.fields), `${fieldsResp.fields?.length} fields`);
  assert('/fields includes "service"', fieldNames.includes('service'), fieldNames.join(', '));
  assert('/fields includes "user_id"', fieldNames.includes('user_id'), fieldNames.join(', '));
  assert('/fields includes "request_id"', fieldNames.includes('request_id'), fieldNames.join(', '));
  assert('/fields includes built-in "level"', fieldNames.includes('level'), fieldNames.join(', '));
  assert('/fields includes built-in "message"', fieldNames.includes('message'), fieldNames.join(', '));
  assert('/fields includes built-in "timestamp"', fieldNames.includes('timestamp'), fieldNames.join(', '));

  const serviceField = (fieldsResp.fields || []).find(f => f.name === 'service');
  assert('service field has top_values', Array.isArray(serviceField?.top_values), JSON.stringify(serviceField));
  assert('service top_values includes "api"', serviceField?.top_values.includes('api'), JSON.stringify(serviceField?.top_values));

  // ── 2. Page loads with highlight overlay ────────
  console.log('\n2️⃣  Highlight overlay present');
  await page.goto(BASE_URL);
  await setTimeout(2000);

  const hlExists = await page.evaluate(() =>
    !!document.querySelector('.search-highlight')
  );
  assert('Highlight overlay element exists', hlExists);

  const inputExists = await page.evaluate(() =>
    !!document.querySelector('.search-editor-input')
  );
  assert('Search editor input exists', inputExists);

  // ── 3. Syntax highlighting tokens ───────────────
  console.log('\n3️⃣  Syntax highlighting tokens');

  const searchInput = page.locator('.search-editor-input');

  // level:ERROR — field + colon + value
  await searchInput.fill('level:ERROR');
  await setTimeout(100);
  const hasFieldSpan = await page.evaluate(() =>
    !!document.querySelector('.search-highlight .hl-field')
  );
  assert('level:ERROR renders hl-field span', hasFieldSpan);

  // AND operator
  await searchInput.fill('level:ERROR AND service:api');
  await setTimeout(100);
  const hasOpSpan = await page.evaluate(() =>
    !!document.querySelector('.search-highlight .hl-op')
  );
  assert('AND renders hl-op span', hasOpSpan);

  // Quoted string
  await searchInput.fill('"connection refused"');
  await setTimeout(100);
  const hasQuoteSpan = await page.evaluate(() =>
    !!document.querySelector('.search-highlight .hl-quote')
  );
  assert('"connection refused" renders hl-quote span', hasQuoteSpan);

  // Wildcard
  await searchInput.fill('message:*timeout*');
  await setTimeout(100);
  const hasWildcard = await page.evaluate(() =>
    !!document.querySelector('.search-highlight .hl-wildcard')
  );
  assert('*timeout* renders hl-wildcard span', hasWildcard);

  // Range brackets
  await searchInput.fill('[now-1h TO now]');
  await setTimeout(100);
  const hasRange = await page.evaluate(() =>
    !!document.querySelector('.search-highlight .hl-range')
  );
  assert('[now-1h TO now] renders hl-range span', hasRange);

  // Unmatched paren → error
  await searchInput.fill('(level:ERROR');
  await setTimeout(100);
  const hasError = await page.evaluate(() =>
    !!document.querySelector('.search-highlight .hl-error')
  );
  assert('Unmatched ( renders hl-error span', hasError);

  // Highlight in sync after rapid typing
  await searchInput.fill('');
  for (const ch of 'level:INFO') {
    await searchInput.type(ch);
    await setTimeout(20);
  }
  await setTimeout(100);
  const syncedField = await page.evaluate(() =>
    document.querySelector('.search-highlight .hl-field')?.textContent
  );
  assert('Highlight overlay stays in sync', syncedField === 'level', `got: ${syncedField}`);

  // ── 4. Autocomplete — field names ───────────────
  console.log('\n4️⃣  Autocomplete — field names');

  await searchInput.fill('');
  await searchInput.type('ser');
  await setTimeout(300);

  const dropdownVisible = await page.evaluate(() => {
    const d = document.querySelector('.search-autocomplete');
    return d && d.style.display !== 'none' && d.children.length > 0;
  });
  assert('Dropdown visible after typing "ser"', dropdownVisible);

  const hasService = await page.evaluate(() =>
    Array.from(document.querySelectorAll('.search-autocomplete-item'))
      .some(el => el.textContent.includes('service'))
  );
  assert('Dropdown contains "service"', hasService);

  // Arrow down + Enter to select
  await searchInput.press('ArrowDown');
  await searchInput.press('Enter');
  await setTimeout(150);
  const inputAfterEnter = await searchInput.inputValue();
  assert('Enter selects completion into input', inputAfterEnter.includes('service:'), `got: "${inputAfterEnter}"`);

  const dropdownAfterEnter = await page.evaluate(() => {
    const d = document.querySelector('.search-autocomplete');
    return d && d.style.display !== 'none';
  });
  assert('Dropdown dismissed after Enter', !dropdownAfterEnter);

  // Escape dismisses dropdown
  await searchInput.fill('');
  await searchInput.type('lev');
  await setTimeout(300);
  await searchInput.press('Escape');
  await setTimeout(100);
  const dropdownAfterEsc = await page.evaluate(() => {
    const d = document.querySelector('.search-autocomplete');
    return d && d.style.display !== 'none';
  });
  assert('Escape dismisses dropdown', !dropdownAfterEsc);
  const inputAfterEsc = await searchInput.inputValue();
  assert('Escape does not change input value', inputAfterEsc === 'lev', `got: "${inputAfterEsc}"`);

  // Built-in fields appear when input is empty
  await searchInput.fill('');
  await searchInput.focus();
  await searchInput.type('l');
  await setTimeout(300);
  const hasLevel = await page.evaluate(() =>
    Array.from(document.querySelectorAll('.search-autocomplete-item'))
      .some(el => el.textContent.includes('level'))
  );
  assert('Built-in field "level" appears in dropdown', hasLevel);

  // ── 5. Autocomplete — field values ──────────────
  console.log('\n5️⃣  Autocomplete — field values');

  await searchInput.fill('');
  await searchInput.type('level:');
  await setTimeout(300);

  const valueDropdown = await page.evaluate(() => {
    const d = document.querySelector('.search-autocomplete');
    return d && d.style.display !== 'none' && d.children.length > 0;
  });
  assert('Dropdown visible after typing "level:"', valueDropdown);

  const hasINFO = await page.evaluate(() =>
    Array.from(document.querySelectorAll('.search-autocomplete-item'))
      .some(el => el.textContent === 'INFO')
  );
  assert('Dropdown contains "INFO" value for level:', hasINFO);

  // Select INFO from dropdown
  await searchInput.press('ArrowDown');
  await searchInput.press('Enter');
  await setTimeout(150);
  const inputWithLevel = await searchInput.inputValue();
  assert('Selecting value inserts level:INFO', inputWithLevel === 'level:INFO', `got: "${inputWithLevel}"`);

  // service: values
  await searchInput.fill('');
  await searchInput.type('service:');
  await setTimeout(300);
  const hasApiValue = await page.evaluate(() =>
    Array.from(document.querySelectorAll('.search-autocomplete-item'))
      .some(el => el.textContent === 'api')
  );
  assert('Dropdown shows "api" for service:', hasApiValue);

  // ── 6. Query execution after autocomplete ───────
  console.log('\n6️⃣  Query execution after autocomplete');

  await searchInput.fill('');
  await searchInput.type('level:');
  await setTimeout(300);
  await searchInput.press('ArrowDown');
  await searchInput.press('Enter'); // accept completion
  await setTimeout(150);
  await searchInput.press('Enter'); // execute query
  await setTimeout(1500);

  const rowCount = await page.evaluate(() =>
    document.querySelectorAll('.log-row').length
  );
  assert('Query returns rows after autocomplete selection', rowCount > 0, `${rowCount} rows`);

  // Clear and retype — highlighting should be correct on new query
  await searchInput.fill('');
  await page.evaluate(() => {
    const inp = document.querySelector('.search-editor-input');
    if (inp) inp.dispatchEvent(new Event('input'));
  });
  await setTimeout(100);
  await searchInput.type('message:*timeout*');
  await setTimeout(150);
  const freshField = await page.evaluate(() =>
    !!document.querySelector('.search-highlight .hl-field')
  );
  assert('Highlight correct after clear and retype', freshField);

  // ── Summary ─────────────────────────────────────
  await page.screenshot({ path: '/tmp/peek-test-search.png', fullPage: false });
  console.log('\n📸 Screenshot: /tmp/peek-test-search.png');

  const exitCode = printSummary();
  if (exitCode > 0) {
    console.log('\n🔴 Some tests failed. Browser open 15s…');
    await setTimeout(15_000);
  } else {
    console.log('\n🟢 All tests passed!');
    await setTimeout(3_000);
  }

  process.exit(exitCode);

} catch (error) {
  console.error('❌ Error:', error);
  process.exit(1);
} finally {
  await cleanup();
}
