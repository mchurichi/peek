# Implementation Plan: [FEATURE]

**Branch**: `[###-feature-name]` | **Date**: [DATE] | **Spec**: [link]
**Input**: Feature specification from `/specs/[###-feature-name]/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/plan-template.md` for the execution workflow.

## Summary

[Extract from feature spec: primary requirement + technical approach from research]

## Technical Context

<!--
  ACTION REQUIRED: Replace the content in this section with the technical details
  for the project. The structure here is presented in advisory capacity to guide
  the iteration process.
-->

**Language/Version**: Go 1.24+ with embedded HTML/CSS/JS in `pkg/server/index.html`  
**Primary Dependencies**: BadgerDB, Gorilla WebSocket, BurntSushi/toml, VanJS, Playwright (when UI is affected)  
**Storage**: BadgerDB at `~/.peek/db` (unless the feature explicitly changes storage behavior)  
**Testing**: `mise exec -- go test ./...`, `mise exec -- go vet ./...`, and `mise exec -- npm run test:e2e` or focused Playwright specs for UI changes  
**Target Platform**: Local CLI workflows with an embedded browser UI  
**Project Type**: Single-binary Go CLI plus embedded web UI  
**Performance Goals**: Preserve interactive local log browsing and existing performance expectations documented in `docs/README.md`  
**Constraints**: Preserve `//go:embed` delivery, zero-build UI, immutable VanJS state, CSS Grid log table, and `log:{timestamp_nano}:{id}` keys unless a justified migration is planned  
**Scale/Scope**: Local-first log ingestion, storage, query, and UI flows across `cmd/`, `internal/`, `pkg/`, `e2e/`, `scripts/`, and docs

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- `Single-Binary Local-First Delivery`: Confirm the feature preserves the
  embedded UI model, avoids introducing a frontend build step, and keeps
  runtime UI dependencies bundled with the binary.
- `Storage and Query Compatibility`: Identify whether parsing, query semantics,
  `Filter` implementations, or the `log:{timestamp_nano}:{id}` key format are
  affected. If yes, document compatibility or a migration plan.
- `Embedded UI Invariants`: If `pkg/server/index.html` changes, confirm the log
  grid remains CSS Grid based, VanJS state updates stay immutable, and critical
  interactions such as scroll preservation remain covered.
- `Repo-Native Verification`: List the exact `mise exec -- ...` validation
  commands that will run. UI changes MUST include Playwright coverage.
- `Documentation and Release Discipline`: Identify required updates to
  `README.md`, `docs/README.md`, `AGENTS.md`, and any release-label or
  Conventional Commit expectations impacted by the work.

## Project Structure

### Documentation (this feature)

```text
specs/[###-feature]/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)
<!--
  ACTION REQUIRED: Replace the placeholder tree below with the concrete layout
  for this feature. Delete unused options and expand the chosen structure with
  real paths (e.g., apps/admin, packages/something). The delivered plan must
  not include Option labels.
-->

```text
cmd/peek/                # CLI entry point and command routing
internal/config/         # Config loading and defaults
pkg/parser/              # Log parsing and detection
pkg/query/               # Lucene query parsing and filters
pkg/storage/             # BadgerDB storage and retention
pkg/server/              # HTTP server, WebSocket, embedded UI
e2e/                     # Playwright end-to-end coverage and helpers
scripts/                 # Installer and support scripts
docs/                    # Developer and technical documentation
.github/                 # CI and release automation
```

**Structure Decision**: [Document the concrete repo paths this feature touches
and explain why they are sufficient. Add new top-level paths only if the
Constitution Check justifies them.]

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| [e.g., 4th project] | [current need] | [why 3 projects insufficient] |
| [e.g., Repository pattern] | [specific problem] | [why direct DB access insufficient] |
