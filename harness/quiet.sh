#!/bin/sh
# quiet.sh — run a command and print one line on success, only the failure on failure.
#
# Usage: harness/quiet.sh <label> <command> [args...]
#
# Every make target, hook step and CI step goes through this wrapper so that
# an agent reading the output sees "ok: <label>" or "FAIL: <label>" followed
# by the diagnostics, and nothing else. Set ORCHESTRA_VERBOSE=1 to see everything.
set -u

label=$1
shift

if [ "${ORCHESTRA_VERBOSE:-0}" = "1" ]; then
  echo "run: $label"
  exec "$@"
fi

out=$(mktemp "${TMPDIR:-/tmp}/orchestra-quiet.XXXXXX")

"$@" >"$out" 2>&1
status=$?

if [ "$status" -eq 0 ]; then
  echo "ok: $label"
  rm -f "$out"
  exit 0
fi

echo "FAIL: $label (exit $status)"

# Strip the progress noise of the tools so only diagnostics remain.
sed -e 's/\x1b\[[0-9;]*m//g' "$out" \
  | grep -vE '^(ok[[:space:]]+[^[:space:]]+[[:space:]]+[0-9.]+s|\?[[:space:]]+.*\[no test files\]|PASS$|=== RUN|--- PASS|=== PAUSE|=== CONT)' \
  | grep -vE '^[[:space:]]*(RUN[[:space:]]+v[0-9]|Start at|Duration|Checking formatting|Finished in|validating .* using lint rules|\[STARTED\]|\[COMPLETED\]|Legend:)' \
  | grep -vE '^(cd |make(\[[0-9]+\])?: (Entering|Leaving) directory)' \
  | grep -vE '^[[:space:]]*$'

rm -f "$out"
exit "$status"
