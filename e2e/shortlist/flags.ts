/**
 * `run.ts`'s own command-line flag parsing, split out only for eslint's
 * `max-lines` (docs/plans/staging.md, Task 3: adding `--stages` pushed
 * `run.ts` past the 300-line cap) - every function here reads
 * `process.argv` directly, exactly as it did inline in `run.ts` before
 * this split.
 */

/** `--wording a,b,c`'s names, deduplicated in first-seen order - `undefined` when `--wording` was not given at all. */
export function wordingNames(): readonly string[] | undefined {
  const flagIndex = process.argv.indexOf("--wording");

  if (flagIndex === -1) return undefined;

  const raw = process.argv[flagIndex + 1] ?? "";
  const named = raw
    .split(",")
    .map((n) => n.trim())
    .filter((n) => n.length > 0);

  return [...new Set(named)];
}

/** One flag's value, e.g. `flagValue("--thinking")` for `--thinking off`. `undefined` when the flag was not given. */
function flagValue(flag: string): string | undefined {
  const flagIndex = process.argv.indexOf(flag);

  if (flagIndex === -1) return undefined;

  return process.argv[flagIndex + 1];
}

/** `--thinking on|off`, validated - anything else is a usage error, not a silent no-op. */
export function thinkingArg(): "on" | "off" | undefined {
  const raw = flagValue("--thinking");

  if (raw === undefined) return undefined;

  if (raw !== "on" && raw !== "off") {
    throw new Error(`--thinking must be "on" or "off", got ${JSON.stringify(raw)}`);
  }

  return raw;
}

/** `--repeat-penalty <float>`, validated - anything that does not parse as a finite number is a usage error. */
export function repeatPenaltyArg(): number | undefined {
  const raw = flagValue("--repeat-penalty");

  if (raw === undefined) return undefined;

  const parsed = Number(raw);

  if (!Number.isFinite(parsed)) {
    throw new TypeError(`--repeat-penalty must be a number, got ${JSON.stringify(raw)}`);
  }

  return parsed;
}

/** `--stages 1|2` (docs/plans/staging.md, Task 3), validated - anything else is a usage error, not a silent no-op. */
export function stagesArg(): 1 | 2 | undefined {
  const raw = flagValue("--stages");

  if (raw === undefined) return undefined;

  if (raw !== "1" && raw !== "2") {
    throw new Error(`--stages must be "1" or "2", got ${JSON.stringify(raw)}`);
  }

  return raw === "2" ? 2 : 1;
}

/** `--corpus mid` (docs/plans/midsizing.md, Task 3): the only corpus name this flag accepts besides the default (unset, the shortlist corpus) - anything else is a usage error, not a silent no-op. */
export function corpusArg(): "mid" | undefined {
  const raw = flagValue("--corpus");

  if (raw === undefined) return undefined;

  if (raw !== "mid") {
    throw new Error(`--corpus must be "mid", got ${JSON.stringify(raw)}`);
  }

  return raw;
}

/**
 * `--narrowing on|off`: the full-catalogue Jev trial's own flag (the
 * hierarchical pick request, internal/adapter/planner/jev/hierarchical.go)
 * - `run-mid.ts`'s own mid pass always ran with narrowing on, K=20 (the
 * shortlist measurement's own setting, `boot.ts`'s `NARROWING`) until this
 * flag existed; `--narrowing off` gives Jev the mid fixture's full,
 * un-narrowed catalogue instead. `undefined` (the flag not given at all)
 * keeps `runMid`'s own previous, byte-identical default - narrowing on -
 * so an invocation that predates this flag behaves exactly as it always
 * has. Validated the same way `--thinking`/`--corpus` are: anything but
 * "on"/"off" is a usage error, not a silent no-op.
 */
export function narrowingArg(): "on" | "off" | undefined {
  const raw = flagValue("--narrowing");

  if (raw === undefined) return undefined;

  if (raw !== "on" && raw !== "off") {
    throw new Error(`--narrowing must be "on" or "off", got ${JSON.stringify(raw)}`);
  }

  return raw;
}

