# Peek

Minimalist CLI log collector and web UI. Single Go binary — reads structured logs from stdin, stores in BadgerDB, serves a real-time web dashboard.

## Build & Run

```bash
# Build
mise exec -- go build -o peek ./cmd/peek

# Collect mode (pipe stdin + web UI)
cat app.log | mise exec -- go run ./cmd/peek --port 8080

# Server mode (browse stored logs)
mise exec -- go run ./cmd/peek server

# E2E tests (requires Playwright)
mise exec -- npm run test:e2e

# Single E2E spec
mise exec -- node e2e/table.spec.mjs
mise exec -- node e2e/resize.spec.mjs
mise exec -- node e2e/search.spec.mjs
mise exec -- node e2e/search-caret.spec.mjs
mise exec -- node e2e/sliding-window.spec.mjs
mise exec -- node e2e/field-filter-append.spec.mjs

# Manual test log generation
mise exec -- node e2e/loggen.mjs --count 200
mise exec -- bash -lc 'node e2e/loggen.mjs --follow --rate 20 | go run ./cmd/peek --all --no-browser'
```

No linter or formatter is configured yet. Use `mise exec -- go vet ./...` for basic checks.

## Project Structure

```
cmd/peek/main.go          CLI entry point, flag parsing, collect/server routing
internal/config/config.go  TOML config, defaults, size parsing
pkg/parser/detector.go     Auto-detection of log formats (JSON, logfmt)
pkg/parser/parser.go       JSON and logfmt parsers
pkg/storage/types.go       LogEntry struct, FieldInfo struct, Filter interface, Stats
pkg/storage/badger.go      BadgerDB: Store, Query, Scan, GetFields, retention
pkg/query/lucene.go        Lucene query parser (AND/OR/NOT, field:value, wildcards, ranges)
pkg/server/server.go       HTTP server, /query, /fields, WebSocket /logs, broadcast
pkg/server/index.html      Web UI (embedded via //go:embed)
e2e/helpers.mjs            Shared Playwright helpers
e2e/table.spec.mjs         Table rendering, expand/collapse, pinned columns
e2e/resize.spec.mjs        Column resize behavior
e2e/search.spec.mjs        Search syntax highlighting and field autocompletion
e2e/search-caret.spec.mjs  Search caret/overlay alignment
e2e/sliding-window.spec.mjs Sliding time presets via client-side window pruning
e2e/screenshot.mjs         Screenshot generator with realistic data
e2e/loggen.mjs             Manual test-data log generator (json/logfmt/mixed)
.github/workflows/ci.yml   CI pipeline (build, vet, unit tests, E2E tests)
```

## Dependencies

- Go, BadgerDB, Gorilla WebSocket, BurntSushi/toml
- Frontend: VanJS from CDN (~1KB), no build step
- E2E: Playwright (Node.js)

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
- Relative time presets (`15m`, `1h`, `6h`, `24h`, `7d`) slide client-side by pruning stale rows on a 1s timer (2m grace), without periodic `/query` polling

### E2E Tests
- Raw Playwright scripts, no test runner
- Pattern: `startServer()` → launch Chromium → assertions → `printSummary()` → exit code
- Custom `assert(label, condition, detail)` helper
- Test port: `9997`
- Screenshots: `/tmp/peek-test-*.png`

## Critical Rules

- **Single UI file**: `pkg/server/index.html` is the only HTML source — embedded via `//go:embed`
- **Scroll preservation**: expanding rows and adding columns must not reset scroll position — this is a critical UX invariant
- **Zero JS dependencies**: no build tools, no npm packages in the UI. VanJS from CDN only.
- **Single binary**: do not break the `//go:embed` distribution model
- **No `<table>` elements**: the log table is CSS Grid
- **No VanJS state mutation**: always replace (`logs.val = [...logs.val, entry]`)
- **Filter interface**: new query features must implement `Match(*LogEntry) bool`
- **BadgerDB key format**: maintain `log:{timestamp_nano}:{id}` — time-range optimizations depend on it
- **New UI features need E2E tests** following the existing Playwright pattern
- **Run Go and test commands via mise**: use `mise exec -- ...` for `go`, `node`, and `npm` commands documented here
- **Docs boundary**: `/README.md` is consumer-facing usage; `/docs/README.md` is technical/developer/testing guidance
- **Technical utilities docs**: document tools like `e2e/loggen.mjs` in `/docs/README.md`, not `/README.md`
- **Keep AGENTS.md up to date**: any change that alters build commands, project structure, dependencies, architecture, conventions, or critical rules documented here MUST include a corresponding update to this file in the same commit
