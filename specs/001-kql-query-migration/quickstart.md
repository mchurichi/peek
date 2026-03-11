# Quickstart: KQL Query Language Migration

## Goal

Verify the KQL migration end to end after implementation without changing
Peek's single-binary, embedded-UI workflow.

## 1. Build and start Peek with sample data

```bash
mise exec -- go build -o peek ./cmd/peek
mise exec -- bash -lc 'node e2e/loggen.mjs --follow --rate 20 | ./peek --all --no-browser --port 8080'
```

Open `http://localhost:8080`.

## 2. Validate primary KQL workflows in the UI

Run these queries from the search bar and confirm the expected behavior:

- `level:ERROR and service:api`
- `message:"connection refused"`
- `message:*timeout*`
- `duration_ms >= 500 and duration_ms < 2000`
- `request_id:*`
- `level:(ERROR or WARN)`

Expected outcomes:

- Results load without leaving the search UI.
- Syntax highlighting reflects KQL fields, operators, wildcards, and error
  states.
- Field and value autocomplete still works.
- Clicking a field value appends a valid KQL filter and preserves scroll.
- Time presets and custom time ranges still narrow results independently of the
  query text.

## 3. Validate migration guidance

Try a legacy query form that is no longer supported, such as a Lucene bracket
range.

Expected outcomes:

- Peek does not silently accept unsupported legacy syntax.
- The user sees corrective guidance that points to the supported KQL form.
- Saved or starred queries with legacy text remain visible and recoverable.

## 4. Run automated verification

```bash
mise exec -- go test ./...
mise exec -- go vet ./...
mise exec -- npx playwright test e2e/search.spec.mjs
mise exec -- npx playwright test e2e/search-caret.spec.mjs
mise exec -- npx playwright test e2e/field-filter-append.spec.mjs
mise exec -- npx playwright test e2e/query-history.spec.mjs
mise exec -- npx playwright test e2e/datetime.spec.mjs
mise exec -- npx playwright test e2e/copy.spec.mjs
mise exec -- npx playwright test e2e/levelless.spec.mjs
mise exec -- npx playwright test e2e/sliding-window.spec.mjs
```

## 5. Verify documentation surfaces

- `README.md` presents KQL-first query examples.
- `docs/README.md` reflects any changed query examples or migration notes.
- `AGENTS.md` no longer describes the search bar as Lucene-based.
- Screenshot generation inputs and captions no longer reference Lucene.