/**
 * The `-nothink` / `-rp<value>` / `-stages2` / `-jev` / `-v2` / `-gate` /
 * `-router` / `-ops` suffix a variant's output files carry (run.ts's own doc
 * comment, docs/plans/staging.md; docs/plans/midsizing.md Task 3 reuses
 * it for `--corpus mid`'s `mid-<variant>.jsonl`) - "" for a plain wording
 * pass, unchanged from before any of these flags existed. `stages` of `1`
 * or `undefined` carries no suffix - `STAGES=1` reproduces the plain pass
 * byte for byte (docs/plans/staging.md, AC-S-101).
 *
 * `-jev`/`-hybrid`, `-v2`, `-gate` and `-router` are read straight from
 * `ORCHESTRA_PICKER`/`ORCHESTRA_JEV_CRITERIA`/`ORCHESTRA_GATE`/
 * `ORCHESTRA_SERVICE_ROUTER`, unlike the other three which come from
 * `run.ts`'s own `--flag` parsing above: the picker, the gate and the
 * service router are each chosen through the platform's own environment
 * (`e2e/eval/services.ts`/`e2e/shortlist/boot.ts` pass all four through
 * the same way), not a run.ts flag, so this reads those environment
 * variables directly rather than growing more parameters every call site
 * would have to thread through for names nothing else here needs.
 * `ORCHESTRA_PICKER=hybrid` (internal/adapter/planner/hybrid, Jev first,
 * falling back to the local picker below its own confidence threshold)
 * gets its own `-hybrid` suffix, not `-jev` - the two name different
 * pickers, and a shared suffix would make a hybrid run's output file
 * indistinguishable from a plain Jev run's. `-router`
 * (docs/measurements/jev-full-catalogue.md; DECISIONS.md 2026-09-18,
 * "let Jev choose the service") is independent of all the others - it
 * can appear alongside any picker/gate combination, since it runs before
 * narrowing rather than in place of the picker or the gate. `-ops`
 * (ORCHESTRA_SERVICE_ROUTER_CRITERIA=ops, the router's own richer
 * criteria form) is meaningful only alongside `-router`, but - like `-v2`
 * alongside `-jev` - is appended whenever the env var is literally "ops"
 * regardless of ORCHESTRA_SERVICE_ROUTER, so the two forms' own output
 * files never collide even if they are ever produced independently.
 */
