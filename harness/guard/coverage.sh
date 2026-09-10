#!/bin/sh
# Coverage guard.
#
# Fails when a Go package covers fewer statements than harness/quality/coverage.txt
# requires of it. See that file for the numbers and why they are what they are.
#
# Per package rather than in total: an average lets template code carry an
# untested implementation, which is exactly what happened - 2.5% overall while
# the two packages holding the work sat under 3%.
#
# Every service is measured. A service is a directory under services/ with a
# go.mod; package names are import paths, so two services never collide.
set -eu

cd "$(dirname "$0")/../.."

config="harness/quality/coverage.txt"
profile="${TMPDIR:-/tmp}/orchestra-coverage.$$"

cleanup() { rm -f "$profile" "$profile".* ; }
trap cleanup EXIT

measured=""
found=0

for service in services/*/; do
  [ -f "${service}go.mod" ] || continue
  found=$((found + 1))

  # A module with no packages is not a failure - `go test ./...` merely has
  # nothing to do, and says so with a non-zero exit.
  [ -n "$(cd "$service" && go list ./... 2>/dev/null)" ] || continue

  name=$(basename "$service")
  prof="$profile.$name"

  (cd "$service" && go test -count=1 -coverprofile="$prof" ./... >/dev/null 2>&1) || {
    echo "coverage: the test run failed in $service; fix the tests first"
    exit 1
  }

  # No packages means no profile written; nothing to measure, not a failure.
  [ -f "$prof" ] || continue

  # 'package total%' for every package the profile mentions.
  one=$(cd "$service" && go tool cover -func="$prof" 2>/dev/null \
    | awk '$1 != "total:" { n=split($1, p, "/"); file=p[n]; sub("/"file"$", "", $1); print $1"\t"$3 }' \
    | sed 's/%//' \
    | awk -F'\t' '{ sum[$1]+=$2; count[$1]++ } END { for (k in sum) printf "%s\t%.1f\n", k, sum[k]/count[k] }')

  measured="$measured$one
"
done

if [ "$found" -eq 0 ]; then
  echo "coverage: no services yet"
  exit 0
fi

printf '%s\n' "$measured" | while IFS="$(printf '\t')" read -r pkg pct; do
  [ -z "$pkg" ] && continue

  # Longest matching suffix from the policy wins; 'default' is the floor.
  want=$(awk -v pkg="$pkg" '
    $1 == "default" { def = $2; next }
    /^[[:space:]]*(#|$)/ { next }
    { if (index(pkg, $1) > 0 && length($1) > best) { best = length($1); want = $2 } }
    END { print (want == "" ? def : want) }
  ' "$config")

  awk -v p="$pct" -v w="$want" 'BEGIN { exit (p + 0 >= w + 0) ? 0 : 1 }' || {
    printf '%s: %s%% covered, minimum is %s%%\n' "$pkg" "$pct" "$want"
    echo fail >> "$profile.failed"
  }
done

if [ -f "$profile.failed" ]; then
  count=$(wc -l < "$profile.failed")
  echo "coverage: $count package(s) below their minimum"
  exit 1
fi

echo "coverage: every package meets its minimum"
