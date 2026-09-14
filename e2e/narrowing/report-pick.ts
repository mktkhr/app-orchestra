/**
 * Prints the three pick rows (TODO.md "Measure the pick, not only the
 * recall"; `gather-pick.ts`) - pulled out of `report.ts` only for eslint's
 * `max-lines`, the same reason `recall.ts` and `report-types.ts` were split
 * out of `measure.ts`. `report.ts`'s `printReport` calls `printPickRows`
 * exactly where it prints every other section.
 */
import type { PickAxisResult, PickConfigurationResult, PickSizeResult } from "./report-types.ts";
import { PICK_SHORTLIST_K } from "./gather-pick-shortlists.ts";

const AXIS_COLUMNS: readonly ("A" | "B" | "C" | "D" | "E" | "overall")[] = [
  "A",
  "B",
  "C",
  "D",
  "E",
  "overall",
];

/** A percentage, rounded to the nearest whole point, or "n/a" when there is nothing to divide by — the same rule `report.ts`'s own `percent` uses. */
function percent(hits: number, total: number): string {
  return total === 0 ? "n/a" : `${String(Math.round((hits / total) * 100))}%`;
}

/** One pick axis's cell: "correct%/flagged%". */
function pickCellOf(axis: PickAxisResult): string {
  return `${percent(axis.correct, axis.total)}/${percent(axis.flagged, axis.total)}`.padEnd(10);
}

/** The `PickAxisResult` for one axis out of a pick size's results, or a zeroed placeholder if it is somehow missing. */
function pickAxisAt(size: PickSizeResult, axis: (typeof AXIS_COLUMNS)[number]): PickAxisResult {
  const found = size.axisResults.find((entry) => entry.axis === axis);

  return found ?? { axis, total: 0, excluded: 0, correct: 0, flagged: 0 };
}

/** One pick row's one catalogue size: its excluded-question note, then its one row of axis cells and the picker's own per-query wall-clock. */
function printPickSize(size: PickSizeResult): void {
  const totalExcluded = size.axisResults
    .filter((axis) => axis.axis !== "overall")
    .reduce((sum, axis) => sum + axis.excluded, 0);
  const cells = AXIS_COLUMNS.map((axis) => pickCellOf(pickAxisAt(size, axis)));

  console.log(
    `  size ${String(size.size)} (${String(size.operationCount)} ops): ` +
      `${String(totalExcluded)} of 100 questions excluded (no answer in this catalogue)`,
  );
  console.log(
    `${String(size.size).padEnd(6)}${String(size.operationCount).padEnd(7)}` +
      cells.join("") +
      size.averageQueryMillis.toFixed(3),
  );
}

/** One pick row's block: its header, the shared column header, then one line per catalogue size. */
function printPickConfiguration(configuration: PickConfigurationResult): void {
  console.log("");
  console.log(`== ${configuration.configId} ==`);
  console.log("size  ops    " + AXIS_COLUMNS.map((axis) => axis.padEnd(10)).join("") + "ms/query");

  for (const size of configuration.sizes) printPickSize(size);
}

/**
 * The three pick rows: each cell is correct%/flagged% out of the questions
 * eligible at that size - correct means the picked operationId is among the
 * question's answers; flagged means the picker returned `ambiguous`. On
 * axis B (the genuinely ambiguous questions) a high flagged% is desired
 * behaviour; on axis A a high flagged% is a false alarm. Both percentages
 * are printed for every row; which one matters at a given axis is left to
 * the reader. Prints nothing when there are no pick rows to show (llama-swap
 * was unreachable - `report.ts`'s `printSkipped` names them instead).
 */
export function printPickRows(pickConfigurations: readonly PickConfigurationResult[]): void {
  if (pickConfigurations.length === 0) return;

  console.log("");
  console.log(
    `pick@K=${String(PICK_SHORTLIST_K)} (DECISIONS.md, 2026-09-15, "The local picker reads the ` +
      'shortlist in reranker order"): the local picker (qwen3.5-9b-q8, thinking off) chooses one ' +
      "operationId from the top K reranked candidates. Each cell is correct%/flagged% out of the " +
      "questions eligible at that size. On axis B (the genuinely ambiguous questions) a high " +
      "flagged% is desired behaviour; on axis A a high flagged% is a false alarm — both are " +
      "printed for every row, and which one matters is left to the reader.",
  );

  for (const configuration of pickConfigurations) printPickConfiguration(configuration);
}