export function variantSuffix(
  thinking?: "on" | "off",
  repeatPenalty?: number,
  stages?: 1 | 2,
): string {
  // "on" gets its own suffix too: since 2026-09-16 the platform's default is
  // off, so an explicit "on" is a distinct variant, not the plain pass.
  const thinkSuffix = { on: "-think", off: "-nothink" } as const;
  const nothink = thinking === undefined ? "" : thinkSuffix[thinking];
  const rp = repeatPenalty === undefined ? "" : `-rp${String(repeatPenalty)}`;
  const st = stages === undefined || stages === 1 ? "" : `-stages${String(stages)}`;
  const picker = process.env["ORCHESTRA_PICKER"];
  const jev = picker === "hybrid" ? "-hybrid" : picker === "jev" ? "-jev" : "";
  // The Jev trial's second round (2026-09-17, "v2: richer criteria"):
  // ORCHESTRA_JEV_CRITERIA, read the same direct way as ORCHESTRA_PICKER
  // just above. Only meaningful alongside "-jev" (jev), but this suffix
  // is appended whenever ORCHESTRA_JEV_CRITERIA is literally "v2"
  // regardless of ORCHESTRA_PICKER, mirroring how `st` above does not
  // itself check that a picker exists to be staged - the value's own
  // presence is what names the variant. "v1" (the default) carries no
  // suffix, matching `st`'s own no-suffix-for-the-default rule.
  const criteria = process.env["ORCHESTRA_JEV_CRITERIA"] === "v2" ? "-v2" : "";
  // The v3 Jev trial (2026-09-17, "a noul refusal gate in front of the
  // local pick"): ORCHESTRA_GATE, read the same direct way as
  // ORCHESTRA_PICKER/ORCHESTRA_JEV_CRITERIA just above - the gate is
  // chosen through the platform's own environment, not a run.ts flag.
  // Only "jev" carries a suffix; "none" (the default) does not, matching
  // every other suffix here.
  const gate = process.env["ORCHESTRA_GATE"] === "jev" ? "-gate" : "";
  // The full-catalogue Jev trial's own follow-up (2026-09-18, "let Jev
  // choose the service"): ORCHESTRA_SERVICE_ROUTER, read the same direct
  // way as ORCHESTRA_GATE just above. Only "jev" carries a suffix; "none"
  // (the default) does not, matching every other suffix here.
  const router = process.env["ORCHESTRA_SERVICE_ROUTER"] === "jev" ? "-router" : "";
  // The service router's own second criteria form (2026-09-18, "ops:
  // every operation, not just a few names"): ORCHESTRA_SERVICE_ROUTER_CRITERIA,
  // read the same direct way as ORCHESTRA_SERVICE_ROUTER just above.
  // Appended after "-router" so the two forms' own output files never
  // collide; only "ops" carries a suffix, "names" (the default) does
  // not, matching every other suffix here.
  const routerCriteria = process.env["ORCHESTRA_SERVICE_ROUTER_CRITERIA"] === "ops" ? "-ops" : "";
  // The fill-stage experiment's own two arms (docs/measurements/jev-conditions.md):
  // ORCHESTRA_FILL_ENUM and ORCHESTRA_FILL_SKIP_EMPTY, read the same direct
  // way as ORCHESTRA_GATE/ORCHESTRA_SERVICE_ROUTER just above - each arm is
  // chosen through the platform's own environment, not a run.ts flag, and
  // the two are independent of each other and of every suffix above, so
  // both can appear together.
  const fillEnum = process.env["ORCHESTRA_FILL_ENUM"] === "jev" ? "-fillenum" : "";
  const skipEmpty = process.env["ORCHESTRA_FILL_SKIP_EMPTY"] === "1" ? "-skipempty" : "";
  // Arm 2's own two independent option-set changes (today's measurement,
  // docs/measurements/jev-conditions.md): ORCHESTRA_FILL_ENUM_REFUSAL (a
  // third sentinel option, "this operation cannot answer the question at
  // all") and ORCHESTRA_FILL_ENUM_UNSET_WORDING (the __unset__ criterion's
  // own wording), read the same direct way as ORCHESTRA_FILL_ENUM/
  // ORCHESTRA_FILL_SKIP_EMPTY just above. Only meaningful alongside
  // "-fillenum", but appended whenever the env var itself says so,
  // matching every other suffix here; "narrow" (the default) carries no
  // suffix, matching ORCHESTRA_SERVICE_ROUTER_CRITERIA's own
  // "names"-carries-no-suffix rule. ORCHESTRA_FILL_ENUM_REFUSAL's own third
  // value, "separate" (the whole-request judgements asked as their own
  // questions instead of in-options sentinels - the category-error fix for
  // the two ListInventoryItems rows docs/measurements/jev-conditions.md
  // captured), gets its own "-refusalsep" suffix rather than reusing
  // "-refusal": the two modes send a materially different request shape, so
  // their own output files must never collide.
  const refusalEnv = process.env["ORCHESTRA_FILL_ENUM_REFUSAL"];
  const refusal = refusalEnv === "1" ? "-refusal" : refusalEnv === "separate" ? "-refusalsep" : "";
  const unsetWording =
    process.env["ORCHESTRA_FILL_ENUM_UNSET_WORDING"] === "wide" ? "-unsetwide" : "";
  // A model comparison's own flag (boot.ts's ORCHESTRA_LLM_MODEL override):
  // read the same direct way as every other env-sourced suffix above. Only
  // a model other than the platform's own default, "qwen3.5-9b-q8", carries
  // a suffix - the default (set explicitly or left unset) carries none,
  // matching every other suffix here. Sanitised to `[a-z0-9.-]` so a model
  // name can never break the output filename it becomes part of.
  const DEFAULT_LLM_MODEL = "qwen3.5-9b-q8";
  const llmModel = process.env["ORCHESTRA_LLM_MODEL"];
  const model =
    llmModel === undefined || llmModel === DEFAULT_LLM_MODEL
      ? ""
      : `-model-${llmModel.toLowerCase().replaceAll(/[^a-z0-9.-]/gu, "-")}`;
  // The Anthropic thinking-comparison flag (docs/measurements: a
  // 2026-09-15 round found thinking bought nothing on this task; the tree
  // has changed since): ORCHESTRA_ANTHROPIC_THINKING, read the same direct
  // way as ORCHESTRA_LLM_MODEL just above - the platform's own environment,
  // not a run.ts flag. Only "on" carries a suffix; "off" (the default) and
  // unset do not, matching every other suffix here, so a thinking-on run's
  // output files never collide with the thinking-off ones.
  const anthropicThinking = process.env["ORCHESTRA_ANTHROPIC_THINKING"] === "on" ? "-think" : "";
  // The fill budget override for a thinking-on comparison (2026-09-19,
  // docs/specs/shortlisting.md: the fixed 1024-token fill budget truncated
  // 34 of 176 calls with thinking on, and the corpus score fell 81 -> 56):
  // ORCHESTRA_PLANNER_MAX_TOKENS, read the same direct way as
  // ORCHESTRA_ANTHROPIC_THINKING just above - the platform's own
  // environment, not a run.ts flag. Any value carries its own "-mt<value>"
  // suffix, unlike every boolean-ish suffix above, so a bigger-budget run's
  // output files never collide with the platform's own default (1024,
  // unset).
  const maxTokensEnv = process.env["ORCHESTRA_PLANNER_MAX_TOKENS"];
  const maxTokens = maxTokensEnv === undefined ? "" : `-mt${maxTokensEnv}`;

  return `${nothink}${rp}${st}${jev}${criteria}${gate}${router}${routerCriteria}${fillEnum}${skipEmpty}${refusal}${unsetWording}${model}${anthropicThinking}${maxTokens}`;
}
