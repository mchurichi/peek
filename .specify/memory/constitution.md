<!--
Sync Impact Report
Version change: unratified template -> 1.0.0
Modified principles:
- Template Principle 1 -> I. Single-Binary Local-First Delivery
- Template Principle 2 -> II. Storage and Query Compatibility
- Template Principle 3 -> III. Embedded UI Invariants
- Template Principle 4 -> IV. Repo-Native Verification
- Template Principle 5 -> V. Documentation and Release Discipline
Added sections:
- Implementation Guardrails
- Delivery Workflow & Quality Gates
Removed sections:
- None
Templates requiring updates:
- ✅ `.specify/memory/constitution.md`
- ✅ `.specify/templates/plan-template.md`
- ✅ `.specify/templates/spec-template.md`
- ✅ `.specify/templates/tasks-template.md`
Follow-up TODOs:
- None
-->

# Peek Constitution

## Core Principles

### I. Single-Binary Local-First Delivery
Peek MUST ship as a single Go binary that keeps the web UI embedded in
`pkg/server/index.html` via `//go:embed`. Changes MUST preserve local-first
operation, avoid introducing a frontend build step, and keep runtime UI
dependencies bundled with the binary. Rationale: the product promise in
`README.md` and `AGENTS.md` is a self-contained log collector that starts from
stdin and serves its own dashboard without external services.

### II. Storage and Query Compatibility
Changes to parsing, querying, or storage MUST preserve the current contracts
unless an explicit migration plan is documented in the feature plan. New query
behavior MUST continue to flow through `Filter.Match(*LogEntry) bool`, and
storage changes MUST preserve the `log:{timestamp_nano}:{id}` key format or
document the migration and verification steps required to replace it.
Rationale: time-range access and cross-package interoperability depend on these
invariants.

### III. Embedded UI Invariants
User-facing UI work MUST stay inside the embedded UI architecture and preserve
its interaction guarantees. `pkg/server/index.html` remains the only HTML
source, the log grid MUST continue to use CSS Grid rather than `<table>`, VanJS
state updates MUST be immutable, and changes to expansion or column behavior
MUST preserve scroll position. Rationale: these constraints protect the
zero-build UI model and critical browsing behavior.

### IV. Repo-Native Verification
Every contributor and CI change MUST be verified with the repo's native
commands. Go, Node, and test commands MUST run through `mise exec -- ...`; UI
changes MUST add or update Playwright E2E coverage; and non-UI changes MUST run
the closest relevant automated validation such as `go test`, `go vet`, or
focused command-level verification. Rationale: the repo has no separate lint or
format pipeline, so correctness depends on explicit, repeatable validation,
while end-user docs may still present direct install and usage commands.

### V. Documentation and Release Discipline
Changes MUST update the right documentation surface and release metadata at the
same time as the code. End-user usage belongs in `README.md`, developer and
testing guidance belongs in `docs/README.md`, conventions and critical rules
changes MUST update `AGENTS.md`, and commits and PR titles MUST follow
Conventional Commits while every PR carries exactly one release label from
`release:patch`, `release:minor`, `release:major`, or `skip-release`.
Rationale: Peek relies on docs and label-driven automation as operational
interfaces, not optional polish.

## Implementation Guardrails

- Features and plans MUST respect the existing repo shape: `cmd/`, `internal/`,
  `pkg/`, `e2e/`, `scripts/`, and embedded assets under `pkg/server/`.
- New UI behavior MUST not add npm packages, build tooling, or alternative HTML
  entry points.
- Installer changes MUST remain POSIX `sh` compatible and stay aligned with
  `.goreleaser.yml` archive and checksum naming.
- Any change that alters build commands, project structure, dependencies,
  architecture, conventions, or critical rules MUST update `AGENTS.md` in the
  same change.
- If a proposal needs to break any of these rules, the plan MUST record the
  justification, migration steps, and validation needed before implementation
  begins.

## Delivery Workflow & Quality Gates

- Specifications MUST identify whether a change touches single-binary delivery,
  storage and query invariants, embedded UI behavior, docs boundaries, or
  release automation.
- Implementation plans MUST include a Constitution Check that names the
  affected invariants, the validation commands to run, and any required doc or
  release updates.
- Task lists MUST include constitution-driven work when applicable: Playwright
  coverage for UI changes, `mise`-based verification, docs updates,
  release-label considerations, and AGENTS synchronization.
- Pull requests and reviews MUST verify constitution compliance explicitly, not
  implicitly.
- Work that changes governance or these repo-level rules MUST update this
  constitution, the affected templates, and any referenced guidance docs in the
  same change.

## Governance

This constitution is the authoritative source for project-wide engineering
rules in the Peek repository. When guidance conflicts, this document takes
precedence over ad hoc practice, while `AGENTS.md`, `README.md`, and
`docs/README.md` remain the operational references that MUST stay aligned with
it.

Amendments MUST describe the principle or section being changed, the reason for
the change, the downstream templates or docs that need syncing, and the
validation used to confirm the new rule is actionable. A new constitution
without prior ratification history counts as initial adoption rather than an
amendment.

Versioning follows semantic versioning for governance: MAJOR for removing or
redefining a principle in a backward-incompatible way, MINOR for adding a
principle or materially expanding required behavior, and PATCH for
clarifications or wording-only refinements. Because the previous file was an
unratified placeholder template, this document is ratified as version 1.0.0 on
first adoption.

Compliance review is mandatory for every spec, plan, task list, and PR that
touches repo-level rules, delivery architecture, UI invariants, storage or
query contracts, or release and documentation workflows. Reviews MUST name the
affected constitutional principles and the verification artifacts that prove
compliance, and MUST confirm that required template updates, docs updates,
AGENTS synchronization, and validation commands are present before approval.

**Version**: 1.0.0 | **Ratified**: 2026-03-11 | **Last Amended**: 2026-03-11
