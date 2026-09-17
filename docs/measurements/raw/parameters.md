# Parameters

Every knob that affects a captured body in this bundle, and its value in
each round. Source is the code, not memory: `services/platform/internal/infra/config/config.go`
and `config_gate.go` (env vars), `internal/adapter/planner/jev/*.go` (Jev
adapter switches), `~/ghq/github.com/mktkhr/local-llm/deploy/llama/config.yaml`
(llama-swap).

## 1. Platform env vars

| Var                                 | Field                          | Default when unset                       | Controls                                                                                                                                                                                                             |
| ----------------------------------- | ------------------------------ | ---------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `ORCHESTRA_PLANNER_STAGES`          | `Config.PlannerStages`         | `2` (pick-then-fill)                     | `1` = single toolcall; `2` = pick (usecase.Picker) then fill                                                                                                                                                         |
| `ORCHESTRA_PLANNER_WORDING`         | `Config.PlannerWording`        | `wording.Default().Name` (`v1`)          | Which named word-set the toolcall planner's own prompt uses                                                                                                                                                          |
| `ORCHESTRA_PLANNER_THINKING`        | `Config.PlannerThinking`       | `off` (`false`)                          | Qwen3.5 thinking on/off for the toolcall/fill call; measured 2026-09-16: off wins on latency, ties on correctness (67/71 vs 67/70)                                                                                   |
| `ORCHESTRA_PLANNER_TODAY`           | `Config.PlannerToday`          | unset (real clock)                       | Pins the `今日は YYYY-MM-DD（曜）です。` date line so two runs on different days send byte-identical requests                                                                                                        |
| `ORCHESTRA_PLANNER_REPEAT_PENALTY`  | `Config.PlannerRepeatPenalty`  | unset (field omitted)                    | `chat.Request.RepeatPenalty` for the toolcall/fill call                                                                                                                                                              |
| `ORCHESTRA_PLANNER_REPEAT_LAST_N`   | `Config.PlannerRepeatLastN`    | `64`                                     | Only effective alongside `ORCHESTRA_PLANNER_REPEAT_PENALTY`                                                                                                                                                          |
| `ORCHESTRA_NARROWING_EMBED_MODEL`   | `Config.NarrowingEmbedModel`   | unset (narrowing off)                    | llama-swap model name for `/v1/embeddings`; all three `NARROWING_*` vars must be set together or not at all                                                                                                          |
| `ORCHESTRA_NARROWING_RERANK_MODEL`  | `Config.NarrowingRerankModel`  | unset                                    | llama-swap model name for `/v1/rerank`                                                                                                                                                                               |
| `ORCHESTRA_NARROWING_K`             | `Config.NarrowingK`            | unset                                    | Shortlist size narrowing cuts the catalogue to                                                                                                                                                                       |
| `ORCHESTRA_PICKER`                  | `Config.Picker`                | `local`                                  | `local` (internal/adapter/planner/pick) or `jev` (internal/adapter/planner/jev); only meaningful under `PLANNER_STAGES=2`                                                                                            |
| `ORCHESTRA_JEV_CRITERIA`            | `Config.JevCriteria`           | `v1`                                     | `v1` (flat one-line criteria) or `v2` (object: `what`/`examples`/`not_for`)                                                                                                                                          |
| `ORCHESTRA_JEV_OBJECT_INSTRUCTIONS` | `Config.JevObjectInstructions` | `false` (any non-empty value means true) | Sends the "pick" question's instructions as the v5 object form instead of a plain string. Renamed from `ORCHESTRA_JEV_LEGACY_INSTRUCTIONS` (docs/measurements/jev-v5.md's own command lines still show the old name) |
| `ORCHESTRA_GATE`                    | `Config.Gate`                  | `none`                                   | `none` or `jev` (internal/adapter/planner/jev.Gate, the standalone noul refusal gate)                                                                                                                                |
| `ORCHESTRA_JEV_GATE_THRESHOLD`      | `Config.JevGateThreshold`      | `0.7`                                    | noul probability at/above which Gate (or the fan-out path) judges a question Impossible                                                                                                                              |
| `ORCHESTRA_CONTEXT_TURNS`           | `Config.ContextTurns`          | `defaultContextTurns`                    | How many prior turns `Orchestrator.Plan` keeps and sends as `turns`                                                                                                                                                  |

