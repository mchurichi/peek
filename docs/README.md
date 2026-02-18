# Peek Docs

This folder contains deeper documentation for Peek.

## Architecture

```
┌──────────────────────────────────────────────────┐
│  cat app.log | peek              (single process) │
│  Collect + Embedded Server                        │
├──────────────────┬───────────────────────────────┤
│  stdin → Parser  │  Embedded HTTP Server         │
│  ├─ JSON/logfmt  │  localhost:8080               │
│  ├─ Validate     │  ├─ GET /health               │
│  └─ Store ───────┼──├─ WS /logs (real-time push) │
│                  │  ├─ POST /query               │
│                  │  ├─ GET /stats                │
│                  │  └─ Web UI                    │
└──────────────────┴───────────────────────────────┘
                   │
                   ↓
        ┌──────────────────────┐
        │ Badger Database      │
        │ ~/.peek/db           │
        │ Retention enforced   │
        └──────────────────────┘
                   ↑
                   │
        ┌──────────────────────┐
        │ peek server          │
        │ (standalone mode)    │
        │ Browse collected logs│
        └──────────────────────┘
```

## API Endpoints

### GET /health
Health check endpoint
```json
{
  "status": "ok",
  "logs_stored": 12534,
  "db_size_bytes": 245235000
}
```

### GET /stats
Statistics endpoint
```json
{
  "total_logs": 12534,
  "db_size_mb": 234.5,
  "levels": {
    "ERROR": 245,
    "WARN": 1234,
    "INFO": 10320,
    "DEBUG": 735
  }
}
```

### POST /query
Execute a query
```json
{
  "query": "level:ERROR AND service:api",
  "limit": 100,
  "offset": 0
}
```

Response:
```json
{
  "logs": [...],
  "total": 5000,
  "took_ms": 45
}
```

### WS /logs
WebSocket endpoint for real-time log streaming

## Performance

- **Collect**: 1K+ logs/sec
- **Query**: 100K logs in <500ms
- **Storage**: Efficient compression with BadgerDB
- **Binary**: <20MB

## Roadmap

### Phase 2 (Future)
- UI support for multiple collectors
- Log export/download
- TLS/HTTPS support
- Additional log formats

