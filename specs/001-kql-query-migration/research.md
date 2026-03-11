# Research: KQL Query Language Migration

## Decision 1: Ship a bounded Peek KQL subset instead of full Kibana parity

- **Decision**: Support a documented Peek KQL subset covering match-all,
  free-text search, field filters, quoted phrases, case-insensitive boolean
  operators, grouped clauses, wildcard values, comparison-style numeric and time
  filters, field existence checks, and same-field multi-value expressions.
- **Rationale**: The current product surface only needs the workflows already
  exercised through `/query`, `/logs`, query highlighting, autocomplete,
  click-to-filter, and saved-query reuse. Official Elastic guidance confirms KQL
  supports these constructs while broader features such as nested objects depend
  on data-model capabilities Peek does not expose.
- **Alternatives considered**:
  - Full Kibana/Elasticsearch KQL parity — rejected because nested-field and
    mapping-driven semantics exceed Peek's current structured-log model.
  - Keep Lucene and add optional KQL — rejected because the feature spec removes
    Lucene as the public language and dual-mode UX would prolong documentation
    and migration complexity.

## Decision 2: Keep time controls separate from the query-language migration

- **Decision**: Preserve the existing time preset and custom-range controls as
  first-class filtering surfaces while adding KQL comparison-style timestamp
  filters only where they fit the bounded language subset.
- **Rationale**: Current API and UI behavior already treat time as orthogonal to
  query parsing: `/query` and WebSocket subscribe messages carry `start` and
  `end`, and E2E coverage validates preset-driven sliding windows separately.
  Migrating the language must not collapse that existing contract.
- **Alternatives considered**:
  - Move all time filtering into query text — rejected because it would break the
    established time-range UI and expand the feature beyond the current product
    contract.
  - Disallow timestamp comparisons in KQL — rejected because comparison-style
    time filters are part of the feature goals and fit common log-investigation
    workflows.

## Decision 3: Preserve current unquoted string matching behavior for ordinary field filters

- **Decision**: Keep unquoted string field matching aligned with Peek's current
  user-visible behavior: quoted filters remain exact phrase/value matches, while
  ordinary unquoted string filters continue to behave like forgiving
  case-insensitive matches for troubleshooting workflows.
- **Rationale**: The current parser's `FieldFilter` and `KeywordFilter` favor
  practical log search over strict type-aware exact matching. A sudden move to
  exact-only semantics would break existing query expectations even if the
  syntax changed successfully.
- **Alternatives considered**:
  - Adopt exact-only KQL semantics for all non-text values — rejected because
    Peek lacks Elasticsearch-style field mappings and users already rely on more
    forgiving matching.
  - Attempt field-type inference for every stored field — rejected for planning
    because it adds complexity not required to preserve current workflows.

## Decision 4: Replace Lucene-only bracket ranges with comparison-style KQL ranges

- **Decision**: The supported language will use comparison operators such as
  `>`, `>=`, `<`, and `<=` for numeric and timestamp ranges instead of exposing
  Lucene bracket syntax as a supported form.
- **Rationale**: Official KQL guidance favors comparison-style range filters, and
  the feature spec explicitly removes Lucene details from the public contract.
  This is a meaningful migration difference that documentation, error handling,
  and quickstart examples must surface.
- **Alternatives considered**:
  - Keep both comparison operators and bracket ranges permanently — rejected
    because it leaves Lucene details in the user-facing contract.
  - Remove range support entirely during migration — rejected because range
    filtering is already part of shipped functionality and required feature
    parity.

## Decision 5: Add field existence checks as the primary KQL-native capability

- **Decision**: Add field existence queries such as `field:*` to the supported
  Peek KQL subset and make them discoverable in autocomplete and docs.
- **Rationale**: Official KQL documentation defines `field:*` as a standard way
  to filter for present values, and Peek's `FieldInfo` plus `/fields` surfaces
  make this especially useful for semi-structured logs where users often need to
  find records that carry a field before refining by value.
- **Alternatives considered**:
  - Add multiple new KQL-only capabilities at once — rejected because the plan
    should keep the migration bounded and user-learnable.
  - Defer all KQL-native capability additions — rejected because existence checks
    are low-risk and directly useful for Peek's log-exploration use case.

## Decision 6: Migrate saved queries with deterministic reuse rules and corrective guidance

- **Decision**: Preserve history and starred-query data, auto-accept stored
  strings that remain valid in the supported KQL subset, deterministically
  rewrite trivial legacy forms where safe, and flag non-convertible Lucene-only
  entries with corrective guidance instead of silently dropping them.
- **Rationale**: Saved queries are user data, and the current UI already persists
  them in browser storage: history entries carry `query`, `lastUsedAt`, and
  `useCount`, while starred queries are stored as raw strings. A hard reset
  would damage trust, while a hidden dual-language parser would keep Lucene
  alive in practice.
- **Alternatives considered**:
  - Clear all existing history and starred queries — rejected because it throws
    away user intent and violates the spec's migration trust goals.
  - Keep a hidden Lucene fallback for stored queries — rejected because it would
    undermine the goal of removing Lucene as the supported language.

## Decision 7: Align invalid-query behavior across HTTP, WebSocket, and UI surfaces

- **Decision**: Keep the existing API shapes but standardize invalid-query
  behavior so HTTP requests, live subscriptions, and UI error states all expose
  corrective migration guidance instead of today’s mixed behavior.
- **Rationale**: The current HTTP path returns an explicit invalid-query error,
  while the WebSocket path logs and continues silently. The migration must make
  failure behavior predictable for the user-facing product.
- **Alternatives considered**:
  - Preserve current inconsistency — rejected because it hides query failures in
    live workflows.
  - Introduce a second out-of-band error channel — rejected because stable
    envelopes are more valuable than extra protocol complexity.

## Decision 8: Keep external API shapes stable while updating the query contract

- **Decision**: Preserve the `/query` request and response shape, `/fields`
  output, and WebSocket subscribe envelope while changing only the meaning of the
  `query` string to the documented Peek KQL subset.
- **Rationale**: The current server surfaces are already used by the embedded UI,
  test helpers, and screenshot tooling. Stable envelopes minimize migration risk
  while still allowing the user-facing language to change.
- **Alternatives considered**:
  - Introduce versioned endpoints for KQL — rejected because it expands the
    implementation and documentation surface without clear user value.
  - Change `/fields` to a brand-new schema — rejected because current field and
    top-value data are already sufficient for the planned autocomplete behavior.

## Decision 9: Keep verification centered on repo-native Go and Playwright coverage

- **Decision**: Treat parser behavior, `/query` and `/logs` behavior, editor
  highlighting, autocomplete, click-to-filter, history migration, and time-range
  interplay as mandatory automated acceptance coverage using the repo's `mise`
  commands.
- **Rationale**: The constitution requires repo-native verification, and this
  feature changes both backend semantics and user-visible editor behavior. The
  existing search-related Playwright suite already provides the right surface to
  extend.
- **Alternatives considered**:
  - Rely only on unit tests — rejected because the current search contract is
    highly UI-visible.
  - Rely only on manual exploratory QA — rejected because the feature changes a
    public contract and needs repeatable acceptance coverage.