Fan-out gate is not its own env var: `pkg/app/app_staging.go`'s
`fanOut := cfg.Picker.Name == PickerJev && cfg.Gate.Name == GateJev` turns
it on when **both** `ORCHESTRA_PICKER=jev` and `ORCHESTRA_GATE=jev` are set
together - that combination builds one `jev.Picker` with `WithFanOutGate`
and skips the standalone `jev.Gate` entirely. `ORCHESTRA_GATE=jev` with
`ORCHESTRA_PICKER=local` (or unset) builds the standalone gate instead
(`jev-gate-only-request.json`, `jev-gate-v3-example.json`).

## 2. Six Jev configurations captured in this bundle

`buildRequest(query, answers, turns, shortlist, criteria, objectInstructions)`
and `buildGateRequest(query, answers, shortlist)`, `internal/adapter/planner/jev/mapping.go`.

| File                                   | criteria             | turns | objectInstructions | fan-out | Env vars that produce it                                                                               |
| -------------------------------------- | -------------------- | ----- | ------------------ | ------- | ------------------------------------------------------------------------------------------------------ |
| `jev-v1-criteria-request.json`         | v1                   | no    | no                 | no      | `ORCHESTRA_PICKER=jev` (criteria unset)                                                                |
| `jev-v2-criteria-request.json`         | v2                   | no    | no                 | no      | `ORCHESTRA_PICKER=jev ORCHESTRA_JEV_CRITERIA=v2`                                                       |
| `jev-v2-with-turns-request.json`       | v2                   | yes   | no                 | no      | same as above, with a prior turn present - today's production default (`5ff012d`, `b4ac66c`)           |
| `jev-object-instructions-request.json` | v2                   | no    | yes                | no      | `+ ORCHESTRA_JEV_OBJECT_INSTRUCTIONS=1`                                                                |
| `jev-fanout-gate-request.json`         | v2                   | no    | no                 | yes     | `ORCHESTRA_PICKER=jev ORCHESTRA_GATE=jev` (v5's own default criteria/instructions when built this way) |
| `jev-gate-only-request.json`           | n/a (gate, not pick) | no    | n/a                | n/a     | `ORCHESTRA_GATE=jev` with `ORCHESTRA_PICKER` left `local`                                              |

All six were built for 「在庫の一覧を見せて」 (the turns variant also carries
the follow-up 「勤怠の方も見せて」 with one prior turn), against the real dev
catalogue (`make dev-services`: inventory on :8081, attendance on :8082;
6 operations total) - see this directory's `README.md` for how.

`docs/measurements/jev-v5-example-request.json` / `-response.json` is a
**different, seventh** combination captured live in the v5 round: v2
criteria + object instructions + turns + fan-out, all four together (the
production v5 default at the time) - not an exact response for any one of
the six files above. `docs/measurements/jev-gate-v3-example.json` is a
gate-only request/response pair, but for a different question (「休暇申請書を
印刷したい」, m44) against an illustrative 3-endpoint shortlist, not the
6-endpoint dev catalogue `jev-gate-only-request.json` above uses. See
`README.md` section 3 for exactly which of the six requests have no
captured response at all.

## 3. Fixed sampling parameters the platform sends

| Call                            | temperature | max_tokens                                                              | thinking                                                                                                                            |
| ------------------------------- | ----------- | ----------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------- |
| pick (`chat.Client`)            | `0`         | `200` (`pickMaxTokens`, `internal/adapter/planner/pick/picker.go`)      | always off - `chat_template_kwargs: {"enable_thinking": false}` sent unconditionally, regardless of the request's own thinking flag |
| fill / toolcall (`chat.Client`) | `0`         | `1024` (`planningMaxTokens`, `internal/adapter/planner/chat/client.go`) | off only when `ORCHESTRA_PLANNER_THINKING` resolves off (`chatTemplateKwargsFor`, `internal/adapter/planner/toolcall/planner.go`)   |

## 4. llama-swap models and the machine

Machine: RTX 4080 SUPER, 16 GB VRAM (`~/ghq/github.com/mktkhr/local-llm`
README).

The `narrowing` group (`groups.narrowing`, config.yaml, bottom of file):
`swap: false`, `exclusive: false`, `persistent: true` - all three models
stay resident so one `/api/plan` call never pays a model-swap cost, the
reason this group exists (`DECISIONS.md` 2026-09-15: up to 3 swaps/request,
7-40s, without it). Members, quoted verbatim:

```yaml
qwen3.5-9b-q8:
  name: Qwen3.5 9B (Q8_0) / bench baseline
  cmd: |
    llama-server --host 0.0.0.0 --port ${PORT} --jinja -ngl 99 --flash-attn on -np 1 --metrics
    -hf unsloth/Qwen3.5-9B-GGUF:Q8_0
    --ctx-size 131072
    --kv-unified
    -ctk q8_0 -ctv q8_0
    -b 2048 -ub 512
    -lv 5

e5-large-q8:
  name: multilingual-e5-large (Q8_0)
  cmd: |
    llama-server --host 0.0.0.0 --port ${PORT} -ngl 99
    --embedding
    --pooling mean
    -hf chris-code/multilingual-e5-large-Q8_0-GGUF:Q8_0
    --ctx-size 512
    -b 512 -ub 512

bge-reranker-v2-m3-q8:
  name: BGE reranker v2 m3 (Q8_0)
  cmd: |
    llama-server --host 0.0.0.0 --port ${PORT} -ngl 99
    --reranking
    -hf gpustack/bge-reranker-v2-m3-GGUF:Q8_0
    --ctx-size 8192
    -b 8192 -ub 2048
```

`qwen3.5-9b-q8` is Qwen3.5 9B at Q8_0 quantisation - the platform's
`ORCHESTRA_LLM_MODEL` / `DEV_LLM_MODEL` in every capture here.
`e5-large-q8` is multilingual-e5-large at Q8_0, mean-pooled, with the
`query:`/`passage:` prefix contract visible in every narrowing body in this
bundle. `bge-reranker-v2-m3-q8` is BGE reranker v2 m3 at Q8_0.

**`--cache-reuse 256` was removed from `qwen3.5-9b-q8`'s command line on
2026-09-17.** This is observed, not inferred: `git diff` in
`~/ghq/github.com/mktkhr/local-llm` shows it as an **uncommitted**
working-tree edit against commit `fb3e1ff` (2026-09-05), with no commit
message and no comment anywhere in `config.yaml` or that repo's `.md`
files explaining it (grepped for "cache-reuse" across all of them). The
entry's own comment block says `--cache-reuse 256` and `-lv 5` were added
together "V-2（プレフィックスキャッシュのヒット率）を測るための設定。常用
エントリとは分けてある" ("settings for measuring V-2, prefix-cache hit
rate; kept separate from the everyday-use entry") - `-lv 5` (verbose
logging) stayed, `--cache-reuse 256` did not. State this as what it is:
an observed, undocumented, uncommitted change on today's date, not a
rationale this bundle invents.

## 5. Fixed narrowing parameters (dev)

`ORCHESTRA_NARROWING_EMBED_MODEL=e5-large-q8`,
`ORCHESTRA_NARROWING_RERANK_MODEL=bge-reranker-v2-m3-q8`,
`ORCHESTRA_NARROWING_K=20` (`Makefile`'s `dev-services` target). The dev
catalogue (inventory + attendance, `make dev-services`) has only 6
operations total, so `K=20` never actually truncates it in any capture
here - every narrowing body in this bundle carries the full catalogue.
