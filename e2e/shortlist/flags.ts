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
 * The `-nothink` / `-rp<value>` / `-stages2` / `-jev` suffix a variant's
 * output files carry (run.ts's own doc comment, docs/plans/staging.md;
 * docs/plans/midsizing.md Task 3 reuses it for `--corpus mid`'s
 * `mid-<variant>.jsonl`) - "" for a plain wording pass, unchanged from
 * before any of the three flags existed. `stages` of `1` or `undefined`
 * carries no suffix - `STAGES=1` reproduces the plain pass byte for byte
 * (docs/plans/staging.md, AC-S-101).
 *
 * `-jev` is read straight from `ORCHESTRA_JEV_API_KEY`, unlike the other
 * three which come from `run.ts`'s own `--flag` parsing above: the
 * picker is chosen through the platform's own environment
 * (`ORCHESTRA_PICKER`, `e2e/eval/services.ts`/`e2e/shortlist/boot.ts`
 * pass it through the same way), not a run.ts flag, so this reads that
 * one environment variable directly rather than growing a fourth
 * parameter every call site would have to thread through for a name
 * nothing else here needs.
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
  const jev = process.env["ORCHESTRA_PICKER"] === "jev" ? "-jev" : "";
  // The Jev trial's second round (2026-09-17, "v2: richer criteria"):
  // ORCHESTRA_JEV_CRITERIA, read the same direct way as ORCHESTRA_PICKER
  // just above. Only meaningful alongside "-jev" (jev), but this suffix
  // is appended whenever ORCHESTRA_JEV_CRITERIA is literally "v2"
  // regardless of ORCHESTRA_PICKER, mirroring how `st` above does not
  // itself check that a picker exists to be staged - the value's own
  // presence is what names the variant. "v1" (the default) carries no
  // suffix, matching `st`'s own no-suffix-for-the-default rule.
  const criteria = process.env["ORCHESTRA_JEV_CRITERIA"] === "v2" ? "-v2" : "";

  return `${nothink}${rp}${st}${jev}${criteria}`;
}
