# Peek

Minimalist CLI log collector and web UI. Single Go binary — reads structured logs from stdin, stores in BadgerDB, serves a real-time web dashboard.

## Build & Run

```bash
# Build
mise exec -- go build -o peek ./cmd/peek

# Print binary version (supports ldflags injection)
mise exec -- go run ./cmd/peek version

# Collect mode (pipe stdin + web UI)
cat app.log | mise exec -- go run ./cmd/peek --port 8080

# Standalone mode (browse stored logs)
mise exec -- go run ./cmd/peek

# E2E tests (requires Playwright)
mise exec -- npm run test:e2e
mise exec -- ./e2e/run.sh --headless --parallel 3

# Single E2E spec
mise exec -- npx playwright test e2e/table.spec.mjs
mise exec -- npx playwright test e2e/resize.spec.mjs
mise exec -- npx playwright test e2e/search.spec.mjs
mise exec -- npx playwright test e2e/search-caret.spec.mjs
mise exec -- npx playwright test e2e/sliding-window.spec.mjs
mise exec -- npx playwright test e2e/field-filter-append.spec.mjs
mise exec -- npx playwright test e2e/query-history.spec.mjs
mise exec -- npx playwright test e2e/datetime.spec.mjs
mise exec -- npx playwright test e2e/levelless.spec.mjs
mise exec -- npx playwright test e2e/copy.spec.mjs
mise exec -- npx playwright test e2e/ui-prefs.spec.mjs

# Manual test log generation
mise exec -- node e2e/loggen.mjs --count 200
mise exec -- bash -lc 'node e2e/loggen.mjs --follow --rate 20 | go run ./cmd/peek --all --no-browser'

# Linux install script (release binaries)
curl -fsSL https://raw.githubusercontent.com/mchurichi/peek/main/scripts/get-peek.sh | sh -s -- install
curl -fsSL https://raw.githubusercontent.com/mchurichi/peek/main/scripts/get-peek.sh | sh -s -- uninstall
curl -fsSL https://raw.githubusercontent.com/mchurichi/peek/main/scripts/get-peek.sh | sh -s -- uninstall --purge --force
```

No linter or formatter is configured yet. Use `mise exec -- go vet ./...` for basic checks.

## Project Structure

```
cmd/peek/main.go          CLI entry point, flag parsing, collect/standalone routing
internal/config/config.go  TOML config, defaults, size parsing
pkg/parser/detector.go     Auto-detection of log formats (JSON, logfmt)
pkg/parser/parser.go       JSON and logfmt parsers
pkg/storage/types.go       LogEntry struct, FieldInfo struct, Filter interface, Stats
pkg/storage/badger.go      BadgerDB: Store, Query, Scan, GetFields, retention
pkg/query/lucene.go        Lucene query parser (AND/OR/NOT, field:value, wildcards, ranges)
pkg/server/server.go       HTTP server, /query, /fields, WebSocket /logs, broadcast
pkg/server/index.html      Web UI (embedded via //go:embed)
playwright.config.mjs      Playwright Test runner config (Chromium, retries, artifacts)
e2e/run.sh                 Compatibility wrapper for Playwright Test invocations
e2e/helpers.mjs            Shared E2E helpers (server lifecycle, deterministic ports, DOM utils)
e2e/table.spec.mjs         Table rendering, expand/collapse, pinned columns
e2e/resize.spec.mjs        Column resize behavior
e2e/search.spec.mjs        Search syntax highlighting and field autocompletion
e2e/search-caret.spec.mjs  Search caret/overlay alignment
e2e/sliding-window.spec.mjs Sliding time presets via client-side window pruning
e2e/field-filter-append.spec.mjs Field-value click appends safe Lucene token to query
e2e/query-history.spec.mjs Query history and starred queries (localStorage, shortcuts, dropdowns)
e2e/datetime.spec.mjs      Datetime range picker UI and API integration
e2e/levelless.spec.mjs     Levelless log entries rendering and filtering
e2e/copy.spec.mjs          Row copy button and field-value click-to-filter
e2e/ui-prefs.spec.mjs      Persistent UI preferences (columns, widths, time preset, reset)
e2e/screenshot.mjs         Screenshot generator with realistic data
e2e/loggen.mjs             Manual test-data log generator (json/logfmt/mixed)
.github/workflows/ci-build-test.yml   CI pipeline (build, vet, unit tests, E2E tests)
.github/actions/compute-release-context/action.yml Composite action that computes release-label policy and SemVer context
.github/workflows/release-validate-labels-and-suggest-bump.yml Validate PR release labels and upsert bump suggestion comment
.github/workflows/release-create-tag-from-merged-pr-label.yml Run main-branch CI then create SemVer tag from merged PR release label
.github/workflows/release-publish-artifacts-from-tag.yml Publish GitHub Release artifacts on SemVer tag push
.github/scripts/release-utils.cjs Shared release-label and SemVer helper functions for workflows
scripts/get-peek.sh        Linux install/uninstall script for GitHub release binaries
.goreleaser.yml            GoReleaser build/archive/checksum config
```

## Dependencies

