# Raw material

Primitive data for the measurement work: exact request/response bodies,
byte-for-byte as sent or received, pretty-printed. No prose interpretation
here - that belongs in `docs/measurements/jev-chip-order.md` and its rows
file (not touched by this bundle). Repository commit at capture time:
`ef018a6`, 2026-09-17.

Every JSON file in this directory carries no `Authorization` header and no
API key in any field - `grep -riE "apikey_|typesafe/key|config/typesafe|sk-ant|config/anthropic|authorization"` over this directory is empty.

## 1. Local pipeline traffic (`local-*.json`)

**How captured:** a small logging reverse proxy
(`http.server.ThreadingHTTPServer`, ~80 lines of Python, never committed)
forwarding every request to the real llama-swap at `127.0.0.1:11435` and
writing each request/response body pair to disk before relaying it
unchanged. The platform's own `internal/adapter/planner/chat.Client` and
`internal/adapter/narrowing/llamaswap` package both call one configured
base URL (`ORCHESTRA_LLM_BASE_URL`) for every one of chat completions,
`/v1/embeddings` and `/v1/rerank` - llama-swap serves all three - so
pointing that one env var at the proxy captures the platform's entire
model-facing traffic with no code change and nothing to revert.

A dedicated platform instance was built with `make dev-services` (unmodified
binary) and started by hand on port 8090, its own sqlite DB, against the
real dev `inventory`/`attendance` services already running on :8081/:8082,
with `ORCHESTRA_LLM_BASE_URL=http://127.0.0.1:18099/v1` (the proxy) instead
of llama-swap directly - kept on a port and DB separate from the shared dev
platform on :8080 so this capture could not race another session's own use
of it. Two questions were sent through `/api/plan` after signing in as
`admin`: 「在庫の一覧を見せて」, then 「勤怠の方も見せて」 with
`turns: [{question: "在庫の一覧を見せて", kind: "result", service: "inventory", operationId: "ListInventoryItems"}]`
(the same shape the web client would send back from the first answer).
Every file below is the real body the running binary sent or received for
one of those two calls - no field was hand-edited after capture, only
pretty-printed. The proxy process, the second platform instance, and its
separate DB file were all torn down after capture; `make dev-services` was
re-run afterward to restore the shared dev platform exactly as `make
dev-services` always leaves it.

