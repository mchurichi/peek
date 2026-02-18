#!/usr/bin/env bash
#
# Run all e2e specs sequentially.
# Usage:  ./e2e/run.sh            (all specs)
#         ./e2e/run.sh table      (just table.spec.mjs)
#
set -euo pipefail
cd "$(dirname "$0")/.."

# Kill any lingering peek process on the test port
pkill -9 -f "peek.*--port 9997" 2>/dev/null || true
sleep 1

specs=("$@")
if [[ ${#specs[@]} -eq 0 ]]; then
  specs=(e2e/*.spec.mjs)
else
  specs=("${specs[@]/#/e2e/}")
  specs=("${specs[@]/%/.spec.mjs}")
fi

total_pass=0
total_fail=0

for spec in "${specs[@]}"; do
  echo ""
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "▶ Running: $(basename "$spec")"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  if node "$spec"; then
    ((total_pass++))
  else
    ((total_fail++))
  fi
  # Ensure port is freed between specs
  pkill -9 -f "peek.*--port 9997" 2>/dev/null || true
  sleep 1
done

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Specs: $((total_pass + total_fail)) total, ${total_pass} passed, ${total_fail} failed"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

exit "$total_fail"
