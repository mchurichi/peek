/**
 * resize.spec.mjs — Column resize behaviour.
 */

import { test, expect } from '@playwright/test';
import { setTimeout as delay } from 'timers/promises';
import { portForTestFile, startServer, stopServer } from './helpers.mjs';

let server;
let baseURL;

test.describe('resize', () => {
  test.beforeAll(async ({}, workerInfo) => {
    const port = portForTestFile(workerInfo);
    server = await startServer(port, { rows: 5 });
    baseURL = `http://localhost:${port}`;
  });

  test.afterAll(async () => {
    await stopServer(server);
  });

  test('resizes target columns while leaving others stable', async ({ page }) => {
    await page.goto(baseURL);
    await delay(2000);

    const headerWidths = async () => page.evaluate(() =>
      Array.from(document.querySelectorAll('.log-table-header > div'))
        .map((d) => ({ cls: d.className.split(' ')[0], width: d.offsetWidth, text: d.textContent.trim() }))
    );

    const dragHandle = async (colClass, dx) => {
      const handle = page.locator(`.log-table-header .${colClass} .resize-handle`);
      const box = await handle.boundingBox();
      expect(box).not.toBeNull();

      const x = box.x + box.width / 2;
      const y = box.y + box.height / 2;
      await page.mouse.move(x, y);
      await page.mouse.down();
      await page.mouse.move(x + dx, y, { steps: 15 });
      await page.mouse.up();
      await delay(300);
    };

    const before = await headerWidths();
    await dragHandle('col-time', 100);
    const after = await headerWidths();

    const timeBefore = before.find((h) => h.cls === 'col-time')?.width ?? 0;
    const timeAfter = after.find((h) => h.cls === 'col-time')?.width ?? 0;
    expect(timeAfter).toBeGreaterThan(timeBefore + 50);

    const levelBefore = before.find((h) => h.cls === 'col-level')?.width ?? 0;
    const levelAfter = after.find((h) => h.cls === 'col-level')?.width ?? 0;
    expect(Math.abs(levelAfter - levelBefore)).toBeLessThan(5);

    const before2 = await headerWidths();
    await dragHandle('col-level', 60);
    const after2 = await headerWidths();

    const lvlBefore = before2.find((h) => h.cls === 'col-level')?.width ?? 0;
    const lvlAfter = after2.find((h) => h.cls === 'col-level')?.width ?? 0;
    expect(lvlAfter).toBeGreaterThan(lvlBefore + 30);

    const timeSame = after2.find((h) => h.cls === 'col-time')?.width ?? 0;
    expect(Math.abs(timeSame - timeAfter)).toBeLessThan(5);

    const gridTemplate = await page.evaluate(() =>
      document.querySelector('.log-table')?.style.gridTemplateColumns ?? ''
    );
    expect(gridTemplate.length).toBeGreaterThan(0);
  });
});
