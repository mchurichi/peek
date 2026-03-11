# Feature Specification: KQL Query Language Migration

**Feature Branch**: `001-kql-query-migration`  
**Created**: 2026-03-11  
**Status**: Draft  
**Input**: User description: "Migrate the query language to KQL. Remove all lucene details and implementation. Maintain feature parity with whats already supported by Lucene, and add more features that make sense for peek use case. Make sure UI is updated accordingly, so syntax highlight, completion, filtering, works as today, and adds new features as part of this spec. Update documentation accordingly. Ensure new spec if extensively tested."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Search Logs with KQL (Priority: P1)

As a Peek user investigating live or stored logs, I want to search with a
documented KQL-based query language so I can express the filters I already rely
on today while gaining clearer structured filtering for semi-structured log
data.

**Why this priority**: Querying logs is the core value of Peek. If the
migration breaks core search tasks, the feature fails regardless of other UI or
documentation improvements.

**Independent Test**: A user can run KQL queries for free text, field filters,
quoted phrases, boolean combinations, grouped clauses, wildcard matches,
comparison-based ranges, and field existence checks against a representative log
dataset, and the returned log set matches the intended filter behavior.

**Acceptance Scenarios**:

1. **Given** a dataset containing logs from multiple services and levels,
   **When** the user runs a structured KQL filter such as `level: ERROR and
   service: api`, **Then** Peek shows only the matching log entries.
2. **Given** logs with message text, numeric values, and timestamps, **When**
   the user runs phrase, wildcard, comparison, or existence filters, **Then**
   Peek returns the matching records without requiring a second query language.
3. **Given** a user enters a legacy Lucene-only construct that is no longer part
   of the supported language, **When** they execute the query, **Then** Peek
   shows a corrective message that explains the issue and points them toward the
   supported KQL form.

---

### User Story 2 - Compose Queries Confidently in the UI (Priority: P2)

As a Peek user composing searches in the dashboard, I want syntax highlighting,
autocomplete, click-to-filter, and saved-query flows to understand KQL so I can
build correct filters quickly without leaving the interface.

**Why this priority**: Peek's query bar is already a guided editing experience.
If the language changes without updating those affordances, migration friction
will be high even if the backend behavior is correct.

**Independent Test**: A user can create and refine KQL queries through typing,
keyboard-driven autocomplete, field-value clicks, query history, starred
queries, and time controls, and every generated query remains valid KQL for the
supported Peek subset.

**Acceptance Scenarios**:

1. **Given** the user is typing a KQL query, **When** they pause or navigate the
   suggestion list, **Then** Peek highlights KQL tokens and offers relevant
   field, operator, value, and existence suggestions without losing caret or
   scroll alignment.
2. **Given** the user clicks a field value from a log detail panel, **When**
   Peek appends that filter to the current search, **Then** the resulting query
   is valid KQL, preserves the existing query intent, and keeps the current
   browsing position stable.
3. **Given** the user reopens recent or starred searches after the migration,
   **When** they load one into the editor, **Then** Peek either executes it as a
   valid KQL query or clearly marks that it needs migration help before it can
   run.

---

### User Story 3 - Learn and Trust the Migration (Priority: P3)

As a returning Peek user, I want documentation, examples, and migration guidance
to teach the new query language clearly so I can move from Lucene terminology to
KQL without guesswork.

**Why this priority**: The migration changes a visible part of the product
contract. Users need consistent help text and examples to trust that the new
language is the supported path forward.

**Independent Test**: A user can learn the new syntax from built-in examples and
updated documentation, rewrite common legacy searches using the migration guide,
and confirm that Lucene-centric wording is no longer presented as the active
query language.

**Acceptance Scenarios**:

1. **Given** a user reads query examples in the product or documentation,
   **When** they look for search syntax guidance, **Then** they see KQL-first
   examples and a short migration reference for common legacy patterns.
