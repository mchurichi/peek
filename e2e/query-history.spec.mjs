#!/usr/bin/env node
/**
 * query-history.spec.mjs — Query history and starred queries
 *
 * Covers: localStorage persistence (history + starred), history dropdown,
 * starred dropdown, Alt+↑/↓/S/C keyboard shortcuts, copy action,
 * and verifying existing autocomplete Enter/Tab still work.
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
  // ── Setup ──────────────────────────────────────
  console.log('⏳ Starting peek server…');
  server = await startServer(PORT);
  console.log('✅ Server ready');

  const headless = !!(process.env.CI || process.env.HEADLESS);
  browser = await chromium.launch({ headless, ...(headless ? {} : { slowMo: 80 }) });
  const ctx = await browser.newContext({ viewport: { width: 1400, height: 900 } });
  const page = await ctx.newPage();
  page.on('pageerror', err => console.error('  PAGE ERROR:', err.message));

  // Navigate and clear any leftover localStorage
  await page.goto(BASE_URL);
  await setTimeout(1500);
  await page.evaluate(() => {
    localStorage.removeItem('peek.queryHistory.v1');
    localStorage.removeItem('peek.starredQueries.v1');
  });
  await page.reload();
  await setTimeout(1500);

  const searchInput = page.locator('.search-editor-input');

  // ── 1. Query history persistence ───────────────
  console.log('\n1️⃣  Query history persistence');

  // Execute 3 different queries
  await searchInput.fill('level:ERROR');
  await searchInput.press('Enter');
  await setTimeout(800);

  await searchInput.fill('service:api');
  await searchInput.press('Enter');
  await setTimeout(800);

  await searchInput.fill('level:INFO');
  await searchInput.press('Enter');
  await setTimeout(800);

  const histBefore = await page.evaluate(() => {
    try { return JSON.parse(localStorage.getItem('peek.queryHistory.v1') || '[]') } catch { return [] }
  });
  assert('History has 3 entries', histBefore.length === 3, `got ${histBefore.length}`);
  assert('Most recent query is first', histBefore[0]?.query === 'level:INFO', `got: ${histBefore[0]?.query}`);
  assert('Second entry is correct', histBefore[1]?.query === 'service:api', `got: ${histBefore[1]?.query}`);
  assert('Third entry is correct', histBefore[2]?.query === 'level:ERROR', `got: ${histBefore[2]?.query}`);
  assert('History entry has useCount', histBefore[0]?.useCount === 1, `got: ${histBefore[0]?.useCount}`);
  assert('History entry has lastUsedAt', typeof histBefore[0]?.lastUsedAt === 'string', `got: ${histBefore[0]?.lastUsedAt}`);

  // Reload and confirm persistence
  await page.reload();
  await setTimeout(1500);

  const histAfterReload = await page.evaluate(() => {
    try { return JSON.parse(localStorage.getItem('peek.queryHistory.v1') || '[]') } catch { return [] }
  });
  assert('History survives reload', histAfterReload.length === 3, `got ${histAfterReload.length}`);
  assert('History order preserved after reload', histAfterReload[0]?.query === 'level:INFO', `got: ${histAfterReload[0]?.query}`);

  // Repeating a query moves it to top and increments useCount
  await searchInput.fill('level:ERROR');
  await searchInput.press('Enter');
  await setTimeout(800);

  const histAfterRepeat = await page.evaluate(() => {
    try { return JSON.parse(localStorage.getItem('peek.queryHistory.v1') || '[]') } catch { return [] }
  });
  assert('Repeated query moves to top', histAfterRepeat[0]?.query === 'level:ERROR', `got: ${histAfterRepeat[0]?.query}`);
  assert('Repeated query increments useCount', histAfterRepeat[0]?.useCount === 2, `got: ${histAfterRepeat[0]?.useCount}`);
  assert('History stays deduped (still 3 entries)', histAfterRepeat.length === 3, `got ${histAfterRepeat.length}`);

  // Empty and * queries are not saved
  await searchInput.fill('');
  await searchInput.press('Enter');
  await setTimeout(500);
  await searchInput.fill('*');
  await searchInput.press('Enter');
  await setTimeout(500);
  const histAfterEmpty = await page.evaluate(() => {
    try { return JSON.parse(localStorage.getItem('peek.queryHistory.v1') || '[]') } catch { return [] }
  });
  assert('Empty query not saved to history', histAfterEmpty.length === 3, `got ${histAfterEmpty.length}`);

  // ── 2. History dropdown ─────────────────────────
  console.log('\n2️⃣  History dropdown');

  await page.reload();
  await setTimeout(1500);

  const histBtn = page.locator('[data-testid="history-btn"]');
  assert('History button is visible', await histBtn.isVisible(), '');

  await histBtn.click();
  await setTimeout(200);

  const histPanelOpen = await page.evaluate(() => {
    const panels = document.querySelectorAll('.query-panel');
    return Array.from(panels).some(p => p.style.display !== 'none');
  });
  assert('History panel opens on click', histPanelOpen, '');

  const histPanelItems = await page.evaluate(() =>
    Array.from(document.querySelectorAll('.query-panel-item .qp-text')).map(el => el.textContent)
  );
  assert('History panel shows items', histPanelItems.length > 0, `got ${histPanelItems.length} items`);
  assert('First history item is most recent', histPanelItems[0] === 'level:ERROR', `got: "${histPanelItems[0]}"`);

  // Clicking history item loads and executes query
  await page.evaluate(() => {
    document.querySelector('.query-panel-item')?.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }));
  });
  await setTimeout(800);

  const inputAfterHistClick = await searchInput.inputValue();
  assert('History item click loads query into input', inputAfterHistClick === 'level:ERROR', `got: "${inputAfterHistClick}"`);

  const histPanelClosed = await page.evaluate(() => {
    const panels = document.querySelectorAll('.query-panel');
    return Array.from(panels).every(p => p.style.display === 'none');
  });
  assert('History panel closes after item click', histPanelClosed, '');

  // Clicking outside closes the panel
  await histBtn.click();
  await setTimeout(150);
  await page.mouse.click(10, 10);
  await setTimeout(200);
  const histPanelClosedOutside = await page.evaluate(() => {
    const panels = document.querySelectorAll('.query-panel');
    return Array.from(panels).every(p => p.style.display === 'none');
  });
  assert('History panel closes on outside click', histPanelClosedOutside, '');

  // ── 3. Starred queries persistence ─────────────
  console.log('\n3️⃣  Starred queries persistence');

  await page.reload();
  await setTimeout(1500);

  // Execute a query then star it
  await searchInput.fill('level:ERROR');
  await searchInput.press('Enter');
  await setTimeout(800);

  const starBtn = page.locator('[data-testid="star-btn"]');
  assert('Star button is visible', await starBtn.isVisible(), '');

  // Star button should not be active yet (not starred)
  const starBtnClassBefore = await page.evaluate(() =>
    document.querySelector('[data-testid="star-btn"]')?.className || ''
  );
  assert('Star button not active before starring', !starBtnClassBefore.includes('starred'), `class: ${starBtnClassBefore}`);

  await starBtn.click();
  await setTimeout(200);

  const starredAfterStar = await page.evaluate(() => {
    try { return JSON.parse(localStorage.getItem('peek.starredQueries.v1') || '[]') } catch { return [] }
  });
  assert('Query starred in localStorage', starredAfterStar.includes('level:ERROR'), JSON.stringify(starredAfterStar));

  const starBtnClassAfter = await page.evaluate(() =>
    document.querySelector('[data-testid="star-btn"]')?.className || ''
  );
  assert('Star button shows active state when starred', starBtnClassAfter.includes('starred'), `class: ${starBtnClassAfter}`);

  // Reload and confirm persistence by typing the same query
  await page.reload();
  await setTimeout(1500);
  await searchInput.fill('level:ERROR');
  await setTimeout(200);  // allow oninput / VanJS to react

  const starBtnClassReloaded = await page.evaluate(() =>
    document.querySelector('[data-testid="star-btn"]')?.className || ''
  );
  assert('Starred state persists after reload', starBtnClassReloaded.includes('starred'), `class: ${starBtnClassReloaded}`);

  // Unstar
  await starBtn.click();
  await setTimeout(200);
  const starredAfterUnstar = await page.evaluate(() => {
    try { return JSON.parse(localStorage.getItem('peek.starredQueries.v1') || '[]') } catch { return [] }
  });
  assert('Query removed from starred after unstar', !starredAfterUnstar.includes('level:ERROR'), JSON.stringify(starredAfterUnstar));

  const starBtnClassUnstarred = await page.evaluate(() =>
    document.querySelector('[data-testid="star-btn"]')?.className || ''
  );
  assert('Star button no longer active after unstar', !starBtnClassUnstarred.includes('starred'), `class: ${starBtnClassUnstarred}`);

  // ── 4. Starred dropdown ─────────────────────────
  console.log('\n4️⃣  Starred dropdown');

  // Star a query then open the starred panel
  await searchInput.fill('service:api');
  await searchInput.press('Enter');
  await setTimeout(800);
  await starBtn.click();
  await setTimeout(200);

  const starredBtn = page.locator('[data-testid="starred-btn"]');
  assert('Starred button is visible', await starredBtn.isVisible(), '');

  await starredBtn.click();
  await setTimeout(200);

  const starredPanelOpen = await page.evaluate(() => {
    const panels = document.querySelectorAll('.query-panel');
    return Array.from(panels).some(p => p.style.display !== 'none');
  });
  assert('Starred panel opens on click', starredPanelOpen, '');

  const starredPanelItems = await page.evaluate(() =>
    Array.from(document.querySelectorAll('.query-panel-item .qp-text')).map(el => el.textContent)
  );
  assert('Starred panel shows items', starredPanelItems.length > 0, `got ${starredPanelItems.length} items`);
  assert('Starred panel contains starred query', starredPanelItems.includes('service:api'), JSON.stringify(starredPanelItems));

  // Clicking a starred item loads and executes query
  await page.evaluate(() => {
    document.querySelector('.query-panel-item')?.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }));
  });
  await setTimeout(800);
  const inputAfterStarClick = await searchInput.inputValue();
  assert('Starred item click loads query into input', inputAfterStarClick === 'service:api', `got: "${inputAfterStarClick}"`);

  // Opening history closes starred (mutually exclusive)
  await starredBtn.click();
  await setTimeout(150);
  await histBtn.click();
  await setTimeout(150);
  const onlyHistOpen = await page.evaluate(() => {
    const panels = document.querySelectorAll('.query-panel');
    const visible = Array.from(panels).filter(p => p.style.display !== 'none');
    return visible.length === 1;
  });
  assert('Only one panel open at a time', onlyHistOpen, '');
  await page.mouse.click(10, 10);
  await setTimeout(150);

  // ── 5. Keyboard shortcuts ───────────────────────
  console.log('\n5️⃣  Keyboard shortcuts — Alt+↑/↓ history navigation');

  await page.reload();
  await setTimeout(1500);

  // Populate history: execute two queries
  await searchInput.fill('level:ERROR');
  await searchInput.press('Enter');
  await setTimeout(800);

  await searchInput.fill('service:api');
  await searchInput.press('Enter');
  await setTimeout(800);

  // Clear the input so we can navigate from blank state
  await searchInput.fill('');
  await setTimeout(100);
  await searchInput.focus();

  // Alt+ArrowUp → most recent history item
  await page.keyboard.press('Alt+ArrowUp');
  await setTimeout(100);
  const inputAfterAltUp = await searchInput.inputValue();
  assert('Alt+ArrowUp loads most recent history item', inputAfterAltUp === 'service:api', `got: "${inputAfterAltUp}"`);

  // Alt+ArrowUp again → older item
  await page.keyboard.press('Alt+ArrowUp');
  await setTimeout(100);
  const inputAfterAltUp2 = await searchInput.inputValue();
  assert('Alt+ArrowUp again loads older history item', inputAfterAltUp2 === 'level:ERROR', `got: "${inputAfterAltUp2}"`);

  // Alt+ArrowDown → back to newer item
  await page.keyboard.press('Alt+ArrowDown');
  await setTimeout(100);
  const inputAfterAltDown = await searchInput.inputValue();
  assert('Alt+ArrowDown navigates to newer item', inputAfterAltDown === 'service:api', `got: "${inputAfterAltDown}"`);

  // Alt+ArrowDown from index 0 → restores original input
  await page.keyboard.press('Alt+ArrowUp');
  await setTimeout(100);
  await page.keyboard.press('Alt+ArrowDown');
  await setTimeout(100);
  await page.keyboard.press('Alt+ArrowDown');
  await setTimeout(100);
  const inputAfterRestore = await searchInput.inputValue();
  assert('Alt+ArrowDown at top restores original input', inputAfterRestore === '', `got: "${inputAfterRestore}"`);

  // ── 6. Alt+S — star/unstar ──────────────────────
  console.log('\n6️⃣  Keyboard shortcut — Alt+S (star/unstar)');

  await searchInput.fill('level:WARN');
  await setTimeout(100);
  await page.keyboard.press('Alt+s');
  await setTimeout(200);

  const starredAfterAltS = await page.evaluate(() => {
    try { return JSON.parse(localStorage.getItem('peek.starredQueries.v1') || '[]') } catch { return [] }
  });
  assert('Alt+S stars the current query', starredAfterAltS.includes('level:WARN'), JSON.stringify(starredAfterAltS));

  // Alt+S again → unstar
  await page.keyboard.press('Alt+s');
  await setTimeout(200);
  const starredAfterAltSUnstar = await page.evaluate(() => {
    try { return JSON.parse(localStorage.getItem('peek.starredQueries.v1') || '[]') } catch { return [] }
  });
  assert('Alt+S again unstars the query', !starredAfterAltSUnstar.includes('level:WARN'), JSON.stringify(starredAfterAltSUnstar));

  // ── 7. Alt+C — copy current query ──────────────
  console.log('\n7️⃣  Keyboard shortcut — Alt+C (copy query)');

  // Stub the clipboard API before issuing the shortcut
  await page.evaluate(() => {
    window._clipboardText = '';
    Object.defineProperty(navigator, 'clipboard', {
      get: () => ({ writeText: async t => { window._clipboardText = t; } }),
      configurable: true,
    });
  });

  await searchInput.fill('level:DEBUG');
  await setTimeout(100);
  await page.keyboard.press('Alt+c');
  await setTimeout(300);

  const copiedText = await page.evaluate(() => window._clipboardText);
  assert('Alt+C copies current query text to clipboard', copiedText === 'level:DEBUG', `got: "${copiedText}"`);

  // ── 8. Copy button in panel items ──────────────
  console.log('\n8️⃣  Copy button in history panel items');

  // Reuse clipboard stub (already set) — open history panel and click a copy button
  await histBtn.click();
  await setTimeout(200);

  await page.evaluate(() => {
    window._clipboardText = '';
    const copyBtn = document.querySelector('.query-panel-item .qp-copy');
    copyBtn?.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }));
  });
  await setTimeout(300);

  const copiedFromPanel = await page.evaluate(() => window._clipboardText);
  assert('Copy button in panel copies query text', copiedFromPanel.length > 0, `got: "${copiedFromPanel}"`);
  await page.mouse.click(10, 10);
  await setTimeout(150);

  // ── 9. Autocomplete Enter/Tab still work ───────
  console.log('\n9️⃣  Autocomplete Enter/Tab unaffected by new shortcuts');

  await searchInput.fill('');
  await searchInput.type('lev');
  await setTimeout(300);

  const dropdownVisible = await page.evaluate(() => {
    const d = document.querySelector('.search-autocomplete');
    return d && d.style.display !== 'none' && d.children.length > 0;
  });
  assert('Autocomplete dropdown still shows', dropdownVisible, '');

  // ArrowDown + Enter selects completion (not history navigation because no Alt key)
  await searchInput.press('ArrowDown');
  await searchInput.press('Enter');
  await setTimeout(200);
  const inputAfterAutoEnter = await searchInput.inputValue();
  assert('Enter still selects autocomplete item', inputAfterAutoEnter.includes('level'), `got: "${inputAfterAutoEnter}"`);

  const dropdownAfterEnter = await page.evaluate(() => {
    const d = document.querySelector('.search-autocomplete');
    return d && d.style.display !== 'none';
  });
  assert('Dropdown dismissed after Enter selects completion', !dropdownAfterEnter, '');

  // Tab completion
  await searchInput.fill('');
  await searchInput.type('ser');
  await setTimeout(300);
  await searchInput.press('Tab');
  await setTimeout(100);
  const inputAfterTab = await searchInput.inputValue();
  assert('Tab still accepts first autocomplete suggestion', inputAfterTab.includes('service:'), `got: "${inputAfterTab}"`);

  // Escape still dismisses dropdown
  await searchInput.fill('');
  await searchInput.type('lev');
  await setTimeout(300);
  await searchInput.press('Escape');
  await setTimeout(100);
  const dropdownAfterEsc = await page.evaluate(() => {
    const d = document.querySelector('.search-autocomplete');
    return d && d.style.display !== 'none';
  });
  assert('Escape still dismisses autocomplete dropdown', !dropdownAfterEsc, '');

  // ── Screenshot ──────────────────────────────────
  await page.screenshot({ path: '/tmp/peek-test-query-history.png', fullPage: false });
  console.log('\n📸 Screenshot: /tmp/peek-test-query-history.png');

  const exitCode = printSummary();
  if (exitCode > 0) {
    console.log('\n🔴 Some tests failed.');
    await setTimeout(5_000);
  } else {
    console.log('\n🟢 All tests passed!');
    await setTimeout(1_000);
  }

  process.exit(exitCode);

} catch (error) {
  console.error('❌ Error:', error);
  process.exit(1);
} finally {
  await cleanup();
}
