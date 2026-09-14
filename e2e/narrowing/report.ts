/**
 * Prints `measure.ts`'s results (docs/plans/narrowing.md Task 4, Step 5).
 * Pure formatting: everything here reads a `SizeResult[]` and calls
 * `console.log`; the recall computation itself lives in `measure.ts`.
 */
import type { AxisRecall, K, RecallAtK, SizeResult } from "./measure.ts";

const AXIS_COLUMNS: readonly ("A" | "B" | "C" | "D" | "E" | "overall")[] = [
  "A",
  "B",
  "C",
  "D",
  "E",
  "overall",
];

/**
 * A gap this wide between the pessimistic and optimistic figure is the
 * report saying the number is close to a coin toss (spec section 6) rather
 * than noise — chosen well above what two questions flipping at n=25 could
 * produce (8 points), so a flagged line means the *tie*, not the sample
 * size, is doing the work.
 */
const COIN_TOSS_GAP_POINTS = 15;

/** A percentage, rounded to the nearest whole point, or "n/a" when there is nothing to divide by. */
function percent(hits: number, total: number): string {
  return total === 0 ? "n/a" : `${String(Math.round((hits / total) * 100))}%`;
}

/** One axis's cell at one K: "worst%/best%", starred when the gap is wide enough to read as a coin toss. */
function cellOf(axis: AxisRecall, recall: RecallAtK): string {
  const worst = percent(recall.pessimisticHits, axis.total);
  const best = percent(recall.optimisticHits, axis.total);
  const gapPoints =
    axis.total === 0 ? 0 : ((recall.optimisticHits - recall.pessimisticHits) / axis.total) * 100;
  const flag = gapPoints >= COIN_TOSS_GAP_POINTS ? "*" : " ";

  return `${worst}/${best}${flag}`.padEnd(10);
}

/** The `RecallAtK` for one K out of an axis's recalls, or a zeroed placeholder if `measure` was never asked for that K. */
function recallAt(axis: AxisRecall, k: K): RecallAtK {
  const found = axis.recalls.find((recall) => recall.k === k);

  return found ?? { k, pessimisticHits: 0, optimisticHits: 0 };
}

/** "A 3, B 25, C 1, D 0, E 2" — how many questions per axis had no answer in this catalogue. */
function excludedByAxisLine(axisRecalls: readonly AxisRecall[]): string {
  return axisRecalls
    .filter((axis) => axis.axis !== "overall")
    .map((axis) => `${axis.axis} ${String(axis.excluded)}`)
    .join(", ");
}

/** One (size, K) row: the axis columns, then overall, then the per-query wall-clock. */
function printRow(size: SizeResult, k: K): void {
  const cells = AXIS_COLUMNS.map((axis) => {
    const axisRecall = size.result.axisRecalls.find((entry) => entry.axis === axis);

    if (axisRecall === undefined) throw new Error(`report.ts: no axis recall for ${axis}`);

    return cellOf(axisRecall, recallAt(axisRecall, k));
  });

  console.log(
    `${String(size.size).padEnd(6)}${String(size.result.operationCount).padEnd(7)}${String(k).padEnd(5)}` +
      cells.join("") +
      size.result.averageQueryMillis.toFixed(3),
  );
}

/** One catalogue size: its excluded-question note, then one row per K. */
function printSize(size: SizeResult, kValues: readonly K[]): void {
  const totalExcluded = size.result.axisRecalls
    .filter((axis) => axis.axis !== "overall")
    .reduce((sum, axis) => sum + axis.excluded, 0);

  console.log(
    `  size ${String(size.size)} (${String(size.result.operationCount)} ops): ` +
      `${String(totalExcluded)} of 100 questions excluded (no answer in this catalogue) ` +
      `— by axis: ${excludedByAxisLine(size.result.axisRecalls)}`,
  );

  for (const k of kValues) printRow(size, k);
}

/**
 * Prints the full report: a header stating what each number means and which
 * direction is better (docs/specs/narrowing.md section 6; the model
 * comparison this repeats the mistake of not doing is `DECISIONS.md`,
 * 2026-09-14, "nine local models on one corpus"), then one block per
 * catalogue size.
 */
export function printReport(results: readonly SizeResult[], kValues: readonly K[]): void {
  console.log(
    "recall@K over the fixture catalogue (docs/specs/narrowing.md section 6). " +
      "Higher is better — 100% means every eligible question's answer was " +
      "inside the top K.",
  );
  console.log(
    "Each cell is worst%/best% out of the questions eligible at that size " +
      "(excluded questions are listed below, not folded into the denominator). " +
      "worst counts an answer recalled only if it is still inside K even when " +
      "every operation tied with it outranks it (a tie is not a hit); best " +
      "counts it if the most favourable tie order would put it inside K. A " +
      `cell marked * differs by ${String(COIN_TOSS_GAP_POINTS)} points or more between the two — that much ` +
      "of the number is a coin toss, not a measurement of the mechanism.",
  );
  console.log("ms/query is per-query wall-clock for the narrowing call itself, no LLM involved.");
  console.log("");
  console.log(
    "size  ops    K    " + AXIS_COLUMNS.map((axis) => axis.padEnd(10)).join("") + "ms/query",
  );

  for (const size of results) {
    printSize(size, kValues);
  }
}