2. **Given** a user has existing search habits from the Lucene-based version,
   **When** they encounter an outdated pattern, **Then** Peek explains the KQL
   equivalent instead of leaving them with a generic parse error.

---

### Edge Cases

- What happens when a user enters adjacent clauses without an explicit boolean
  operator and expects today's implicit narrowing behavior?
- How does the system handle values that include spaces, quotes, URLs, colons,
  or stack-trace punctuation?
- What happens when a user applies comparison or multi-value filters to fields
  whose values cannot be compared consistently?
- How does the system handle queries against missing fields, levelless log
  entries, or empty result sets?
- What happens when a stored history or starred query contains legacy syntax
  that cannot be converted one-to-one into the supported KQL subset?
- How does the system behave when a user combines query-language time clauses
  with the existing time preset or custom time-range controls?
- What happens while a user is still typing an incomplete clause, unmatched
  parenthesis, or unfinished quoted value?

## Requirements *(mandatory)*

### Constitution Alignment

- **Delivery Impact**: The migration preserves Peek's single-binary,
  local-first experience and replaces Lucene as the user-facing search language
  without introducing a second primary search mode.
- **Data/Query Impact**: Stored log data and log ingestion remain unchanged, but
  the visible search contract moves to a bounded Peek KQL subset across manual
  searches and live filtering flows.
- **UI Impact**: The query editor, syntax highlighting, autocomplete,
  click-to-filter, query history, starred queries, time controls, and scroll
  preservation all remain part of the experience while adopting KQL syntax.
- **Docs & Release Impact**: Consumer docs, technical docs, screenshots,
  examples, and release notes switch to KQL terminology and include migration
  guidance for existing users.

### Functional Requirements

- **FR-001**: Peek MUST adopt KQL as the only documented and supported query
  language for all user-visible search entry points.
- **FR-002**: Peek MUST preserve the current supported search workflows for
  equivalent KQL queries, including blank or match-all searches, free-text
  search, fielded filters, quoted phrases, boolean logic, grouped clauses,
  wildcard matching, and numeric or timestamp filtering.
- **FR-003**: Peek MUST define and document a bounded "Peek KQL subset" so
  users know exactly which KQL behaviors are supported and which Lucene-only
  constructs are no longer valid.
- **FR-004**: Peek MUST support free-text search across the same user-visible
  log content that can be searched today, without requiring users to specify a
  field for common troubleshooting queries.
- **FR-005**: Peek MUST support field-scoped KQL filters for common log fields,
  including `level`, `message`, `timestamp`, and dynamic structured fields.
- **FR-006**: Peek MUST support quoted phrase matching for ordered exact text
  searches and MUST preserve reliable searching for values that contain spaces or
  punctuation.
- **FR-007**: Peek MUST support case-insensitive boolean operators, grouped
  clauses with parentheses, and the existing user expectation that adjacent
  narrowing clauses behave as logical AND unless documented otherwise.
- **FR-008**: Peek MUST support wildcard matching for field values, including
  the currently working prefix, suffix, and infix patterns that users rely on
  today.
- **FR-009**: Peek MUST support comparison-style numeric and time filters using
  KQL-friendly operators such as `>`, `>=`, `<`, and `<=`, including relative
  time expressions that make sense for log investigation.
- **FR-010**: Peek MUST support field existence queries so users can find logs
  where a field is present even when they do not know the exact value.
- **FR-011**: Peek MUST support matching multiple acceptable values for the same
  field within a single query so users can express common workflows such as
  "error or warning" without duplicating the full field name.
- **FR-012**: Peek MUST clearly define how unquoted string matching behaves in
  the supported KQL subset so users can predict whether Peek performs exact,
  partial, or analyzed matching for ordinary field:value filters.
- **FR-013**: Peek MUST provide migration-aware error feedback for invalid KQL
  and unsupported legacy Lucene patterns, and the guidance MUST explain what the
  user can do next.
