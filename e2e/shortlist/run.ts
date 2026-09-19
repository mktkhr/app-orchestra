import {
  corpusArg,
  narrowingArg,
  repeatPenaltyArg,
  stagesArg,
  thinkingArg,
  wordingNames,
} from "./flags.ts";
import { runMid } from "./run-mid.ts";
import { runShortlist } from "./run-shortlist.ts";

/**
 * `run.ts`'s own entry point: parses the command-line flags and dispatches
 * to `runShortlist` (the 100-question narrowing corpus, or, with
 * `--corpus ext`, the 50-question axis D/E extension corpus) or `runMid`
 * (`--corpus mid`, docs/plans/midsizing.md Task 3, the thirty-operation mid
 * subset's sixty questions). Split into a thin dispatcher plus
 * `run-shortlist.ts` / `run-mid.ts` so no one file needs more than ten
 * dependencies or 300 lines (harness/quality/file-length.txt, the
 * import/max-dependencies lint rule).
 *
 * Run with `node shortlist/run.ts` (invoked by `make eval-shortlist`),
 * `node shortlist/run.ts --corpus mid` (`make eval-mid`) or `node
 * shortlist/run.ts --corpus ext` (the extension corpus, `ext-` output
 * prefix) - every other flag (`--wording`, `--thinking`,
 * `--repeat-penalty`, `--stages`, `--on-only`/`--off-only`) is
 * `runShortlist`'s and `runMid`'s own, see their doc comments.
 */
async function main(): Promise<void> {
  const names = wordingNames();
  const thinking = thinkingArg();
  const repeatPenalty = repeatPenaltyArg();
  const stages = stagesArg();
  const corpus = corpusArg();
  const narrowing = narrowingArg();

  if (corpus === "mid") {
    // `--corpus mid` composes with `--wording` / `--thinking` / `--stages`
    // (docs/plans/midsizing.md Task 3): one mid pass per named wording, or
    // one plain "default" pass when `--wording` was not given. No `v1`
    // forcing here - that is `runShortlist`'s own baseline convention for
    // the shortlist corpus, not part of the mid measurement
    // (docs/specs/midsizing.md M5/M6). `--narrowing off` is the full-catalogue
    // Jev trial's own flag (internal/adapter/planner/jev/hierarchical.go);
    // `runShortlist` honours it too, as a variant pass whose files carry a
    // `-narrowing-off` suffix, so a picker run never overwrites the plain
    // `--off-only` pass's own `off.jsonl`.
    await runMid(names, thinking, repeatPenalty, stages, narrowing);

    return;
  }

  // `corpus` is "ext" or undefined here - "mid" already returned above.
  // `runShortlist` reads it to switch to `extensionQuestions()` and the
  // `ext-` output prefix, otherwise behaving byte-identically to before
  // this flag existed.
  await runShortlist(names, thinking, repeatPenalty, stages, narrowing, corpus);
}

await main();
