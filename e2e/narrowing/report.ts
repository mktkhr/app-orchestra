/**
 * Prints `measure.ts`'s results (docs/plans/narrowing.md Task 4, Step 5;
 * docs/plans/retrieving.md Task 2, Step 4; AC-V-104). Pure formatting:
 * everything here reads a `ConfigurationResult[]` and calls `console.log`;
 * the recall computation itself lives in `measure.ts`.
 *
 * One block per configuration, in a consistent column order, rather than
 * one wide table: six embedding configurations plus the lexical floor,
 * each at four catalogue sizes and three K values, does not fit across a
 * terminal as a single table (docs/plans/retrieving.md, "the report is
 * getting wide"). A configuration's contract-check rate (AC-V-101) is
 * printed once, in its block's header, beside the recall it sits above -
 * not as a per-row column, and not as a gate: a configuration that fails
 * often is not read as evidence about its model (spec section 4).
 */
import type {
  AxisRecall,
  ConfigurationResult,
  K,
  RecallAtK,
  SizeResult,
  SkippedConfiguration,
} from "./measure.ts";

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
 * The contract-check line for a configuration's block header, or "" for the
 * lexical floor, which has no contract to check. AC-V-101: the rate is
 * printed, not a pass/fail label — reading it is left to whoever reads the
 * recall it sits beside (spec section 4).
 */
function contractLine(configuration: ConfigurationResult): string {
  const check = configuration.contractCheck;

  if (check === undefined) return "";

  const failed = check.failedOperationIds.length;
  const sampled = check.sampledOperationIds.length;

  return (
    ` — contract check (AC-V-101, retrieves itself by its own text): fails ${String(failed)}/${String(sampled)}` +
    " — a rate, not a gate (docs/specs/retrieving.md section 4)"
  );
}

/** One configuration's block: its header (id and contract-check rate), the shared column header, then one size-block per catalogue size. */
function printConfiguration(configuration: ConfigurationResult, kValues: readonly K[]): void {
  console.log("");
  console.log(`== ${configuration.configId} ==${contractLine(configuration)}`);
  console.log(
    "size  ops    K    " + AXIS_COLUMNS.map((axis) => axis.padEnd(10)).join("") + "ms/query",
  );

  for (const size of configuration.sizes) printSize(size, kValues);
}

/** The configurations that could not be measured at all, and why (docs/plans/retrieving.md Task 2 Step 5). */
function printSkipped(skipped: readonly SkippedConfiguration[]): void {
  console.log("");
  console.log("skipped (llama-swap unreachable) — the lexical row above still ran:");

  for (const entry of skipped) console.log(`  ${entry.configId}: ${entry.reason}`);
}

/**
 * Prints the full report: a header stating what each number means and which
 * direction is better (docs/specs/narrowing.md section 6; the model
 * comparison this repeats the mistake of not doing is `DECISIONS.md`,
 * 2026-09-14, "nine local models on one corpus"), then one block per
 * configuration — the lexical floor first, then every embedding
 * configuration that could be reached (AC-V-104) — and finally the
 * configurations that could not be.
 */
export function printReport(
  configurations: readonly ConfigurationResult[],
  skipped: readonly SkippedConfiguration[],
  kValues: readonly K[],
): void {
  console.log(
    "recall@K over the fixture catalogue (docs/specs/narrowing.md section 6, " +
      "docs/specs/retrieving.md section 6). Higher is better — 100% means every " +
      "eligible question's answer was inside the top K. One block per " +
      "configuration below; every block uses the same column order.",
  );
  console.log(
    "Each cell is worst%/best% out of the questions eligible at that size " +
      "(excluded questions are listed below, not folded into the denominator). " +
      "worst counts an answer recalled only if it is still inside K even when " +
      "every operation tied with it outranks it (a tie is not a hit); best " +
      "counts it if the most favourable tie order would put it inside K. A " +
      `cell marked * differs by ${String(COIN_TOSS_GAP_POINTS)} points or more between the two — that much ` +
      "of the number is a coin toss, not a measurement of the mechanism. " +
      "Embedding vectors are floats and rarely tie, so the two columns " +
      "usually collapse onto each other for embedding configurations; both " +
      "are still printed for every configuration.",
  );
  console.log("ms/query is per-query wall-clock for the narrowing call itself, no LLM involved.");

  for (const configuration of configurations) printConfiguration(configuration, kValues);
  if (skipped.length > 0) printSkipped(skipped);
}
