import { existsSync, readdirSync } from "node:fs";
import path from "node:path";

import { midCatalog } from "../narrowing/fixture/index.ts";
import { readResults } from "./parse-result.ts";
import { pickMeanMs } from "./pick-log.ts";
import { renderMidSection } from "./report-mid.ts";
import { renderReport, renderWordingReport, type WordingRunReport } from "./report.ts";
import { scoreMid } from "./score-mid.ts";
import { scoreboard } from "./score.ts";

/**
 * Prints the shortlist report from whichever output files run.ts wrote
 * (docs/plans/shortlisting.md Task 4, Step 5; docs/plans/wording.md Task
 * 2, Step 3): the plain `on.jsonl` / `off.jsonl` pair when both are
 * present, and a block per `on-<name>.jsonl` wording file when any exist.
 * `make eval-shortlist` runs `run.ts` then this - kept separate so a
 * report can be reprinted from an already-finished run without re-running
 * any question.
 *
 * Also prints the mid section (docs/plans/midsizing.md Task 3) for every
 * `mid-<variant>.jsonl` output file `run.ts --corpus mid` wrote, alongside
 * whatever shortlist reports above - `make eval-mid` runs `run.ts
 * --corpus mid` then this same script.
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

/** Every `mid-<variant>.jsonl` file's variant name, in directory order. */
function midVariantNames(): readonly string[] {
  if (!existsSync(outDir)) return [];

  return readdirSync(outDir)
    .map((entry) => /^mid-(.+)\.jsonl$/u.exec(entry)?.[1])
    .filter((name): name is string => name !== undefined);
}

function printMidReport(names: readonly string[]): void {
  const catalogIds = new Set(midCatalog().map((operation) => operation.operationId));

  for (const name of names) {
    const rows = readResults(path.join(outDir, `mid-${name}.jsonl`));
    const board = scoreMid(rows, catalogIds);
    const pickMean = pickMeanMs(path.join(outDir, `mid-${name}.log`));

    console.log(`# mid ${name}`);
    console.log(renderMidSection(board, pickMean));
  }
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

const midNames = midVariantNames();

if (midNames.length > 0) printMidReport(midNames);
