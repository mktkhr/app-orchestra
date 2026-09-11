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
#   - A property a public operation actually draws on screen with no
#     `title`: the field label a person sees comes from the schema's
#     `title` (`web/src/entities/rendering/model/rows.ts`, `columnTitle`);
#     with none, the raw English JSON key (`name`, `quantity`) leaks
#     through instead, and nothing fails - Task 13/14 only turned this up
#     once a UI existed to look at (TODO.md, DECISIONS.md 2026-09-11).
#     "Draws on screen" is read straight off the platform's own rendering
#     code, not redefined here:
#       - the response side mirrors domain.FieldsSchema
#         (internal/domain/rendering.go): a table's row schema (the array
#         itself, or the sole array-valued property of a wrapper object
#         such as {items: [...], total: n}), or a detail's response object
#         itself, using the same first-2xx-json-response rule
#         specsource/http.convertResponse applies;
#       - the request body side is a create/update form's fields
#         (usecase.mergeRequestBody): every property of an object request
#         body;
#       - parameters count too: usecase.inputSchemaFor merges every
#         parameter into the same form schema `ask` degrades to whenever a
#         stuck argument has no enum to offer (usecase/orchestrator.go,
#         `ask`), and the form component renders that schema's properties
#         directly (web/src/entities/rendering/ui/ResultForm.tsx) - a
#         parameter reaches the screen exactly like a body field does.
#     A shared schema (ItemStatus, say) only needs its `title` written
#     once; every $ref to it inherits it, and redocly has already resolved
#     every $ref by the time this guard reads the bundle.
#
# This guard bundles every services/*/api/openapi.yaml with redocly, reads
# each spec's operations with jq, and fails on any of the three. A malformed
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

# missing_titles_query finds every property a public operation draws on
# screen with no `title` - see the header comment above for what "draws on
# screen" means and why. It mirrors three pieces of Go exactly, by name, so
# a future change to any of them is a prompt to re-check this query rather
# than a silent divergence:
#
#   - sole_array_prop / is_object_array / fields_schema mirror
#     soleArrayProperty / isObjectArray / FieldsSchema
#     (internal/domain/rendering.go).
#   - response_schema mirrors convertResponse
#     (internal/adapter/specsource/http/parse.go): the first 2xx response,
#     in ascending status order, with an application/json schema.
#   - the request body branch mirrors mergeRequestBody's object case
#     (internal/usecase/tools.go): a non-object body is carried as a single
#     "body" argument there, not expanded into named properties, so it has
#     no per-property titles to require here either.
missing_titles_query='
  def sole_array_prop(s):
    ([s.properties // {} | to_entries[] | select(.value.type == "array")]) as $arrs
    | if ($arrs | length) == 1 then $arrs[0].value else null end;

  def is_object_array(s):
    if s == null then false
    elif s.type == "array" then (s.items != null and s.items.type == "object")
    elif s.type == "object" then
      (sole_array_prop(s)) as $arr
      | ($arr != null and $arr.items != null and $arr.items.type == "object")
    else false
    end;

  def fields_schema(s):
    if s == null then null
    elif is_object_array(s) then
      (if s.type == "array" then s.items else sole_array_prop(s).items end)
    elif s.type == "object" then s
    else null
    end;

  def response_schema(op):
    ((op.responses // {}) | keys | map(select(test("^2"))) | sort) as $statuses
    | reduce $statuses[] as $st (null;
        if . != null then .
        else (op.responses[$st].content["application/json"].schema // null)
        end);

  def untitled_names(props):
    [(props // {}) | to_entries[] | select((.value.title // "") == "") | .key];

  .paths // {}
  | ..
  | objects
  | select(has("operationId"))
  | select(.["x-orchestra-expose"] == true)
  | . as $op
  | (
      (untitled_names(fields_schema(response_schema($op)).properties)
        | map({op: $op.operationId, source: "response field", prop: .})),
      (if (($op.requestBody.content["application/json"].schema.type // "") == "object")
        then untitled_names($op.requestBody.content["application/json"].schema.properties)
          | map({op: $op.operationId, source: "request body field", prop: .})
        else [] end),
      (untitled_names(($op.parameters // []) | map({(.name): .schema}) | add // {})
        | map({op: $op.operationId, source: "parameter", prop: .}))
    )
  | .[]
  | "\(.op)\t\(.source)\t\(.prop)"
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
  #
  # --dereferenced, not a plain bundle: an ordinary bundle only inlines
  # external documents and leaves an internal $ref (say, a parameter's
  # schema pointing at #/components/schemas/ItemStatus) exactly as
  # written, so missing_titles_query would see the $ref object itself,
  # never the shared schema's title. Dereferencing resolves every $ref in
  # place before jq ever reads the file, which is also what "a shared
  # schema's title covers every $ref to it" (see the header comment)
  # depends on being true.
  if ! pnpm exec redocly bundle "$spec" -o "$tmp_json" --ext json --dereferenced >/dev/null 2>"$bundle_err"; then
    cat "$bundle_err" >&2
    echo "guard-exposed-ops: could not bundle $spec"
    exit 1
  fi

  unrenderable=$(jq -r "$unrenderable_query" "$tmp_json")
  if [ -n "$unrenderable" ]; then
    printf '%s: exposed but nothing can render it (no requestBody, no 2xx application/json response): %s\n' \
      "$svc" "$(printf '%s' "$unrenderable" | tr '\n' ' ')" >> "$problems"
  fi

  jq -r "$missing_titles_query" "$tmp_json" | while IFS="$(printf '\t')" read -r op source prop; do
    [ -n "$op" ] || continue
    printf '%s: %s %s: %s has no title\n' "$svc" "$op" "$source" "$prop" >> "$problems"
  done

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
  echo "guard-exposed-ops: an exposed operation must be renderable, every service must expose at least one, and every property it draws on screen needs a title"
  exit 1
fi

echo "guard-exposed-ops: $services service(s), every x-orchestra-expose: true operation renderable, none empty, every drawn property titled"
