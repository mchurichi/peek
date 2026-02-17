# Peek

A minimalist, dev-first CLI log collector and web UI. Pipe logs into `peek`, store them locally, and query them through a real-time web dashboard.

**Status:** Phase 1 MVP Complete

## Features

- 🚀 **Single binary** - No external dependencies
- 📊 **JSON & slog support** - Auto-detects log formats
- 💾 **Local storage** - BadgerDB with configurable retention
- 🔍 **Lucene queries** - Powerful search syntax
- ⚡ **Real-time updates** - WebSocket streaming
- 🎨 **Web UI** - Clean, minimal interface
- ⚙️ **Configurable** - TOML config + CLI flags

## Installation

```bash
# Build from source
git clone https://github.com/mchurichi/peek.git
cd peek
go build -o peek ./cmd/peek

# Or install directly
go install github.com/mchurichi/peek/cmd/peek@latest
```

## Quick Start

### Collect Logs

Pipe logs from any source:

```bash
# From a file
cat application.log | peek

# From a running process
docker logs my-container | peek

# From kubectl
kubectl logs my-pod -f | peek
```

### View Logs

Start the web UI:

```bash
peek server
```

The browser will auto-open to `http://localhost:8080`

## Usage

### Collect Mode

```bash
cat app.log | peek [OPTIONS]

Options:
  --db-path PATH         Database path (default: ~/.peek/db)
  --retention-size SIZE  Max storage (e.g., 1GB, 500MB)
  --retention-days DAYS  Max age of logs (default: 7)
  --format FORMAT        auto | json | slog (default: auto)
```

### Server Mode

```bash
peek server [OPTIONS]

Options:
  --db-path PATH    Database path (default: ~/.peek/db)
  --port PORT       HTTP port (default: 8080)
  --no-browser      Don't auto-open browser
```

## Query Syntax

Peek supports ElasticSearch Lucene query syntax:

```
# Keyword search
error timeout

# Field-based queries
level:ERROR
service:api
user_id:123

# Boolean operators
level:ERROR AND service:api
level:ERROR OR level:WARN
NOT level:DEBUG

# Wildcards
message:*timeout*
service:api*

# Quoted phrases
message:"connection refused"

# Complex queries
(level:ERROR OR level:CRITICAL) AND service:api
```

## Log Formats

Peek supports JSON and Go slog formats with auto-detection:

### JSON Format
```json
{
  "timestamp": "2026-02-17T10:30:45Z",
  "level": "ERROR",
  "message": "Connection timeout",
  "service": "api",
  "attempt": 3
}
```

### Slog Format
```json
{
  "time": "2026-02-17T10:30:45Z",
  "level": "ERROR",
  "msg": "Connection timeout",
  "service": "api",
  "attempt": 3
}
```

## Configuration

Default config location: `~/.peek/config.toml`

```toml
[storage]
retention_size = "1GB"
retention_days = 7
db_path = "~/.peek/db"

[server]
port = 8080
auto_open_browser = true

[parsing]
format = "auto"
auto_timestamp = true

[logging]
level = "info"
```

CLI flags override config file values.

## Architecture

```
┌──────────────────────────────────────┐
│  cat app.log | peek                  │
│  Collect Mode (stdin)                │
└──────────────────┬───────────────────┘
                   │
                   ├─ Parse (JSON/slog)
                   ├─ Validate & structure
                   └─ Write to Badger
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
        │ localhost:8080       │
        ├──────────────────────┤
        │ HTTP API:            │
        │ - GET /health        │
        │ - WS /logs           │
        │ - POST /query        │
        │ - GET /stats         │
        ├──────────────────────┤
        │ Web UI:              │
        │ - Search (Lucene)    │
        │ - Live log stream    │
        │ - Real-time updates  │
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

## Examples

### Collect from multiple sources

```bash
# Collect from application
cat app.log | peek

# Start server and view
peek server

# Collect more logs (same database)
cat another-app.log | peek
```

### Filter and search

```bash
# In the web UI:
level:ERROR                    # Show only errors
service:api                    # Filter by service
level:ERROR AND service:auth   # Combine filters
message:*timeout*              # Wildcard search
```

## Development

### Requirements

- Go 1.21+
- BadgerDB v4
- Gorilla WebSocket

### Build

```bash
go build -o peek ./cmd/peek
```

### Project Structure

```
peek/
├── cmd/peek/           # Main entry point
├── pkg/
│   ├── parser/         # Log format parsers
│   ├── storage/        # BadgerDB storage layer
│   ├── query/          # Lucene query engine
│   └── server/         # HTTP server & WebSocket
├── internal/
│   ├── config/         # Configuration management
│   └── web/            # Web UI (embedded)
└── go.mod
```

## Performance

- **Collect**: 1K+ logs/sec
- **Query**: 100K logs in <500ms
- **Storage**: Efficient compression with BadgerDB
- **Binary**: <20MB

## Roadmap

### Phase 2 (Future)
- Multiple collectors support
- Log export/download
- Advanced analytics
- TLS/HTTPS support

### Phase 3 (Future)
- Multi-user support
- Authentication/authorization
- Additional log formats
- Advanced visualizations

## Contributing

Contributions welcome! Please open an issue first to discuss changes.

## License

MIT License - See LICENSE file for details

## Credits

Built with:
- [BadgerDB](https://github.com/dgraph-io/badger) - Embedded key-value database
- [Gorilla WebSocket](https://github.com/gorilla/websocket) - WebSocket library
- [BurntSushi/toml](https://github.com/BurntSushi/toml) - TOML parser

---

**Local-first. Security-first. Minimal. Modular.**
