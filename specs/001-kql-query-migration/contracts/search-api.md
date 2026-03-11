# Contract: Search API and Live Query Surfaces

## Purpose

Define the stable external interfaces that carry query text during the KQL
migration.

## HTTP Query Contract

### `POST /query`

**Request shape**

```json
{
  "query": "level:ERROR and service:api",
  "limit": 100,
  "offset": 0,
  "start": "2026-03-11T00:00:00Z",
  "end": "2026-03-11T01:00:00Z"
}
```

**Response shape**

```json
{
  "logs": [],
  "total": 0,
  "took_ms": 0
}
```

**Contract rules**

- The request and response envelope remains stable during the migration.
- Empty `query` input continues to behave as match-all.
- Invalid KQL or unsupported legacy syntax returns corrective feedback through
  the existing error path without changing the envelope.
- `start` and `end` remain optional and orthogonal to the query-language text.

### Invalid query response behavior

- Invalid or unsupported query text must produce a user-correctable failure.
- Error feedback must explain whether the failure is invalid KQL, incomplete
  syntax, or unsupported legacy Lucene syntax.
- Migration guidance must include the supported KQL direction when a legacy form
  is rejected.

## Fields Contract

### `GET /fields`

**Response shape**

```json
{
  "fields": [
    {
      "name": "service",
      "type": "string",
      "top_values": ["api", "worker"]
    }
  ]
}
```

**Contract rules**

- Field metadata remains the source of truth for autocomplete and value
  suggestions.
- The migration can extend how the UI uses these fields, but the endpoint stays
  stable.

## WebSocket Contract

### `WS /logs`

**Subscribe message shape**

```json
{
  "action": "subscribe",
  "query": "level:ERROR and service:api",
  "start": "2026-03-11T00:00:00Z",
  "end": "2026-03-11T01:00:00Z"
}
```

**Unsubscribe message shape**

```json
{
  "action": "unsubscribe"
}
```

**Contract rules**

- The subscribe envelope remains stable while the meaning of `query` moves to
  the supported Peek KQL subset.
- Live filtering intent must stay aligned with one-time `/query` execution.
- Time-range bounds remain separate from the query-language string.
- Invalid query text during subscription must surface a client-visible error
  outcome instead of failing silently.

## Persisted Query Contract

- Query history and starred queries remain user-visible persisted state.
- Stored query text can be `native`, `rewritten`, or `needs-attention`, but it
  must remain inspectable and recoverable in the UI.
- UI-generated queries, including click-to-filter and autocomplete insertion,
  must remain valid for the supported contract above.
