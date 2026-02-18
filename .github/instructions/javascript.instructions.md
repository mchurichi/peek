---
description: 'JavaScript conventions for peek — VanJS web UI and Playwright E2E tests'
applyTo: '**/*.js,**/*.mjs,**/index.html'
---

# JavaScript Instructions for Peek

## Web UI (index.html)

### Framework

- VanJS loaded from CDN — zero JS dependencies, no npm packages, no build tools, no bundlers in the UI
- All UI code lives inline in `<script>` tags inside the HTML file
- The single UI source of truth is `pkg/server/index.html`, embedded via `//go:embed`

### VanJS State Rules

- Create state with `van.state(initialValue)`
- Derive computed values with `van.derive(() => expr)`
- **Never mutate state** — always replace:
  ```javascript
  // CORRECT
  logs.val = [...logs.val, newEntry]

  // WRONG — mutation
  logs.val.push(newEntry)
  ```
- Use `van.add(parent, ...children)` to attach elements

### Table / Layout

- The log table uses **CSS Grid** with `display: contents` rows — no `<table>` elements
- Column widths are controlled via `gridTemplateColumns` on the grid container
- Column pinning: users click field keys in expanded detail rows
- Column resize: drag handles on column headers manipulate `gridTemplateColumns`

### Scroll Preservation

- Expanding/collapsing rows and adding/removing columns must **not** reset scroll position
- This is a critical UX invariant — test any layout change against it

### Styling

- All CSS is inline in `<style>` tags inside the HTML
- Level colors use classes: `.level-ERROR`, `.level-WARN`, `.level-INFO`, `.level-DEBUG`, `.level-TRACE`
- Dark theme by default (`background: #1a1a2e`)

## E2E Tests (e2e/)

### Framework

- Raw Playwright scripts (Node.js) — no test runner (no Jest, Mocha, etc.)
- Import shared helpers from `e2e/helpers.mjs`

### Test Pattern

```javascript
import { startServer, assert, printSummary } from './helpers.mjs'

const server = await startServer(9997)
const browser = await chromium.launch()
// ... assertions ...
printSummary()
process.exit(failed > 0 ? 1 : 0)
```

### Conventions

- Test port: `9997` (never `8080` — avoid colliding with dev instances)
- Use `assert(label, condition, detail)` helper — not Node's `assert` module
- Screenshots go to `/tmp/peek-test-*.png`
- Each spec file is self-contained and runs independently via `node e2e/<name>.spec.mjs`
- All specs run sequentially via `./e2e/run.sh`
- Clean up server processes between specs — `pkill` on the test port

### New UI Features Need E2E Tests

- Every user-visible UI feature must have at least one E2E assertion
- Follow the existing spec file pattern (startServer → browser → assert → printSummary)
