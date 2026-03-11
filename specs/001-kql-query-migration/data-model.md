# Data Model: KQL Query Language Migration

## Query Expression

- **Purpose**: Represents the active user-authored search text executed against
  stored and live log data.
- **Fields**:
  - `rawText`: Original query text as entered or generated in the UI.
  - `normalizedText`: Canonical form used for execution, history reuse, and
    deterministic comparison.
  - `clauses`: Ordered list of free-text, field, phrase, wildcard, comparison,
    existence, and grouped boolean clauses.
  - `matchMode`: `match-all`, `free-text`, or `structured`.
  - `validationState`: `valid`, `incomplete`, `invalid`, or `legacy-unsupported`.
  - `diagnosticMessage`: User-facing explanation when validation is not `valid`.
- **Validation rules**:
  - Blank text or `*` resolves to `match-all`.
  - Quotes and parentheses must balance before the expression becomes `valid`.
  - Unsupported Lucene-only constructs move the expression to
    `legacy-unsupported` with migration guidance.
  - Comparison operators apply only to values that can be compared consistently.
- **State transitions**:
  - `incomplete -> valid` when required tokens close correctly.
  - `incomplete -> invalid` when the expression cannot be repaired by normal
    completion rules.
  - `legacy-unsupported -> valid` when the user accepts or enters a supported
    KQL equivalent.

## Query Suggestion

- **Purpose**: Drives autocomplete and guided query composition in the editor.
- **Fields**:
  - `kind`: `field`, `value`, `operator`, `comparison`, `exists`, or
    `migration-help`.
  - `label`: Visible dropdown text.
  - `insertText`: Text inserted into the editor on acceptance.
  - `replaceFrom`: Character offset where replacement begins.
  - `fieldName`: Field the suggestion belongs to, when applicable.
  - `source`: `fields-api`, `language-keyword`, or `migration-rule`.
- **Validation rules**:
  - Suggestions must be valid within the current editor context.
  - Value suggestions must match a known field.
  - Migration-help suggestions appear only when legacy syntax or invalid input is
    detected.

## Query History Entry

- **Purpose**: Preserves recent searches across sessions and drives reuse.
- **Fields**:
  - `query`: Stored query string shown to the user.
  - `lastUsedAt`: ISO timestamp of last successful use.
  - `useCount`: Number of times the query was run.
  - `migrationStatus`: `native`, `rewritten`, or `needs-attention`.
  - `migrationNote`: Optional short guidance when attention is required.
- **Validation rules**:
  - `query` must be non-empty and not just `*` to be persisted in history.
  - `migrationStatus=needs-attention` requires a `migrationNote`.
- **State transitions**:
  - `native -> rewritten` if a deterministic migration rule updates the stored
    query.
  - `native|rewritten -> needs-attention` if a later validation pass finds an
    unsupported legacy form.

**Current persistence note**: Existing browser storage persists history entries
as `{ query, lastUsedAt, useCount }` under `peek.queryHistory.v1`. Migration
state is a design addition for this feature and remains client-side.

## Starred Query Entry

- **Purpose**: Represents a user-pinned reusable query.
- **Fields**:
  - `query`: Stored query string.
  - `migrationStatus`: `native`, `rewritten`, or `needs-attention`.
  - `migrationNote`: Optional short explanation when the query needs user repair.
- **Validation rules**:
  - The starred query must remain displayable even when it needs migration help.
  - Entries requiring user attention cannot fail silently.

**Current persistence note**: Existing browser storage persists starred queries
as raw strings under `peek.starredQueries.v1`. If migration metadata is added,
it remains a client-side persistence concern.

## Query Request

- **Purpose**: Public filter request sent through HTTP and mirrored in WebSocket
  subscribe behavior.
- **Fields**:
  - `query`: Active query string using the supported Peek KQL subset.
  - `limit`: Maximum number of results to return.
  - `offset`: Pagination offset for full-query fetches.
  - `start`: Optional lower time bound from the time-range UI.
  - `end`: Optional upper time bound from the time-range UI.
- **Validation rules**:
  - `query` defaults to `*` when empty.
  - `start` and `end` remain optional and independent from query text.
  - Invalid query text returns corrective feedback without changing the request
    envelope.

## Query Result Set

- **Purpose**: Returned result payload for one-time query execution or initial
  subscription hydration.
- **Fields**:
  - `logs`: Matching `LogEntry` records.
  - `total`: Total count for the active query.
  - `tookMs`: Reported execution duration.
  - `statusMessage`: User-facing success, empty-state, or corrective feedback in
    the UI layer.
- **Validation rules**:
  - `logs` is always returned as a list, even when empty.
  - `statusMessage` is required for invalid-query and guided-migration states.

## Field Information

- **Purpose**: Powers field autocomplete and top-value suggestions.
- **Fields**:
  - `name`: Structured field name.
  - `type`: Stored field type label.
  - `topValues`: Most common values exposed for autocomplete.
- **Validation rules**:
  - `name` must remain stable across query migration work.
  - `topValues` must continue to support field-value completions and existence
    guidance.

## Relationships

- A `Query Expression` can produce zero or more `Query Suggestions` while the
  user types.
- A `Query Expression` is sent as a `Query Request` and yields a `Query Result
  Set`.
- `Query History Entry` and `Starred Query Entry` both reference stored query
  text and can be rehydrated into a live `Query Expression`.
- `Field Information` supports `Query Suggestion` generation and field existence
  workflows.