- Go, BadgerDB, Gorilla WebSocket, BurntSushi/toml
- Frontend: VanJS (~1KB, bundled in binary), no build step
- E2E: `@playwright/test` runner + `playwright` (Node.js)
- Release: GoReleaser via GitHub Actions

## Architecture

```
stdin → Parser (JSON/logfmt/auto) → BadgerDB (~/.peek/db)
                                         ↕
                              HTTP Server (localhost:8080)
                              ├─ GET  /health
                              ├─ GET  /stats
                              ├─ GET  /fields (distinct field names + top values)
                              ├─ POST /query
                              ├─ WS   /logs (real-time)
                              └─ Web UI (embedded)
```

BadgerDB keys: `log:{timestamp_nano}:{id}` — enables time-range key seeking.

## Code Conventions

### Repository
- Use Conventional Commits for all commit messages and PR titles across the entire repository (Go, UI, docs, CI, tooling): `feat:`, `fix:`, `docs:`, `chore:`, `refactor:`, `test:`, `perf:`, `ci:`, `build:`, `style:`, `revert:`; use `!` for breaking changes, e.g. `feat!:`

### Go
- Standard layout: `cmd/`, `pkg/`, `internal/`
- Wrap errors: `fmt.Errorf("context: %w", err)`
- Storage methods hold `sync.RWMutex` for concurrent access
- All query filters implement `Filter` interface: `Match(*LogEntry) bool`
- Key prefixes: `log:`

### Web UI
- VanJS reactive state via `van.state()` and `van.derive()`
- Immutable state updates only — replace arrays/objects, never mutate
- Grid-based table using CSS Grid with `display: contents` rows, not `<table>`
- Column pinning: click field keys in expanded detail rows
- Column resize: drag handles manipulate `gridTemplateColumns`
- Search bar: transparent `<input>` over a syntax-highlight `<div>` (`.search-highlight`) for Lucene token coloring
- Autocomplete dropdown (`.search-autocomplete`) populated from `/fields` API; dismiss with Escape, navigate with arrow keys, accept with Tab/Enter
- Dropdown portal pattern: `openDropdown(triggerEl, buildFn, opts)` appends to `document.body`, positions via `getBoundingClientRect()`, dismisses on outside click/Escape
- Time presets are dropdown-based buttons (`[data-testid="time-preset"]` with `data-value` attribute), not `<select>` elements
- Theme system: CSS custom properties on `:root` (dark) with `.light` overrides; toggle via `applyTheme()`/`toggleTheme()`
- Density modes: `.density-compact` / `.density-comfortable` CSS classes with per-density token overrides
- Inline SVG icon registry: `ICONS` object with Lucide icon paths, rendered via `icon(name, cls)` helper
- Relative time presets (`15m`, `1h`, `6h`, `24h`, `7d`) slide client-side by pruning stale rows on a 1s timer (2m grace), without periodic `/query` polling

### E2E Tests
- Playwright Test runner (`@playwright/test`) with Chromium project
- Pattern: per-spec `beforeAll/afterAll` starts/stops isolated peek server and runs assertions with `expect`
- Wrapper script: `e2e/run.sh` delegates to `npx playwright test`
- Default base test port: `9997` (override with `PEEK_E2E_BASE_PORT`)
- Deterministic port formula: `base + workerIndex*100 + fileOffset`
- Shared helpers include `selectTimePreset()`, `getTimePresetValue()`, `executeSearch()`, `clickResetPreferences()` for dropdown-based UI interactions
- Screenshots: `/tmp/peek-test-*.png`

## Critical Rules

- **Single UI file**: `pkg/server/index.html` is the only HTML source — embedded via `//go:embed`
- **Scroll preservation**: expanding rows and adding columns must not reset scroll position — this is a critical UX invariant
- **Zero JS dependencies**: no build tools, no npm packages in the UI. VanJS is bundled in the binary (`pkg/server/van.min.js`, served at `/van.min.js`).
- **Single binary**: do not break the `//go:embed` distribution model
- **No `<table>` elements**: the log table is CSS Grid
- **No VanJS state mutation**: always replace (`logs.val = [...logs.val, entry]`)
- **Filter interface**: new query features must implement `Match(*LogEntry) bool`
- **BadgerDB key format**: maintain `log:{timestamp_nano}:{id}` — time-range optimizations depend on it
- **New UI features need E2E tests** following the existing Playwright pattern
- **Run Go and test commands via mise**: use `mise exec -- ...` for `go`, `node`, and `npm` commands documented here
- **Docs boundary**: `/README.md` is consumer-facing usage; `/docs/README.md` is technical/developer/testing guidance
- **Technical utilities docs**: document tools like `e2e/loggen.mjs` in `/docs/README.md`, not `/README.md`
- **Installer script compatibility**: keep `scripts/get-peek.sh` POSIX `sh` and aligned with GoReleaser asset names/checksum output
- **Keep AGENTS.md up to date**: any change that alters build commands, project structure, dependencies, architecture, conventions, or critical rules documented here MUST include a corresponding update to this file in the same commit
- **Release labels are mandatory for PRs**: exactly one of `release:patch`, `release:minor`, `release:major`, or `skip-release` is required before merge