| File                                                               | Stage                                                                                                                                                                                                                                                                                                                                                                                                                            |
| ------------------------------------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `local-pick-request.json` / `-response.json`                       | Pick stage (`internal/adapter/planner/pick`), 「在庫の一覧を見せて」, no turns                                                                                                                                                                                                                                                                                                                                                   |
| `local-fill-request.json` / `-response.json`                       | Fill/toolcall stage, same question                                                                                                                                                                                                                                                                                                                                                                                               |
| `local-pick-request-with-turns.json` / `-response-with-turns.json` | Pick stage for the follow-up 「勤怠の方も見せて」, showing the `直前:` line (`b4ac66c`)                                                                                                                                                                                                                                                                                                                                          |
| `local-fill-request-with-turns.json` / `-response-with-turns.json` | Fill/toolcall stage for the follow-up (not explicitly asked for in the brief; kept since it was captured in the same call and shows the conversation-history block the fill's own user content carries)                                                                                                                                                                                                                          |
| `local-narrowing-embed-request.json` / `-response.json`            | The **query-side** embedding call (`/v1/embeddings`, one input, `query:` prefix) that ranks the shortlist for 「在庫の一覧を見せて」                                                                                                                                                                                                                                                                                             |
| `local-narrowing-rerank-request.json` / `-response.json`           | The reranking call (`/v1/rerank`) for the same question. Not truncated - the dev catalogue only has 6 operations, so this is the full body                                                                                                                                                                                                                                                                                       |
| `local-narrowing-catalog-embed-request.json` / `-response.json`    | The **document-side** embedding call: all 20 catalogue passages (6 operations x 3 own-summary-or-examples lines, `passage:` prefix), embedded **once at platform startup** (`llamaswap.Load`, `embed.go`; the platform log line is `"narrowing loaded","vectors":20`), not per question. Response truncated to the first 3 of 20 vectors, each cut to its first 8 of 1024 dimensions (noted in the file's own `_truncated_note`) |

Read `local-narrowing-embed-request.json` and `local-narrowing-catalog-embed-request.json`
together: the shortlist a question gets is these 20 cached document
vectors reranked against that one fresh query vector, not 20 embedding
calls per question.

## 2. Jev configuration bodies (`jev-*.json`)

**How captured:** no network call to Jev for the request bodies themselves.
A temporary Go test (`internal/adapter/planner/jev/zzrawcapture_internal_test.go`,
package `jev`, gated behind `JEV_RAW_CAPTURE=1`, deleted immediately after
use - never committed, `git status` confirmed clean before this bundle was
committed) called this package's own unexported `buildRequest` and
`buildGateRequest` directly against the real dev catalogue (fetched live
via `specsourcehttp.New` from the running :8081/:8082 services - 6
operations), then `json.MarshalIndent`'d each result to
`/tmp/jev-raw/*.json`, which were copied into this directory unchanged.
This is the same code path `jev.Picker.Pick` / `jev.Gate.Gate` build a
request with; only the network round-trip to `api.typesafe.ai` is skipped.
See `parameters.md` section 2 for exactly which knobs (criteria/turns/
objectInstructions/fan-out) each of the six files exercises and which env
vars produce it in the running platform.

All six are for 「在庫の一覧を見せて」 (the turns variant additionally
carries the follow-up 「勤怠の方も見せて」 with one prior turn), matching
the local pipeline capture above so the two can be diffed against each
other question-for-question.

**Captured responses - what exists and what doesn't**, so nothing here is
invented:

- `jev-v1-example-request-response.json` - not a new capture. Extracted
  verbatim from the JSON code block already published in
  `docs/measurements/jev-picker-v1.md` section 1 (a real call, `response_status: 200`).
  That file is for a **different question** (a03, 「ピッキングリストが知りたい」)
  against a different, 3-endpoint shortlist, not 「在庫の一覧を見せて」 - v1.md's
  own text says "Full JSON saved beside this file as
  `example-request-response.json`", but no such file exists anywhere in
  the repository; this bundle recovers that promise from the text that
  already carries the data, rather than re-promising it again.
- `jev-gate-v3-example.json` (already in `docs/measurements/`, not
  duplicated here) pairs with `jev-gate-only-request.json` in shape (both
  are gate-only bodies) but not byte-for-byte - the v3 example is for
  「休暇申請書を印刷したい」 (mid corpus m44) against an illustrative
  3-endpoint shortlist.
- `docs/measurements/jev-v5-example-request.json` / `-response.json`
  (already in `docs/measurements/`, not duplicated here) is a captured
  response, but for a **seventh** combination not listed among the six
  above: v2 criteria + object instructions + turns + fan-out, all four
  together (the v5 round's own production default). It does not stand in
  for `jev-object-instructions-request.json` or `jev-fanout-gate-request.json`
  alone - see `parameters.md` section 2.
- **No captured response exists anywhere in the repository** for
  `jev-v2-criteria-request.json`, `jev-v2-with-turns-request.json`,
  `jev-object-instructions-request.json` (in isolation, i.e. without
  fan-out and turns also on), or `jev-fanout-gate-request.json` (in
  isolation, i.e. without object instructions and turns also on) - `jev-picker-v2.md`
  and `jev-language-v4.md` were grepped for `"choice":`, `"noul":`,
  `response_body` and ` ```json` and none matched. This bundle does not
  spend new API budget to fill that gap: the two corpus-pick `.jsonl`
  files (`jev-picker-v1-corpus-picks.jsonl`, `jev-picker-v2-corpus-picks.jsonl`)
  do carry full per-question probability distributions for v1 and v2
  criteria across the whole 100-question corpus - response-derived data,
  not a raw body, and not for 「在庫の一覧を見せて」 specifically, but the
  closest thing to "what did Jev actually answer" that already exists for
  those two configurations.

## 3. `parameters.md`, `instruments.md`

`parameters.md`: every env var and llama-swap setting behind a body in
this bundle, one table. `instruments.md`: the three measurement
instruments (narrowing corpus, mid subset, eval suite) in data terms - file
paths, how to run each, current numbers.

## 4. Regenerating this bundle

Local traffic: point `ORCHESTRA_LLM_BASE_URL` at a logging proxy in front
of llama-swap, start a platform instance on a spare port against the real
dev services (`make dev-services` first), sign in, `POST /api/plan`.
Jev bodies: add a temporary internal test to
`internal/adapter/planner/jev` calling `buildRequest`/`buildGateRequest`
directly (see section 2), run it, delete it. Neither needs a change to any
tracked file - `git status` shows only `docs/measurements/raw/**` both
before and after either capture.
