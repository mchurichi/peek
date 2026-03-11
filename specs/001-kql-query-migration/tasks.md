# Tasks: KQL Query Language Migration

**Input**: Design documents from `/specs/001-kql-query-migration/`
**Prerequisites**: `plan.md`, `spec.md`, `research.md`, `data-model.md`, `contracts/`, `quickstart.md`

**Tests**: This feature requires Go and Playwright coverage because the spec calls for extensive automated acceptance coverage across parser behavior, invalid-query handling, syntax highlighting, autocomplete, click-to-filter, saved-query migration, time-range interplay, and live filtering.

**Organization**: Tasks are grouped by user story so each increment stays independently testable.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel when the touched files do not depend on unfinished tasks
- **[Story]**: Maps the task to a user story from `spec.md`
- Every task includes exact repo file paths

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Create the shared implementation and verification scaffolding for the KQL migration.

- [ ] T001 Create the KQL parser and test entrypoint files `pkg/query/kql.go`, `pkg/query/kql_test.go`, and `pkg/query/kql_behavior_test.go`
- [ ] T002 [P] Add shared KQL HTTP and WebSocket assertion helpers in `pkg/server/server_test.go` and `e2e/helpers.mjs`
- [ ] T003 [P] Prepare KQL-oriented screenshot and sample-query fixtures in `e2e/screenshot.mjs` and `e2e/loggen.mjs`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Establish the parser, contract, and runtime plumbing that every user story depends on.

**⚠️ CRITICAL**: No user story work should start until this phase is complete.

- [ ] T004 Migrate shared query types and `Filter.Match(*LogEntry) bool` compatibility from `pkg/query/lucene.go` into `pkg/query/kql.go`
- [ ] T005 [P] Repoint `/query` and `/logs` from Lucene parsing to KQL parsing in `pkg/server/server.go` and `pkg/server/server_test.go`
- [ ] T006 [P] Preserve `FieldInfo` and time-range contract compatibility for KQL autocomplete in `pkg/storage/types.go`, `pkg/storage/badger.go`, and `pkg/server/server.go`
- [ ] T007 [P] Add shared invalid-query and migration-status plumbing in `pkg/server/server.go`, `pkg/server/server_test.go`, and `pkg/server/index.html`
- [ ] T008 Remove the retired Lucene parser implementation and test files `pkg/query/lucene.go`, `pkg/query/lucene_test.go`, and `pkg/query/lucene_behavior_test.go` once `pkg/query/kql.go` is wired everywhere

**Checkpoint**: KQL runtime plumbing is in place and all user stories can build on one parser mode.

---

## Phase 3: User Story 1 - Search Logs with KQL (Priority: P1) 🎯 MVP

**Goal**: Replace Lucene search execution with the bounded Peek KQL subset while preserving current filtering workflows and adding migration-aware failures.

**Independent Test**: Run KQL match-all, field, phrase, wildcard, comparison, exists, and grouped queries through `/query` and `/logs`; confirm results match the intended filter behavior and unsupported Lucene-only syntax returns corrective guidance.

### Tests for User Story 1

- [ ] T009 [P] [US1] Add parser parity coverage for KQL match-all, field, phrase, boolean, wildcard, exists, and grouped queries in `pkg/query/kql_test.go`
- [ ] T010 [P] [US1] Add KQL edge-case coverage for relative time, comparison ranges, inconsistent numeric values, and legacy Lucene rejection in `pkg/query/kql_behavior_test.go`
- [ ] T011 [P] [US1] Update HTTP and WebSocket contract tests for KQL success and corrective failure cases in `pkg/server/server_test.go`

### Implementation for User Story 1

- [ ] T012 [US1] Implement the bounded Peek KQL grammar and filter construction in `pkg/query/kql.go`
- [ ] T013 [US1] Keep `/query` execution aligned with KQL semantics and external `start`/`end` bounds in `pkg/server/server.go`
- [ ] T014 [US1] Keep `/logs` subscription filtering and client-visible invalid-query results aligned with `/query` in `pkg/server/server.go`
- [ ] T015 [US1] Verify free-text, field, exists, and comparison behavior against `storage.LogEntry` values in `pkg/query/kql.go` and `pkg/storage/types.go`
- [ ] T016 [US1] Record the core KQL verification flow and commands in `specs/001-kql-query-migration/quickstart.md`

