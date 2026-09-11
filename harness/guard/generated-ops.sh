#!/bin/sh
# Operation-completeness guard.
#
# make guard-generated only tells you the generated code is *stale* relative
# to the spec (a `git diff`). It says nothing about whether the generator
# actually emitted every operation the spec declares: oapi-codegen v2.8.0
# silently drops an OpenAPI 3.2 `query` operation, no error, no warning. The
# spec is valid, `make generate` "succeeds", and the resulting
# ServerInterface is simply missing a method.
#
# This guard bundles each services/*/api/openapi.yaml with redocly, extracts
# every operationId with jq, and checks that each one appears in both
# generated artifacts, when that artifact exists:
#   - services/<name>/internal/adapter/openapi/openapi.gen.go
#     (Go: oapi-codegen turns operationId into a PascalCase method name,
#     e.g. searchItems -> SearchItems)
#   - web/src/shared/api/gen/<name>.d.ts
#     (TypeScript: openapi-typescript uses operationId verbatim as a key of
#     the `operations` type)
#
# A service without a spec, or a spec whose generated files do not exist
# yet, is "nothing to check" here, not a failure: guard-generated is what
# enforces that generated code exists and is current.
set -eu

cd "$(dirname "$0")/../.."

# pascal_case OP turns an operationId into the Go method name oapi-codegen
# would generate for it: split on any run of non-alphanumeric characters,
# capitalise the first letter of every piece, join.
pascal_case() {
  printf '%s' "$1" | awk '
    {
      n = split($0, parts, /[^A-Za-z0-9]+/)
      out = ""
      for (i = 1; i <= n; i++) {
        p = parts[i]
        if (p == "") continue
        out = out toupper(substr(p, 1, 1)) substr(p, 2)
      }
      print out
    }'
}

tmp_json="${TMPDIR:-/tmp}/orchestra-generated-ops.$$.json"
missing="${TMPDIR:-/tmp}/orchestra-generated-ops-missing.$$"
: > "$missing"
trap 'rm -f "$tmp_json" "$missing"' EXIT

checked=0

for gomod in services/*/go.mod; do
  [ -e "$gomod" ] || continue
  svc_dir=$(dirname "$gomod")
  svc=$(basename "$svc_dir")
  spec="$svc_dir/api/openapi.yaml"
  [ -f "$spec" ] || continue

  go_gen="$svc_dir/internal/adapter/openapi/openapi.gen.go"
  ts_gen="web/src/shared/api/gen/$svc.d.ts"

  have_go=0
  [ -f "$go_gen" ] && have_go=1
  have_ts=0
  [ -f "$ts_gen" ] && have_ts=1

  # Neither artifact has been generated yet for this service: nothing to
  # check against.
  if [ "$have_go" -eq 0 ] && [ "$have_ts" -eq 0 ]; then
    continue
  fi

  checked=$((checked + 1))

  pnpm exec redocly bundle "$spec" -o "$tmp_json" --ext json >/dev/null

  op_ids=$(jq -r '[.paths // {} | .. | objects | select(has("operationId")) | .operationId] | unique | .[]' "$tmp_json")

  [ -z "$op_ids" ] && continue

  printf '%s\n' "$op_ids" | while IFS= read -r op; do
    [ -z "$op" ] && continue

    if [ "$have_go" -eq 1 ]; then
      method=$(pascal_case "$op")
      if ! grep -qE "\\b${method}\\(" "$go_gen"; then
        printf '%s: operationId "%s" missing from %s (expected method %s)\n' \
          "$svc" "$op" "$go_gen" "$method" >> "$missing"
      fi
    fi

    if [ "$have_ts" -eq 1 ]; then
      if ! grep -qE "^[[:space:]]*(readonly[[:space:]]+)?${op}\\??:" "$ts_gen"; then
        printf '%s: operationId "%s" missing from %s\n' "$svc" "$op" "$ts_gen" >> "$missing"
      fi
    fi
  done
done

if [ -s "$missing" ]; then
  cat "$missing"
  echo "guard-generated-ops: generated code is missing operation(s) the spec declares"
  exit 1
fi

if [ "$checked" -eq 0 ]; then
  echo "guard-generated-ops: no generated artifacts to check yet"
else
  echo "guard-generated-ops: $checked service(s) checked, every operationId present"
fi
