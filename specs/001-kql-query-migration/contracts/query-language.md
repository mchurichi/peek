# Contract: Peek KQL Query Language

## Purpose

Define the supported user-facing query language for Peek after the Lucene
migration.

## Supported Constructs

| Construct | Contract | Example |
|---|---|---|
| Match all | Empty input or `*` matches all visible logs | `*` |
| Free text | Bare text searches user-visible log content without requiring a field | `timeout` |
| Field filter | `field:value` narrows results to entries matching that field | `service:api` |
| Quoted phrase/value | Quotes preserve exact phrase or exact value intent | `message:"connection refused"` |
| Boolean operators | `and`, `or`, and `not` are case-insensitive | `level:ERROR and service:api` |
| Grouping | Parentheses control precedence | `(level:ERROR or level:WARN) and service:api` |
| Wildcards | `*` is supported inside field values | `message:*timeout*` |
| Comparison ranges | Numeric and timestamp fields support comparison operators | `duration_ms >= 500` |
| Exists | `field:*` matches entries where the field is present | `request_id:*` |
| Same-field multi-value | One field can match multiple accepted values in one grouped clause | `level:(ERROR or WARN)` |

## Unsupported or Migrated Legacy Forms

| Legacy form | Contract after migration | User feedback expectation |
|---|---|---|
| Lucene bracket ranges | Not supported as a documented query form | Explain the comparison-style KQL equivalent |
| Lucene-only advanced constructs not already in Peek's supported workflows | Not supported | Explain that the query uses an unsupported legacy pattern |
| Hidden Lucene fallback mode | Not available | Keep the user in the KQL workflow with corrective guidance |

## Matching Rules

- Blank input and `*` remain valid match-all queries.
- Quoted values preserve exact phrase or exact value intent.
- Ordinary unquoted string filters preserve Peek's current forgiving search
  behavior for troubleshooting rather than switching users to strict exact-only
  matching.
- Boolean operators are case-insensitive.
- Invalid or incomplete expressions surface corrective guidance instead of a
  silent failure.

## Migration Rules

- Common legacy patterns must have documented KQL equivalents in docs and UI
  guidance.
- Persisted history and starred queries remain visible after migration.
- Stored queries that cannot be reused directly must be flagged with migration
  help rather than dropped.

## UI Contract

- Syntax highlighting reflects the supported Peek KQL token set.
- Autocomplete can suggest fields, values, boolean operators, comparison
  operators, and existence-oriented completions.
- Click-to-filter actions always emit valid supported KQL.
- Error states identify unsupported legacy syntax and guide the user toward the
  supported form.