**Checkpoint**: User Story 1 delivers a working KQL backend with explicit migration-aware failure handling.

---

## Phase 4: User Story 2 - Compose Queries Confidently in the UI (Priority: P2)

**Goal**: Update the query editor, autocomplete, click-to-filter, and saved-query flows so every UI-generated search string is valid KQL.

**Independent Test**: Compose KQL queries through typing, autocomplete, click-to-filter, query history, starred queries, and time controls; confirm the generated query text is valid KQL and the UI preserves caret and scroll behavior.

### Tests for User Story 2

- [ ] T017 [P] [US2] Update search editor highlighting and caret-alignment coverage for KQL tokens in `e2e/search.spec.mjs` and `e2e/search-caret.spec.mjs`
- [ ] T018 [P] [US2] Update click-to-filter and copy-interference coverage for KQL query generation in `e2e/field-filter-append.spec.mjs` and `e2e/copy.spec.mjs`
- [ ] T019 [P] [US2] Update history, starred-query migration, and time-range interaction coverage in `e2e/query-history.spec.mjs`, `e2e/datetime.spec.mjs`, and `e2e/sliding-window.spec.mjs`

### Implementation for User Story 2

- [ ] T020 [US2] Replace the Lucene tokenizer/highlighter with KQL-aware tokenization in `pkg/server/index.html`
- [ ] T021 [US2] Update autocomplete suggestions for fields, values, boolean operators, comparisons, and existence flows in `pkg/server/index.html`
- [ ] T022 [US2] Update field-value append logic so clicks emit valid KQL in `pkg/server/index.html`
- [ ] T023 [US2] Migrate query history and starred-query persistence to support KQL reuse and `needs-attention` states in `pkg/server/index.html`
- [ ] T024 [US2] Keep levelless rows, table behavior, and UI preference flows compatible with KQL-driven filtering in `pkg/server/index.html`, `e2e/levelless.spec.mjs`, and `e2e/ui-prefs.spec.mjs`

**Checkpoint**: User Story 2 delivers a KQL-aware editor and saved-query experience without breaking existing browsing interactions.

---

## Phase 5: User Story 3 - Learn and Trust the Migration (Priority: P3)

**Goal**: Remove Lucene-centric language from product guidance and replace it with KQL-first examples, migration help, and updated screenshots.

**Independent Test**: Read the UI help, README, docs, and screenshot example surfaces; confirm KQL is the active language, legacy patterns are explained with KQL equivalents, and no Lucene-first guidance remains.

### Tests for User Story 3

- [ ] T025 [P] [US3] Extend migration-guidance assertions for unsupported legacy syntax and saved-query recovery in `e2e/search.spec.mjs` and `e2e/query-history.spec.mjs`

### Implementation for User Story 3

- [ ] T026 [US3] Replace Lucene-centric end-user query examples and migration guidance in `README.md`
- [ ] T027 [US3] Update technical query API and testing guidance for Peek KQL in `docs/README.md`
- [ ] T028 [US3] Remove Lucene terminology from repo guidance and query-language references in `AGENTS.md`
- [ ] T029 [US3] Update in-product migration copy, empty states, and corrective guidance in `pkg/server/index.html`
- [ ] T030 [US3] Regenerate KQL-first screenshot inputs and output in `e2e/screenshot.mjs` and `docs/screenshot.png`

**Checkpoint**: User Story 3 delivers KQL-first docs and migration guidance across product, docs, and assets.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Final cleanup, repo-wide consistency, and full verification.

