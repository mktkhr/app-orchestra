#!/bin/sh
# Public-mark guard.
#
# x-orchestra-expose: true is what lets an operation reach the model at all
# (services/platform/internal/adapter/specsource/http/parse.go): the
# catalogue drops everything else before ToolsFor and Catalog.Find ever see
# it, so a mark left off or misspelled makes the operation invisible with
# no error anywhere - the service is still fine on its own, the platform
# just never offers it. This guard catches the two ways that silence hides
# a mistake instead of an intent:
#
#   - An operation marked x-orchestra-expose: true that no component could
#     ever render: no requestBody and no 2xx application/json response.
#     Saying "show this to the model" about an operation nothing can draw
#     is a spec bug, not a shape for usecase.ToolsFor to quietly exclude
#     (that exclusion was removed for exactly this reason - DECISIONS.md).
#   - A service with no x-orchestra-expose: true operation at all. Wiring a
#     service up and forgetting to mark anything in it looks, from the
#     platform's side, identical to never having wired it up. The platform's
#     own contract (services/platform/api/openapi.yaml) is exempt from this
#     one check: it is the orchestrator's own API, never a spec
#     specsource/http fetches, so x-orchestra-expose has no meaning on it.
#
# This guard bundles every services/*/api/openapi.yaml with redocly, reads
# each spec's operations with jq, and fails on either case. A malformed
# extension value (not exactly `true`) is caught by
# internal/adapter/specsource/http's own unit tests, not here: this guard
# only sees the bundled document redocly hands back.
set -eu

cd "$(dirname "$0")/../.."

# unrenderable_query finds every exposed operation with neither a request
# body nor a 2xx JSON response - the shape Render (internal/domain/
# rendering.go) can never turn into a component.
unrenderable_query='
  .paths // {}
  | ..
  | objects
  | select(has("operationId"))
  | select(.["x-orchestra-expose"] == true)
  | select(
      (has("requestBody") | not)
      and
      ((.responses // {})
        | to_entries
        | map(select(.key | test("^2")))
        | map(.value.content["application/json"])
        | all(. == null))
    )
  | .operationId
'

exposed_count_query='
  [.paths // {} | .. | objects | select(has("operationId")) | select(.["x-orchestra-expose"] == true)]
  | length
'

tmp_json="${TMPDIR:-/tmp}/orchestra-exposed-ops.$$.json"
bundle_err="${TMPDIR:-/tmp}/orchestra-exposed-ops.$$.err"
problems="${TMPDIR:-/tmp}/orchestra-exposed-ops.$$.problems"
: > "$problems"
trap 'rm -f "$tmp_json" "$bundle_err" "$problems"' EXIT

services=0

for gomod in services/*/go.mod; do
  [ -e "$gomod" ] || continue
  svc_dir=$(dirname "$gomod")
  svc=$(basename "$svc_dir")
  spec="$svc_dir/api/openapi.yaml"
  [ -f "$spec" ] || continue

  services=$((services + 1))

  # Redocly writes a progress line and an upgrade banner to stderr on every
  # run. Hold them back so a passing guard says one line and a failing one
  # says only what is wrong - unless redocly itself fails, which is the one
  # time that output is the diagnostic.
  if ! pnpm exec redocly bundle "$spec" -o "$tmp_json" --ext json >/dev/null 2>"$bundle_err"; then
    cat "$bundle_err" >&2
    echo "guard-exposed-ops: could not bundle $spec"
    exit 1
  fi

  unrenderable=$(jq -r "$unrenderable_query" "$tmp_json")
  if [ -n "$unrenderable" ]; then
    printf '%s: exposed but nothing can render it (no requestBody, no 2xx application/json response): %s\n' \
      "$svc" "$(printf '%s' "$unrenderable" | tr '\n' ' ')" >> "$problems"
  fi

  # The platform's own contract (services/platform/api/openapi.yaml) is the
  # orchestrator's external API - health, plan, invoke - never a spec
  # specsource/http fetches and filters (that only ever happens to the
  # services ORCHESTRA_SERVICES names). x-orchestra-expose has no meaning
  # on it, so it is exempt from "must expose at least one operation";
  # nothing exempts it from the unrenderable check above, since a mark on
  # it would still be a mistake worth catching.
  if [ "$svc" = "platform" ]; then
    continue
  fi

  exposed_count=$(jq "$exposed_count_query" "$tmp_json")
  if [ "$exposed_count" -eq 0 ]; then
    printf '%s: no x-orchestra-expose: true operation - the platform cannot see this service\n' \
      "$svc" >> "$problems"
  fi
done

if [ "$services" -eq 0 ]; then
  echo "guard-exposed-ops: no service contracts to check yet"
  exit 0
fi

if [ -s "$problems" ]; then
  cat "$problems"
  echo "guard-exposed-ops: an exposed operation must be renderable, and every service must expose at least one"
  exit 1
fi

echo "guard-exposed-ops: $services service(s), every x-orchestra-expose: true operation renderable, none empty"
