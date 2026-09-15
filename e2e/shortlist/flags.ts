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
