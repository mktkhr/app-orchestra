import { existsSync, readdirSync } from "node:fs";
import path from "node:path";

import { readResults } from "./parse-result.ts";
import { renderReport, renderWordingReport, type WordingRunReport } from "./report.ts";
import { scoreboard } from "./score.ts";

/**
 * Prints the shortlist report from whichever output files run.ts wrote
 * (docs/plans/shortlisting.md Task 4, Step 5; docs/plans/wording.md Task
 * 2, Step 3): the plain `on.jsonl` / `off.jsonl` pair when both are
 * present, and a block per `on-<name>.jsonl` wording file when any exist.
 * `make eval-shortlist` runs `run.ts` then this - kept separate so a
 * report can be reprinted from an already-finished run without re-running
 * any question.
 */

const outDir = path.join(import.meta.dirname, "out");

/** Every `on-<name>.jsonl` wording file's name, in directory order. */
function wordingNames(): readonly string[] {
  if (!existsSync(outDir)) return [];

  return readdirSync(outDir)
    .map((entry) => /^on-(.+)\.jsonl$/u.exec(entry)?.[1])
    .filter((name): name is string => name !== undefined);
}

function printWordingReport(names: readonly string[]): void {
  const runs: readonly WordingRunReport[] = names.map((name) => {
    const results = readResults(path.join(outDir, `on-${name}.jsonl`));

    return { name, board: scoreboard(results), results };
  });

  console.log(renderWordingReport(runs));
}

function printPlainReport(): void {
  const on = readResults(path.join(outDir, "on.jsonl"));
  const off = readResults(path.join(outDir, "off.jsonl"));

  console.log(
    renderReport({
      on: { board: scoreboard(on), results: on },
      off: { board: scoreboard(off), results: off },
    }),
  );
}

const names = wordingNames();

if (names.length > 0) printWordingReport(names);

if (existsSync(path.join(outDir, "on.jsonl")) && existsSync(path.join(outDir, "off.jsonl"))) {
  printPlainReport();
}
