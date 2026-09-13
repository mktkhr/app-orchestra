#!/bin/sh
# Suppression guard.
#
# Static checks are only worth something when they cannot be silenced ad hoc.
# This script fails when a suppression directive exists in the source tree
# without being registered in harness/quality/suppressions.allow. Registering one is a
# reviewed change to the Repository Harness, not something done in passing.
#
# Covered directives:
#   Go          //nolint (any form)
#   TypeScript  @ts-ignore, @ts-nocheck, @ts-expect-error,
#               eslint-disable*, oxlint-disable*, biome-ignore, prettier-ignore, oxfmt-ignore
#   Coverage    istanbul ignore, c8 ignore, v8 ignore
#
# Coverage directives are here because AGENTS.md rule 2 names "//nolint,
# oxlint-disable, @ts-ignore and friends", and a line that excuses itself
# from the coverage floor is the same act as a line that excuses itself
# from the linter: the check still runs, and this one file stops being
# measured by it. One reached main before this was noticed, on a branch
# the author had decided was unreachable - and an unreachable branch is one
# to delete, not to exempt.
#
# Registry line format (one per line, '#' starts a comment). The source line is
# part of the key so that unrelated edits shifting line numbers do not matter,
# while any change to the suppressed line itself must be re-registered:
#   <path relative to repository root> :: <directive> :: <trimmed source line>
set -eu

cd "$(dirname "$0")/../.."

registry="harness/quality/suppressions.allow"
pattern='//nolint|@ts-ignore|@ts-nocheck|@ts-expect-error|eslint-disable|oxlint-disable|biome-ignore|prettier-ignore|oxfmt-ignore|istanbul ignore|c8 ignore|v8 ignore'

# Paths that are product or tooling source. node_modules and build output are never scanned.
scan_paths="services web/src web/vite.config.ts e2e harness vite.config.ts"

found=$(grep -rnE --include='*.go' --include='*.ts' --include='*.tsx' --include='*.js' --include='*.mjs' \
  --exclude-dir=node_modules --exclude-dir=dist \
  "$pattern" $scan_paths 2>/dev/null \
  | grep -vE '^(harness/quality/suppressions.allow|harness/guard/suppressions\.sh):' \
  || true)

status=0
count=0

# Each hit must appear in the registry with the same file, line and directive.
printf '%s\n' "$found" | while IFS= read -r hit; do
  [ -z "$hit" ] && continue
  file=${hit%%:*}
  rest=${hit#*:}
  line=${rest%%:*}
  code=${rest#*:}
  code=$(printf '%s' "$code" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')
  directive=$(printf '%s' "$rest" | grep -oE "$pattern" | head -n 1)
  key="$file :: $directive :: $code"
  if ! grep -qxF "$key" "$registry" 2>/dev/null; then
    echo "unregistered suppression: $hit"
    echo "  register it as '$key' in $registry (with a reason) or, better, fix the finding"
    echo "$key" >> "${TMPDIR:-/tmp}/app-orchestra-suppressions.$$"
  fi
done

if [ -s "${TMPDIR:-/tmp}/app-orchestra-suppressions.$$" ]; then
  count=$(wc -l < "${TMPDIR:-/tmp}/app-orchestra-suppressions.$$")
  rm -f "${TMPDIR:-/tmp}/app-orchestra-suppressions.$$"
  echo "suppressions: $count unregistered directive(s)"
  exit 1
fi
rm -f "${TMPDIR:-/tmp}/app-orchestra-suppressions.$$"

# Registered entries must still exist, otherwise the registry rots.
grep -vE '^[[:space:]]*(#|$)' "$registry" | while IFS= read -r key; do
  file=${key%% :: *}
  code=${key##* :: }
  if ! grep -qF -- "$code" "$file" 2>/dev/null; then
    echo "stale registry entry: $key (no such line in $file)"
    echo "stale" >> "${TMPDIR:-/tmp}/app-orchestra-suppressions-stale.$$"
  fi
done

if [ -s "${TMPDIR:-/tmp}/app-orchestra-suppressions-stale.$$" ]; then
  rm -f "${TMPDIR:-/tmp}/app-orchestra-suppressions-stale.$$"
  exit 1
fi
rm -f "${TMPDIR:-/tmp}/app-orchestra-suppressions-stale.$$"

total=$(printf '%s\n' "$found" | grep -c . || true)
echo "suppressions: $total directive(s), all registered"
exit $status
