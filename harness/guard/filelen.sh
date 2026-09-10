#!/bin/sh
# File length guard.
#
# Fails when a hand-written source file is longer than the limit in
# harness/quality/file-length.txt. See that file for why the limit exists.
#
# Generated code is excluded by the glob list there, not by this script.
set -eu

cd "$(dirname "$0")/../.."

config="harness/quality/file-length.txt"
limit=$(awk '$1 == "limit" { print $2; exit }' "$config")

if [ -z "${limit:-}" ]; then
  echo "file-length: no 'limit' in $config"
  exit 1
fi

# The same source tree the other guards scan: product and tooling, never
# dependencies or build output.
scan_paths="services web/src e2e harness"

files=$(find $scan_paths \
  \( -name node_modules -o -name dist -o -name .cache -o -name test-results \) -prune -o \
  \( -name '*.go' -o -name '*.ts' -o -name '*.tsx' -o -name '*.js' -o -name '*.mjs' \) -type f -print \
  2>/dev/null | sort)

excluded() {
  awk '$1 == "exclude" { print $2 }' "$config" | while IFS= read -r glob; do
    [ -z "$glob" ] && continue
    # shellcheck disable=SC2254
    case $(basename "$1") in
      $glob) echo match; return ;;
    esac
  done
}

over="${TMPDIR:-/tmp}/orchestra-filelen.$$"
: > "$over"
checked=0

for file in $files; do
  [ -n "$(excluded "$file")" ] && continue
  checked=$((checked + 1))
  lines=$(wc -l < "$file")
  if [ "$lines" -gt "$limit" ]; then
    printf '%s:1: %s lines, limit is %s — split this file\n' "$file" "$lines" "$limit" >> "$over"
  fi
done

if [ -s "$over" ]; then
  cat "$over"
  count=$(wc -l < "$over")
  rm -f "$over"
  echo "file-length: $count file(s) over $limit lines"
  exit 1
fi

rm -f "$over"
echo "file-length: $checked file(s), none over $limit lines"
