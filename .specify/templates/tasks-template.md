---

description: "Task list template for feature implementation"
---

# Tasks: [FEATURE NAME]

**Input**: Design documents from `/specs/[###-feature-name]/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Include the verification work required by the specification and the
constitution. UI changes MUST include Playwright E2E coverage, and every
feature MUST list the `mise exec -- ...` commands used for verification.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- **CLI entry and command routing**: `cmd/peek/`
- **Internal config and shared logic**: `internal/config/`
- **Core packages**: `pkg/parser/`, `pkg/query/`, `pkg/storage/`, `pkg/server/`
- **Embedded UI assets**: `pkg/server/index.html`, `pkg/server/van.min.js`
- **End-to-end coverage**: `e2e/*.spec.mjs`, `e2e/helpers.mjs`
- **Scripts and docs**: `scripts/`, `docs/`, `.github/`
- Task descriptions MUST reference real repo paths, not generic `src/` or
  `tests/` placeholders

<!-- 
  ============================================================================
  IMPORTANT: The tasks below are SAMPLE TASKS for illustration purposes only.
  
  The /speckit.tasks command MUST replace these with actual tasks based on:
  - User stories from spec.md (with their priorities P1, P2, P3...)
  - Feature requirements from plan.md
  - Entities from data-model.md
  - Endpoints from contracts/
  
  Tasks MUST be organized by user story so each story can be:
  - Implemented independently
  - Tested independently
  - Delivered as an MVP increment
  
  DO NOT keep these sample tasks in the generated tasks.md file.
  ============================================================================
-->

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and basic structure

- [ ] T001 Create or update the concrete repo paths named in plan.md
- [ ] T002 Wire feature entry points in `cmd/peek/`, `internal/config/`, or the
      affected `pkg/` packages
- [ ] T003 [P] Add fixtures, helpers, or scripts needed for `mise exec -- ...`
      verification flows

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

Examples of foundational tasks (adjust based on your project):

- [ ] T004 Establish shared parser, query, storage, or server plumbing required
      by all stories
- [ ] T005 [P] Add or update shared config and CLI wiring in `cmd/peek/` and
      `internal/config/`
- [ ] T006 [P] Add shared HTTP, WebSocket, or embedded UI support in
      `pkg/server/` if the feature needs it
- [ ] T007 Create shared Go test helpers or Playwright fixtures used by multiple
      stories
- [ ] T008 Configure error handling, logging, or compatibility guards needed
      across packages
- [ ] T009 Record required docs, AGENTS, or release-automation updates if the
      feature changes repo-level behavior

**Checkpoint**: Foundation ready - user story implementation can now begin in parallel

---

## Phase 3: User Story 1 - [Title] (Priority: P1) 🎯 MVP

**Goal**: [Brief description of what this story delivers]

**Independent Test**: [How to verify this story works on its own]

### Tests for User Story 1 ⚠️

> **NOTE: Add the automated verification required by the spec and constitution
> before or alongside implementation. UI work MUST include Playwright coverage.**

- [ ] T010 [P] [US1] Add or update Go tests in `pkg/.../*_test.go` for the
      affected package behavior
- [ ] T011 [P] [US1] Add or update `e2e/[feature].spec.mjs` when the story
      changes embedded UI behavior

### Implementation for User Story 1

- [ ] T012 [P] [US1] Implement package changes in the real repo paths from
      plan.md (for example `pkg/parser/`, `pkg/query/`, or `pkg/storage/`)
- [ ] T013 [P] [US1] Update server or UI wiring in `pkg/server/` when required
- [ ] T014 [US1] Wire CLI or config behavior in `cmd/peek/` or
      `internal/config/` if the story changes user-facing commands
- [ ] T015 [US1] Add validation, compatibility guards, and error handling
- [ ] T016 [US1] Update end-user or developer docs when the story changes
      behavior
- [ ] T017 [US1] Capture the `mise exec -- ...` verification commands used for
      this story

**Checkpoint**: At this point, User Story 1 should be fully functional and testable independently

---

## Phase 4: User Story 2 - [Title] (Priority: P2)

**Goal**: [Brief description of what this story delivers]

**Independent Test**: [How to verify this story works on its own]

### Tests for User Story 2 ⚠️

- [ ] T018 [P] [US2] Add or update Go tests in `pkg/.../*_test.go` for the
      affected package behavior
- [ ] T019 [P] [US2] Add or update `e2e/[feature].spec.mjs` when the story
      changes embedded UI behavior

### Implementation for User Story 2

- [ ] T020 [P] [US2] Implement package changes in the concrete repo paths from
      plan.md
- [ ] T021 [US2] Update server, UI, or CLI behavior in the affected package
- [ ] T022 [US2] Add integration work with User Story 1 components when needed
- [ ] T023 [US2] Update docs, AGENTS, or release-supporting files if the story
      changes repo-level behavior

**Checkpoint**: At this point, User Stories 1 AND 2 should both work independently

---

## Phase 5: User Story 3 - [Title] (Priority: P3)

**Goal**: [Brief description of what this story delivers]

**Independent Test**: [How to verify this story works on its own]

### Tests for User Story 3 ⚠️

- [ ] T024 [P] [US3] Add or update Go tests in `pkg/.../*_test.go` for the
      affected package behavior
- [ ] T025 [P] [US3] Add or update `e2e/[feature].spec.mjs` when the story
      changes embedded UI behavior

### Implementation for User Story 3

- [ ] T026 [P] [US3] Implement package changes in the concrete repo paths from
      plan.md
- [ ] T027 [US3] Update server, UI, CLI, or installer behavior in the affected
      files
- [ ] T028 [US3] Add docs, AGENTS, and verification-command updates required by
      the story

**Checkpoint**: All user stories should now be independently functional

---

[Add more user story phases as needed, following the same pattern]

---

## Phase N: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories

- [ ] TXXX [P] Documentation updates in `README.md` or `docs/README.md`
- [ ] TXXX Code cleanup and refactoring
- [ ] TXXX Performance optimization across all stories
- [ ] TXXX [P] Additional Go or Playwright coverage required by the constitution
- [ ] TXXX Update `AGENTS.md` when repo conventions or critical rules change
- [ ] TXXX Review release-label and release-automation implications in `.github/`
- [ ] TXXX Run and record the final `mise exec -- ...` validation commands

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Stories (Phase 3+)**: All depend on Foundational phase completion
  - User stories can then proceed in parallel (if staffed)
  - Or sequentially in priority order (P1 → P2 → P3)
- **Polish (Final Phase)**: Depends on all desired user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational (Phase 2) - No dependencies on other stories
- **User Story 2 (P2)**: Can start after Foundational (Phase 2) - May integrate with US1 but should be independently testable
- **User Story 3 (P3)**: Can start after Foundational (Phase 2) - May integrate with US1/US2 but should be independently testable

### Within Each User Story

- Required automated verification MUST be added before a story is marked done
- Package-level changes before cross-package integration
- Core implementation before integration
- Story-specific docs and validation before moving to next priority

### Parallel Opportunities

- All Setup tasks marked [P] can run in parallel
- All Foundational tasks marked [P] can run in parallel (within Phase 2)
- Once Foundational phase completes, all user stories can start in parallel (if team capacity allows)
- All tests for a user story marked [P] can run in parallel
- Independent package tasks within a story marked [P] can run in parallel
- Different user stories can be worked on in parallel by different team members

---

## Parallel Example: User Story 1

```bash
# Launch User Story 1 verification together:
Task: "Add Go coverage for the affected package in pkg/.../*_test.go"
Task: "Add embedded UI coverage in e2e/[feature].spec.mjs"

# Launch independent package work together:
Task: "Implement parser or query changes in pkg/parser/ or pkg/query/"
Task: "Update server or embedded UI behavior in pkg/server/"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL - blocks all stories)
3. Complete Phase 3: User Story 1
4. **STOP and VALIDATE**: Test User Story 1 independently
5. Deploy/demo if ready

### Incremental Delivery

1. Complete Setup + Foundational → Foundation ready
2. Add User Story 1 → Test independently → Deploy/Demo (MVP!)
3. Add User Story 2 → Test independently → Deploy/Demo
4. Add User Story 3 → Test independently → Deploy/Demo
5. Each story adds value without breaking previous stories

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together
2. Once Foundational is done:
   - Developer A: User Story 1
   - Developer B: User Story 2
   - Developer C: User Story 3
3. Stories complete and integrate independently

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Verify the required automated checks cover the story before implementing
- Use logical review checkpoints; commit strategy is project-specific
- Stop at any checkpoint to validate story independently
- Avoid: vague tasks, same file conflicts, cross-story dependencies that break independence
