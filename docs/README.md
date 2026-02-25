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

## Datetime Sliding Behavior

- Relative presets (`15m`, `1h`, `6h`, `24h`, `7d`) use a single query/subscribe setup, then slide client-side.
- Sliding is maintained by pruning stale rows in the UI every 1 second.
- No periodic HTTP `/query` polling is used for sliding windows.
- A 2-minute grace (`120000ms`) is applied before eviction to reduce drops caused by clock skew or late-arriving events.
- Fixed presets (`all`, `today`, `yesterday`, `custom`) do not use sliding pruning.

## Performance

- **Collect**: 1K+ logs/sec
- **Query**: 100K logs in <500ms
- **Storage**: Efficient compression with BadgerDB
- **Binary**: <20MB

## Developer Test Data Generator

This section is for development/testing workflows only; end-user usage stays in `/README.md`.

Use `e2e/loggen.mjs` to generate structured logs with varied levels, levelless rows, and mixed fields for local testing.

### Examples

```bash
# Generate 200 mixed-format logs to stdout
node e2e/loggen.mjs

# Same via npm script
npm run logs:gen -- --count 200

# Generate a finite batch and ingest directly into peek
node e2e/loggen.mjs --count 500 --format mixed | go run ./cmd/peek --all --no-browser

# Rate-limited generation (50 logs/sec)
node e2e/loggen.mjs --count 300 --rate 50 --format json

# Continuous stream (until Ctrl+C)
node e2e/loggen.mjs --follow --rate 20 --format mixed | go run ./cmd/peek --all --no-browser

# Write logs to file and replay later
node e2e/loggen.mjs --count 1000 --out /tmp/peek-sample.log
cat /tmp/peek-sample.log | go run ./cmd/peek --all --no-browser
```

### Options

```text
--count <n>                Number of logs to emit in finite mode (default: 200)
--rate <n>                 Fixed emit rate in logs/sec
--follow                   Stream continuously until Ctrl+C
--format <mixed|json|logfmt>
                           Output format (default: mixed)
--profile <feature>        Data profile (default: feature)
--out <path>               Write output to file (default: stdout)
--seed <n|string>          Deterministic seed for repeatable datasets
--help                     Show usage
```

## Release Automation

Peek uses label-driven release automation in GitHub Actions:

- `release:patch`, `release:minor`, `release:major`, or `skip-release` must be set on every PR (exactly one).
- CI validates label policy and posts a suggestion comment.
- Recommended Conventional Commit types for PR titles/commits: `feat`, `fix`, `docs`, `chore`, `refactor`, `test`, `perf`, `ci`, `build`, `style`, `revert` (optionally with scope, and `!` for breaking changes).
- Suggestion priority is:
  1. PR title conventional prefix (`feat:`, `feat!:`, `fix:`, `perf:`, `refactor:`, `revert:`) or `BREAKING CHANGE` marker
  2. Conventional commit messages in the PR
  3. Default to `skip-release`
- Merged PRs to `main` auto-create a SemVer tag unless `skip-release` is selected.
- Tag pushes (`v*.*.*`) trigger GoReleaser to publish GitHub Releases with multi-platform binaries and checksums.

## Roadmap

### Phase 2 (Future)
- UI support for multiple collectors
- Log export/download
- TLS/HTTPS support
- Additional log formats
