#!/bin/sh
# Cross-service operationId uniqueness guard.
#
# Redocly's operation-operationId-unique rule reads one document at a time,
# so it cannot see that two services declare the same operationId. Nothing
# else looks either: each spec is generated, linted and compiled on its own.
#
# The platform needs them unique across the whole catalogue, for two
# reasons, and both of them fail quietly:
#
#   - A tool definition is named by its operationId alone (services/platform
#     internal/usecase/tools.go). Two services declaring listItems put two
#     tools called listItems in front of the model, which a tool-calling API
#     rejects outright and a JSON planner cannot tell apart.
#   - A tool call carries only the name back, so the platform resolves the
#     service by searching the catalogue for it
#     (internal/adapter/planner/toolcall). With a duplicate, the first match
#     wins and the call goes to a service the model did not mean.
#
# Neither shows up as an error in the service that caused it: the second
# service to use the name is valid on its own, and the collision only exists
# in the platform, at runtime, once both are running.
#
# This guard bundles every services/*/api/openapi.yaml with redocly, reads
# each spec's operationIds with jq, and fails when one name appears in more
# than one service. A duplicate inside a single spec is Redocly's job, not
# this one.
set -eu

cd "$(dirname "$0")/../.."

pairs="${TMPDIR:-/tmp}/orchestra-operation-ids.$$"
tmp_json="${TMPDIR:-/tmp}/orchestra-operation-ids.$$.json"
bundle_err="${TMPDIR:-/tmp}/orchestra-operation-ids.$$.err"
: > "$pairs"
trap 'rm -f "$pairs" "$tmp_json" "$bundle_err"' EXIT

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
    echo "guard-operation-ids: could not bundle $spec"
    exit 1
  fi

  jq -r --arg svc "$svc" \
    '[.paths // {} | .. | objects | select(has("operationId")) | .operationId]
     | unique | .[] | "\(.)\t\($svc)"' "$tmp_json" >> "$pairs"
done

if [ "$services" -eq 0 ]; then
  echo "guard-operation-ids: no service contracts to check yet"
  exit 0
fi

# An operationId owned by more than one service. Sorting by name puts every
# claim on a name together; awk reports the name once with all its services.
duplicates=$(sort "$pairs" | awk -F'\t' '
  {
    if ($1 == name) {
      owners = owners ", " $2
      count++
    } else {
      if (count > 1) printf "%s: claimed by %s\n", name, owners
      name = $1
      owners = $2
      count = 1
    }
  }
  END { if (count > 1) printf "%s: claimed by %s\n", name, owners }
')

if [ -n "$duplicates" ]; then
  printf '%s\n' "$duplicates"
  echo "guard-operation-ids: an operationId names one operation across every service, because that is all a tool call carries"
  exit 1
fi

total=$(wc -l < "$pairs" | tr -d ' ')
echo "guard-operation-ids: $total operationId(s) across $services service(s), all distinct"
