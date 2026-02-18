---
description: 'Go conventions for peek — log collector CLI and storage engine'
applyTo: '**/*.go,**/go.mod,**/go.sum'
---

# Go Instructions for Peek

These instructions follow [Effective Go](https://go.dev/doc/effective_go) and idiomatic Go conventions, tailored to the peek codebase.

## Project Layout

- CLI entry point in `cmd/peek/main.go`
- Public packages in `pkg/` (parser, query, storage, server)
- Internal packages in `internal/` (config, web)
- Do not introduce new top-level directories without updating AGENTS.md

## Formatting

- Always use `gofmt` / `goimports` — never manually align code
- Use tabs for indentation (the `gofmt` default)
- No hard line length limit, but wrap long lines and indent continuations with an extra tab
- Opening brace of control structures (`if`, `for`, `switch`, `select`) must be on the same line — never on the next line (semicolon insertion will break it)

## Naming

- Use `MixedCaps` / `mixedCaps` — never underscores in names
- Exported names start uppercase, unexported start lowercase
- Package names are lowercase, single-word, no underscores or mixedCaps
- Avoid package name stuttering: `storage.Storage` is fine, `storage.StorageStore` is not — use `storage.Store`
- Getters: use `Owner()` not `GetOwner()`; setters: `SetOwner()`
- One-method interfaces use `-er` suffix: `Reader`, `Writer`, `Formatter`
- Don't reuse well-known method names (`Read`, `Write`, `Close`, `String`) unless the signature and semantics match

## Error Handling

- Check errors immediately after the call — never defer error checking
- Don't ignore errors with `_` unless documented why
- Wrap errors with context: `fmt.Errorf("context: %w", err)`
- Error messages are lowercase, no trailing punctuation
- Return early on error — keep the happy path left-aligned, eliminate unnecessary `else`
- Error return is always the last return value
- Create custom error types when callers need to inspect specific errors (type switches / `errors.As`)
- Library code should not `panic` — return errors instead. Use `panic` only for truly unrecoverable programmer errors (e.g., impossible states)

## Control Structures

- `if` and `switch` accept init statements — use them to scope variables: `if err := f(); err != nil { return err }`
- When the body ends in `break`, `continue`, `goto`, or `return`, omit the `else`
- Prefer `switch` over `if-else` chains — expressionless `switch` (`switch { case ...: }`) replaces `if-else-if`
- No automatic fallthrough in `switch` — use comma-separated cases: `case ' ', '?', '&':`
- Use `for range` over slices, maps, and channels; use blank identifier `_` to discard unwanted index/value
- Use labeled `break` / `continue` when breaking out of an outer loop from inside a `switch` or inner `for`

## Functions

- Use multiple return values — return `(result, error)` not in-band sentinel values
- Use named return parameters for documentation when it clarifies which value is which, but avoid bare `return` in long functions
- Use `defer` for resource cleanup (file close, mutex unlock) — it runs on all return paths
- Remember: deferred functions execute LIFO, and arguments are evaluated at the `defer` call, not at execution

## Data Structures

- Make the zero value useful — design structs so `new(T)` or `var t T` is ready to use without an `Init()` call
- Use `new(T)` for allocation when zero value suffices; use `make()` for slices, maps, and channels
- Prefer composite literals with named fields: `&Config{Port: 8080}` not `&Config{8080}`
- Use the comma-ok idiom for map lookups: `v, ok := m[key]`
- Use `append` — never manually manage slice growth
- Prefer slices over arrays; pass slices not array pointers

## Interfaces

- Keep interfaces small — one or two methods is ideal
- Accept interfaces, return concrete types
- Interfaces are satisfied implicitly — no `implements` keyword
- Use compile-time interface checks when there are no static conversions: `var _ Filter = (*FieldFilter)(nil)`
- Don't export a type if it only exists to implement an interface — export the interface, return it from constructors

## Concurrency

- Prefer channels for coordination; use mutexes only for simple shared counters or state where channels are overkill
- "Do not communicate by sharing memory; instead, share memory by communicating"
- Storage methods use `sync.RWMutex` — take `RLock` for reads, `Lock` for writes
- Never hold a lock across a BadgerDB transaction callback — acquire inside the callback or before entering
- WebSocket broadcast uses a channel-based worker, not direct writes from handlers
- Use buffered channels as semaphores to limit concurrency
- Launch goroutines only when the caller has a way to observe completion (channel, `sync.WaitGroup`, or context)

## Storage (BadgerDB)

- Primary key format: `log:{timestamp_nano}:{id}` — time-range optimizations depend on this; do not change
- All values are JSON-serialized `LogEntry` structs
- Use `t.TempDir()` for BadgerDB paths in tests — automatic cleanup, no collisions
- Run `db.RunValueLogGC(0.5)` after bulk deletes to reclaim disk space

## Query Engine

- All query filters implement the `Filter` interface: `Match(*LogEntry) bool`
- New query features must add a struct implementing `Filter` in `pkg/query/`
- Field resolution maps `level` → `entry.Level`, `message` → `entry.Message`, `timestamp` → `entry.Timestamp`; everything else checks `entry.Fields`

## Embedded Assets

- The production UI is embedded via `//go:embed` in `pkg/server/server.go`
- Do not add build steps, generated files, or binary assets that break the single-binary model

## Testing

- Standard `*_test.go` files alongside source
- Table-driven tests where applicable
- No external test dependencies — stdlib `testing` package only
- Run: `go test ./... -race -count=1`

## Comments & Documentation

- Doc comments on all exported types, functions, methods, and packages
- Doc comments start with the name of the thing: `// Store saves a log entry`
- Package comments: `// Package storage provides BadgerDB-backed log storage.`
- Use line comments (`//`) for most comments; block comments (`/* */`) mainly for package docs
- Document *why*, not *what*, unless the logic is non-obvious
- Use `String() string` method to control `%v` / `Println` output for custom types

## Dependencies

- Keep dependencies minimal — prefer the standard library
- Use Go modules (`go.mod` / `go.sum`)
- Avoid import `.` (dot imports)
- Side-effect imports use blank identifier: `import _ "net/http/pprof"`