- [ ] T031 [P] Sweep remaining Lucene strings from test titles and fixture comments in `pkg/server/server_test.go`, `e2e/search.spec.mjs`, `e2e/field-filter-append.spec.mjs`, and `e2e/screenshot.mjs`
- [ ] T032 [P] Re-run and stabilize the cross-story acceptance suites in `e2e/search.spec.mjs`, `e2e/query-history.spec.mjs`, `e2e/datetime.spec.mjs`, `e2e/copy.spec.mjs`, `e2e/levelless.spec.mjs`, and `e2e/sliding-window.spec.mjs`
- [ ] T033 Run and record the final `mise exec -- ...` validation commands in `specs/001-kql-query-migration/quickstart.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies; start immediately.
- **Foundational (Phase 2)**: Depends on Setup; blocks all user stories.
- **User Story 1 (Phase 3)**: Depends on Foundational; this is the MVP and establishes the working KQL runtime.
- **User Story 2 (Phase 4)**: Depends on User Story 1 because the UI must target the final KQL runtime behavior.
- **User Story 3 (Phase 5)**: Depends on User Stories 1 and 2 so docs and migration guidance reflect the final parser and UI behavior.
- **Polish (Phase 6)**: Depends on all selected user stories being complete.

### User Story Dependencies

- **US1 (P1)**: No story dependency beyond the foundational phase.
- **US2 (P2)**: Depends on US1's parser/runtime contract and invalid-query behavior.
- **US3 (P3)**: Depends on US1 and US2 so docs, screenshots, and migration guidance match the shipped behavior.

### Parallel Opportunities

- Tasks marked **[P]** in Phase 1 can run in parallel after the feature branch is ready.
- In Phase 2, T005, T006, and T007 can proceed in parallel once T004 has established the KQL entrypoint plan.
- In US1, parser tests (T009, T010) and server contract tests (T011) can proceed in parallel before implementation hardening.
- In US2, the three Playwright task groups (T017, T018, T019) can run in parallel because they cover different suites.
- In US3, README/docs/AGENTS updates (T026, T027, T028) can run in parallel while the in-product copy and screenshot work are finalized.

---

## Parallel Example: User Story 1

```bash
# Launch KQL parser coverage in parallel:
Task: "Add parser parity coverage in pkg/query/kql_test.go"
Task: "Add edge-case coverage in pkg/query/kql_behavior_test.go"

# Launch runtime contract coverage in parallel:
Task: "Update HTTP and WebSocket contract tests in pkg/server/server_test.go"
```

## Parallel Example: User Story 2

```bash
# Launch UI-facing Playwright suites in parallel:
Task: "Update e2e/search.spec.mjs and e2e/search-caret.spec.mjs"
Task: "Update e2e/field-filter-append.spec.mjs and e2e/copy.spec.mjs"
Task: "Update e2e/query-history.spec.mjs, e2e/datetime.spec.mjs, and e2e/sliding-window.spec.mjs"
```

## Parallel Example: User Story 3

```bash
# Launch documentation updates in parallel:
Task: "Replace Lucene-centric guidance in README.md"
Task: "Update technical guidance in docs/README.md"
Task: "Remove Lucene terminology from AGENTS.md"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup.
2. Complete Phase 2: Foundational.
3. Complete Phase 3: User Story 1.
4. Stop and validate the KQL runtime through `pkg/query/kql_test.go`, `pkg/query/kql_behavior_test.go`, and `pkg/server/server_test.go`.
5. Demo the MVP with the KQL queries listed in `specs/001-kql-query-migration/quickstart.md`.

### Incremental Delivery

1. Finish Setup + Foundational to establish the single KQL parser mode.
2. Deliver US1 to replace Lucene runtime behavior.
3. Deliver US2 to make the embedded UI generate and explain valid KQL.
4. Deliver US3 to finish docs, screenshots, and migration guidance.
5. Finish Phase 6 polish and full verification before release.

### Notes

- Every task follows the required checklist format with an ID and exact file path.
- `[P]` means the task can run in parallel because it targets separate files or suites.
- The suggested MVP scope is **User Story 1 only** after Setup and Foundational work.
- The task list assumes the parser rename is explicit: the migration introduces `pkg/query/kql.go` and retires the Lucene-named parser files.
