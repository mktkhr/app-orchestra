/**
 * Renders the shortlist measurement (docs/plans/shortlisting.md Task 4,
 * Step 5): a table per run (narrowing on, narrowing off), axes as columns,
 * beside two fixed reference rows read from DECISIONS.md's 2026-09-15
 * entries - the stand-in picker's own number, and the recall@20 ceiling a
 * shortlist puts under any picker. No I/O here: `run.ts` writes the
 * scoreboards this reads, `make eval-shortlist` prints what this returns.
 */
import type { LatencyStats, Scoreboard } from "./score.ts";

/** One run's label, as it should head its own table. */
export type RunLabel = "narrowing on" | "narrowing off";

/** A table's columns, in the fixed order every row (a run's or a reference's) uses. */
type Column = "A" | "B" | "C" | "D" | "E" | "overall";

const COLUMNS: readonly Column[] = ["A", "B", "C", "D", "E", "overall"];

/**
 * The stand-in picker's row, `pick:e5-large-q8+reranker` at K=20
 * (DECISIONS.md, 2026-09-15, "The local picker reads the shortlist in
 * reranker order: 78 → 83, for nothing"): 83 correct overall, A100 B100
 * C72 D53 E70.
 */
export const STAND_IN_PICKER_ROW: Readonly<Record<Column, number>> = {
  A: 100,
  B: 100,
  C: 72,
  D: 53,
  E: 70,
  overall: 83,
};

/**
 * recall@20 for `e5-large-q8+reranker(w)+written` (DECISIONS.md,
 * 2026-09-15, "The reranker half held, cleanly."): the reranker reads the
 * operation's own written examples, not just retrieval. This is the exact
 * row the plan asks for - no fallback was needed, it is on record at K=20.
 */
export const RECALL_AT_20_ROW: Readonly<Record<Column, number>> = {
  A: 96,
  B: 96,
  C: 84,
  D: 93,
  E: 100,
  overall: 93,
};

export const RECALL_AT_20_FALLBACK_USED = false;

/** One line per metric, columns A-E then overall, formatted as a fixed-width table row. */
function row(label: string, values: Readonly<Record<Column, number>>): string {
  const cells = COLUMNS.map((k) => String(values[k]).padStart(5));

  return `${label.padEnd(28)}${cells.join("")}`;
}

/** A percentage rate (0..1) rendered as a whole-number percent, rounded. */
function pct(rate: number): number {
  return Math.round(rate * 100);
}

function header(): string {
  const cols = COLUMNS.map((c) => c.padStart(5)).join("");

  return `${"".padEnd(28)}${cols}`;
}

function scoreboardRows(board: Scoreboard): string {
  const at1: Record<Column, number> = {
    A: pct(board.byAxis.A.correctAt1Rate),
    B: pct(board.byAxis.B.correctAt1Rate),
    C: pct(board.byAxis.C.correctAt1Rate),
    D: pct(board.byAxis.D.correctAt1Rate),
    E: pct(board.byAxis.E.correctAt1Rate),
    overall: pct(board.overall.correctAt1Rate),
  };
  const atShown: Record<Column, number> = {
    A: pct(board.byAxis.A.correctAtShownRate),
    B: pct(board.byAxis.B.correctAtShownRate),
    C: pct(board.byAxis.C.correctAtShownRate),
    D: pct(board.byAxis.D.correctAtShownRate),
    E: pct(board.byAxis.E.correctAtShownRate),
    overall: pct(board.overall.correctAtShownRate),
  };

  return [
    row("correct@1 (higher better)", at1),
    row("correct@shown (higher better)", atShown),
  ].join("\n");
}

function latencyLines(latency: LatencyStats): string {
  return (
    `latency (ms, lower is better): mean ${latency.meanMs.toFixed(0)}, ` +
    `p50 ${latency.p50Ms.toFixed(0)}, max ${latency.maxMs.toFixed(0)}, ` +
    `over 5000ms: ${String(latency.over5000ms)}`
  );
}

/** Renders one run's table: correct@1 and correct@shown per axis and overall, plus latency. */
export function renderRun(label: RunLabel, board: Scoreboard): string {
  return [`## ${label}`, header(), scoreboardRows(board), latencyLines(board.latency)].join("\n");
}

/** Renders the two fixed reference rows, from DECISIONS.md's 2026-09-15 entry. */
export function renderReferenceRows(): string {
  const lines = [
    "## reference (DECISIONS.md, 2026-09-15)",
    header(),
    row("pick:e5-large-q8+reranker (correct, K=20)", STAND_IN_PICKER_ROW),
    row("recall@20, +reranker(w)+written", RECALL_AT_20_ROW),
  ];

  if (RECALL_AT_20_FALLBACK_USED) {
    lines.push(
      "note: e5-large-q8+reranker(w)+written was not on record; the recall@20 row above falls back to e5-large-q8+reranker+written.",
    );
  }

  return lines.join("\n");
}

/** Renders both runs and the reference rows as one report. */
export function renderReport(runs: { readonly on: Scoreboard; readonly off: Scoreboard }): string {
  return [
    renderRun("narrowing on", runs.on),
    "",
    renderRun("narrowing off", runs.off),
    "",
    renderReferenceRows(),
  ].join("\n");
}