- **FR-014**: Peek MUST update syntax highlighting so the query editor visually
  distinguishes KQL fields, operators, phrases, wildcards, grouped clauses,
  comparison operators, existence checks, and error states.
- **FR-015**: Peek MUST update autocomplete so users can discover fields,
  values, boolean operators, comparison operators, and existence-oriented query
  building from within the editor.
- **FR-016**: Peek MUST ensure all UI-generated query text, including
  click-to-filter behavior and saved-query insertion, produces valid supported
  KQL.
- **FR-017**: Peek MUST preserve the usefulness of query history and starred
  queries across the migration by converting legacy entries when possible and by
  clearly flagging entries that require user correction when conversion is not
  possible.
- **FR-018**: Peek MUST keep time presets and custom time-range controls working
  as they do today; query-language migration MUST not remove or replace those
  controls.
- **FR-019**: Peek MUST keep filtering intent consistent between one-time query
  execution and live or subscribed result streams so the same KQL query narrows
  both experiences in the same way.
- **FR-020**: Peek MUST remove Lucene terminology from user-facing search help,
  examples, placeholders, screenshots, and release notes.
- **FR-021**: Peek MUST provide a concise migration reference that maps common
  legacy search patterns to their supported KQL equivalents.
- **FR-022**: Peek MUST be released with extensive automated acceptance coverage
  for parser behavior, result matching, invalid-query handling, syntax
  highlighting, autocomplete, click-to-filter, saved-query migration, time-range
  interplay, and live filtering behavior.

### Key Entities *(include if feature involves data)*

- **Query Expression**: A user-authored KQL search string that can contain free
  text, field clauses, boolean operators, phrases, wildcard clauses, comparison
  filters, and existence checks.
- **Query Suggestion**: A contextual hint shown while the user composes a query,
  including field names, operators, field values, and migration-oriented help.
- **Saved Query Entry**: A recent or starred search containing the displayed
  query text plus recency and reuse metadata, and optionally a migration status
  if the stored text needs user attention.
- **Result Set**: The collection of matching log entries returned for the active
  search, along with the visible total count and any user-facing query feedback.

## Assumptions

- Lucene is fully removed as a user-facing query language in this release rather
  than offered as a long-term fallback mode.
- The feature targets a bounded Peek-specific KQL subset rather than complete
  parity with every capability in Kibana or Elasticsearch.
- Existing time presets and custom time-range controls remain part of the
  product and continue to work alongside the migrated query language.
- Existing history and starred queries are treated as user data that should stay
  usable or recoverable after the migration.
- Lucene-only advanced features that Peek does not support today, such as fuzzy
  search or regular expressions, remain out of scope for this migration.

## Non-Goals

- Changing log ingestion formats, stored log structure, or retention behavior.
- Redesigning the broader dashboard layout, table model, or non-query UI
  workflows.
- Replacing time presets or custom time ranges with a query-language-only time
  experience.
- Expanding the feature into full Kibana or Elasticsearch query-language parity.
- Introducing separate query languages, user-selectable parser modes, or new
  query languages beyond the supported Peek KQL subset.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Every query workflow that Peek currently supports for end users
  has a documented KQL equivalent and passes the approved parity dataset review
  before release.
- **SC-002**: At least 95% of valid searches in the migration acceptance suite
  return results, an empty state, or a corrective response within 1 second on
  the reference dataset.
- **SC-003**: At least 90% of guided query-composition tasks in acceptance
  review can be completed using autocomplete, click-to-filter, or saved-query
  helpers without the user manually repairing generated syntax.
- **SC-004**: Zero Lucene terminology remains in user-facing query help,
  examples, placeholder text, or screenshots for the released feature.
- **SC-005**: All primary search journeys—manual query entry, invalid-query
  recovery, live filtering, saved-query reuse, and time-range interaction—have
  passing automated acceptance coverage before the feature is considered
  release-ready.
