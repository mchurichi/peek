# Implementation Plan: KQL Query Language Migration

**Branch**: `001-kql-query-migration` | **Date**: 2026-03-11 | **Spec**: [`/home/maxi/Documents/dev/peek/specs/001-kql-query-migration/spec.md`](/home/maxi/Documents/dev/peek/specs/001-kql-query-migration/spec.md)
**Input**: Feature specification from `/home/maxi/Documents/dev/peek/specs/001-kql-query-migration/spec.md`

## Summary

Replace Peek's Lucene-facing query contract with a bounded Peek KQL subset while
preserving the existing search workflows that users rely on today. The plan
keeps the current runtime shape intact—parse query text, build `Filter`
instances, and execute them through storage-backed querying for both HTTP and
WebSocket flows—while updating editor behavior, saved-query migration,
documentation, screenshots, and automated coverage.

## Technical Context

**Language/Version**: Go 1.24+ with embedded HTML/CSS/JS in `pkg/server/index.html`  
**Primary Dependencies**: Go standard library, BadgerDB, Gorilla WebSocket, BurntSushi/toml, VanJS, Playwright, and official KQL semantics from Elastic documentation as the behavioral reference  
**Storage**: BadgerDB at `~/.peek/db`; preserve stored log shape and `log:{timestamp_nano}:{id}` keys with no data migration  
**Testing**: `mise exec -- go test ./...`, `mise exec -- go vet ./...`, `mise exec -- npx playwright test e2e/search.spec.mjs`, `mise exec -- npx playwright test e2e/search-caret.spec.mjs`, `mise exec -- npx playwright test e2e/field-filter-append.spec.mjs`, `mise exec -- npx playwright test e2e/query-history.spec.mjs`, `mise exec -- npx playwright test e2e/datetime.spec.mjs`, `mise exec -- npx playwright test e2e/copy.spec.mjs`, `mise exec -- npx playwright test e2e/levelless.spec.mjs`, `mise exec -- npx playwright test e2e/sliding-window.spec.mjs`  
**Target Platform**: Local CLI workflows with an embedded browser UI and live WebSocket updates  
**Project Type**: Single-binary Go CLI plus embedded web UI  
**Performance Goals**: Preserve current local log-browsing expectations documented in `docs/README.md`, including responsive query execution and sliding-window updates on representative datasets  
**Constraints**: Preserve `//go:embed` delivery, zero-build UI, immutable VanJS state, CSS Grid log table, scroll preservation, `/query` and `/fields` request shapes, `/logs` subscribe flow, the existing `query.Parse -> Filter -> storage.QueryWithTimeRange` execution model, `Filter.Match(*LogEntry) bool` compatibility, localStorage-backed history/starred-query behavior, and a single supported parser mode  
**Scale/Scope**: Touch `pkg/query/`, `pkg/server/`, `pkg/storage/`, `e2e/`, `README.md`, `docs/README.md`, `AGENTS.md`, `docs/screenshot.png` generation, and feature docs under `specs/001-kql-query-migration/`

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- `Single-Binary Local-First Delivery` — PASS: all planned changes stay inside
  the existing Go binary and embedded UI model; no new frontend build step,
  browser runtime dependency, or external service is introduced.
- `Storage and Query Compatibility` — PASS WITH EXPLICIT MIGRATION PLAN: query
  semantics and `Filter` implementations change, but stored log shape, Badger
  keys, `/query` payload shape, `/fields` output, and WebSocket subscription
  contract remain stable while the visible language changes from Lucene to Peek
  KQL.
- `Embedded UI Invariants` — PASS: `pkg/server/index.html` remains the only HTML
  source, the log grid stays CSS Grid based, query-bar changes preserve
  immutable VanJS state and scroll behavior, and search UX changes are backed by
  Playwright coverage.
- `Repo-Native Verification` — PASS: implementation verification will run via
  `mise exec -- go test ./...`, `mise exec -- go vet ./...`, and focused
  Playwright specs for search, caret alignment, field-filter append, history,
  datetime, levelless rows, and sliding-window behavior.
- `Documentation and Release Discipline` — PASS: `README.md`, `docs/README.md`,
  `AGENTS.md`, screenshot generation inputs, and release-facing query-language
  references will be updated together as part of the migration.

### Post-Design Re-check

- `Single-Binary Local-First Delivery` — PASS: the design artifacts keep the
  migration inside `pkg/query/`, `pkg/server/`, and repo docs with no new
  runtime surface.
- `Storage and Query Compatibility` — PASS: `research.md`, `data-model.md`, and
  `contracts/` preserve storage invariants and keep API contracts stable while
  documenting migration rules for query strings and saved entries.
- `Embedded UI Invariants` — PASS: contracts and quickstart explicitly preserve
  tokenizer/highlighter alignment, autocomplete, click-to-filter, and scroll
  behavior in the existing embedded UI.
- `Repo-Native Verification` — PASS: quickstart and research artifacts enumerate
  repo-native validation commands and the required Playwright coverage updates.
- `Documentation and Release Discipline` — PASS: the plan and contracts call for
  synchronized updates to end-user docs, technical docs, AGENTS guidance, and
  example/screenshot surfaces.

## Project Structure

### Documentation (this feature)

```text
specs/001-kql-query-migration/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── query-language.md
│   └── search-api.md
├── checklists/
│   └── requirements.md
└── tasks.md
```

### Source Code (repository root)

```text
cmd/peek/                # CLI entry point remains stable
internal/config/         # Config defaults if query-language settings expand
pkg/query/               # Replace Lucene parser/filter behavior with Peek KQL subset
pkg/storage/             # Preserve log-entry and field-info contracts
pkg/server/              # /query, /fields, /logs plus embedded search UI
e2e/                     # Search, autocomplete, history, time-range, and live-update coverage
docs/                    # Technical docs and screenshot assets
README.md                # End-user query-language docs and examples
AGENTS.md                # Repo guidance and query-language references
.github/                 # CI remains the enforcement path for verification
```

**Structure Decision**: The feature touches only existing parser, server, UI,
test, and documentation paths because the migration changes a visible query
contract rather than repository topology. No new top-level directories are
required; feature-specific design assets live under
`specs/001-kql-query-migration/`.
