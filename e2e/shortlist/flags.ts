/**
 * `run.ts`'s own command-line flag parsing, split out only for eslint's
 * `max-lines` (docs/plans/staging.md, Task 3: adding `--stages` pushed
 * `run.ts` past the 300-line cap) - every function here reads
 * `process.argv` directly, exactly as it did inline in `run.ts` before
 * this split.
 */

import { envSuffix } from "./flags-env-suffix.ts";

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

/**
 * `--corpus mid|ext` (docs/plans/midsizing.md, Task 3; the 50-question axis
 * D/E extension corpus): the only corpus names this flag accepts besides
 * the default (unset, the 100-question shortlist corpus) - anything else
 * is a usage error, not a silent no-op. `ext` runs `extensionQuestions()`
 * (`e2e/narrowing/corpus/index.ts`) through the same path as the
 * 100-question corpus (`runShortlist`), with its own `ext-` output prefix
 * so its files never collide with the 100-question runs.
 */
export function corpusArg(): "mid" | "ext" | undefined {
  const raw = flagValue("--corpus");

  if (raw === undefined) return undefined;

  if (raw !== "mid" && raw !== "ext") {
    throw new Error(`--corpus must be "mid" or "ext", got ${JSON.stringify(raw)}`);
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
  const thinkSuffix = { on: "-think", off: "-nothink" } as const;
  const nothink = thinking === undefined ? "" : thinkSuffix[thinking];
  const rp = repeatPenalty === undefined ? "" : `-rp${String(repeatPenalty)}`;
  const st = stages === undefined || stages === 1 ? "" : `-stages${String(stages)}`;

  return `${nothink}${rp}${st}${envSuffix()}`;
}

/**
 * The plain on/off dual pass's output file base name (`run-shortlist.ts`'s
 * `runPass`) - `"on"`/`"off"` for the 100-question corpus, unchanged, or
 * `"ext-on"`/`"ext-off"` for `--corpus ext` (the 50-question axis D/E
 * extension corpus), so its files never collide with the 100-question
 * corpus's own.
 */
export function passOutputName(pass: "on" | "off", extCorpus: boolean): string {
  return extCorpus ? `ext-${pass}` : pass;
}

/**
 * A per-wording pass's output file base name (`run-shortlist.ts`'s
 * `runWordingPass`), given `variant` (the wording name plus
 * `variantSuffix`'s own suffix) - `"on-<variant>"` for the 100-question
 * corpus, unchanged, or `"ext-<variant>"` for `--corpus ext`.
 */
export function wordingOutputName(variant: string, extCorpus: boolean): string {
  return extCorpus ? `ext-${variant}` : `on-${variant}`;
}

/**
 * A per-wording pass's miss-list name (`run-shortlist.ts`'s
 * `runWordingPass`, `misses.ts`'s `writeMissesFromOutput`) - `variant`
 * itself, unprefixed, for the 100-question corpus (unchanged: the miss
 * list has never carried the `on-` prefix its own jsonl file does), or
 * `"ext-<variant>"` for `--corpus ext`, so its own `misses-ext-<variant>.txt`
 * never collides with the 100-question corpus's `misses-<variant>.txt` for
 * the same wording/variant name.
 */
export function missesVariant(variant: string, extCorpus: boolean): string {
  return extCorpus ? `ext-${variant}` : variant;
}
