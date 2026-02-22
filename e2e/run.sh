#!/usr/bin/env bash
#
# Thin compatibility wrapper for Playwright Test.
# Usage:  ./e2e/run.sh                    (all specs, headed)
#         ./e2e/run.sh --headless         (all specs, headless)
#         ./e2e/run.sh --parallel 3       (all specs, 3 workers)
#         ./e2e/run.sh table              (just table.spec.mjs)
#         ./e2e/run.sh --headless table   (just table.spec.mjs, headless)
#
set -euo pipefail
cd "$(dirname "$0")/.."

spec_args=()
pw_args=()
parallel=""
needs_value=""

map_spec_arg() {
  local arg="$1"

  if [[ -f "e2e/${arg}.spec.mjs" ]]; then
    echo "e2e/${arg}.spec.mjs"
    return 0
  fi

  if [[ -f "$arg" ]]; then
    echo "$arg"
    return 0
  fi

  if [[ -f "e2e/$arg" ]]; then
    echo "e2e/$arg"
    return 0
  fi

  return 1
}

while [[ $# -gt 0 ]]; do
  if [[ -n "$needs_value" ]]; then
    pw_args+=("$1")
    needs_value=""
    shift
    continue
  fi

  case "$1" in
    --headless)
      export HEADLESS=1
      shift
      ;;
    --parallel)
      if [[ $# -lt 2 ]]; then
        echo "error: --parallel requires a worker count" >&2
        exit 2
      fi
      parallel="$2"
      shift 2
      ;;
    --parallel=*)
      parallel="${1#*=}"
      shift
      ;;
    --)
      shift
      pw_args+=("$@")
      break
      ;;
    --grep|--grep-invert|--project|--config|--reporter|--shard|--retries|--timeout|--workers|--max-failures)
      pw_args+=("$1")
      needs_value=1
      shift
      ;;
    -*)
      pw_args+=("$1")
      shift
      ;;
    *)
      mapped="$(map_spec_arg "$1" || true)"
      if [[ -z "$mapped" ]]; then
        echo "error: spec not found: $1" >&2
        exit 2
      fi
      spec_args+=("$mapped")
      shift
      ;;
  esac
done

if [[ -n "$parallel" ]]; then
  if ! [[ "$parallel" =~ ^[1-9][0-9]*$ ]]; then
    echo "error: --parallel must be a positive integer (got: $parallel)" >&2
    exit 2
  fi
  pw_args+=("--workers" "$parallel")
fi

cmd=(npx playwright test)
cmd+=("${pw_args[@]}")
if [[ ${#spec_args[@]} -gt 0 ]]; then
  cmd+=("${spec_args[@]}")
fi

exec "${cmd[@]}"
