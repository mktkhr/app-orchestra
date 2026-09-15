/**
 * Renders the shortlist measurement (docs/plans/shortlisting.md Task 4,
 * Step 5): a table per run (narrowing on, narrowing off), axes as columns,
 * beside two fixed reference rows read from DECISIONS.md's 2026-09-15
 * entries - the stand-in picker's own number, and the recall@20 ceiling a
 * shortlist puts under any picker. No I/O here: `run.ts` writes the
 * scoreboards this reads, `make eval-shortlist` prints what this returns.
 */
import { AXES, type LatencyStats, type QuestionResult, type Scoreboard } from "./score.ts";

/** One run's label, as it should head its own table - "narrowing on" / "narrowing off" for the plain passes, "wording <name>" for a per-wording pass (docs/plans/wording.md Task 2, Step 3). */
export type RunLabel = string;

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

/** Every cell's width: wide enough for "overall", the longest column header. */
const CELL_WIDTH = "overall".length + 1;

/** One line per metric, columns A-E then overall, formatted as a fixed-width table row. */
function row(label: string, values: Readonly<Record<Column, number>>): string {
  const cells = COLUMNS.map((k) => String(values[k]).padStart(CELL_WIDTH));

  return `${label.padEnd(28)}${cells.join("")}`;
}

/** A percentage rate (0..1) rendered as a whole-number percent, rounded. */
function pct(rate: number): number {
  return Math.round(rate * 100);
}

function header(): string {
  const cols = COLUMNS.map((c) => c.padStart(CELL_WIDTH)).join("");

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

/** One line breaking the run's over-5000ms count down by axis (AC-H-107 - the total alone hides which axis paid for it). */
function over5000ByAxisLine(board: Scoreboard): string {
  const perAxis = AXES.map(
    (axis) => `${axis}=${String(board.latencyByAxis[axis].over5000ms)}`,
  ).join(" ");

  return `over 5000ms by axis: ${perAxis}`;
}

/**
 * One line counting where each `result` row's answer came from
 * (`QuestionResult.via`): a genuine 200 `/api/plan` response ("plan") or
 * the honest fallback read out of a 500 `invoking ...` message
 * ("invoke-500") - so a run where every result is `invoke-500` is
 * visibly not measuring the render path, not silently passing.
 */
export function renderVia(label: RunLabel, results: readonly QuestionResult[]): string {
  const withVia = results.filter((r) => r.via !== undefined);
  const plan = withVia.filter((r) => r.via === "plan").length;
  const invoke500 = withVia.filter((r) => r.via === "invoke-500").length;

  return (
    `${label} result via: plan=${String(plan)} invoke-500=${String(invoke500)} ` +
    `(of ${String(withVia.length)} result rows)`
  );
}

/** Every error row in results, with its question, latency and the platform's own message - not just counted (score.ts's Tally), listed. */
export function renderErrors(label: RunLabel, results: readonly QuestionResult[]): string {
  const errors = results.filter((r) => r.kind === "error");

  if (errors.length === 0) return `## ${label} errors\n(none)`;

  const lines = errors.map((r) => {
    const status = r.errorStatus === undefined ? "" : ` (HTTP ${String(r.errorStatus)})`;
    const message = r.errorMessage ?? "(no message recorded)";

    return `${r.id} ${r.axis} "${r.text}" ${String(r.latencyMs)}ms${status}: ${message}`;
  });

  return [`## ${label} errors (${String(errors.length)})`, ...lines].join("\n");
}

/** Renders one run's table: correct@1 and correct@shown per axis and overall, plus latency. */
export function renderRun(label: RunLabel, board: Scoreboard): string {
  return [
    `## ${label}`,
    header(),
    scoreboardRows(board),
    latencyLines(board.latency),
    over5000ByAxisLine(board),
  ].join("\n");
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

/** One run's scoreboard plus the raw results it was scored from - renderReport needs the raw results for renderErrors. */
export interface RunReport {
  readonly board: Scoreboard;
  readonly results: readonly QuestionResult[];
}

/** Renders both runs, their error lists, and the reference rows as one report. */
export function renderReport(runs: { readonly on: RunReport; readonly off: RunReport }): string {
  return [
    renderRun("narrowing on", runs.on.board),
    renderVia("narrowing on", runs.on.results),
    renderErrors("narrowing on", runs.on.results),
    "",
    renderRun("narrowing off", runs.off.board),
    renderVia("narrowing off", runs.off.results),
    renderErrors("narrowing off", runs.off.results),
    "",
    renderReferenceRows(),
  ].join("\n");
}

/** One named wording's scoreboard plus the raw results it was scored from (docs/plans/wording.md Task 2, Step 3). */
export interface WordingRunReport {
  readonly name: string;
  readonly board: Scoreboard;
  readonly results: readonly QuestionResult[];
}

/** `runs`, `v1` moved first if present - `Array.prototype.sort` is stable, so every other name keeps its given relative order. */
function v1First(runs: readonly WordingRunReport[]): readonly WordingRunReport[] {
  return runs.toSorted((a, b) => Number(b.name === "v1") - Number(a.name === "v1"));
}

/**
 * Renders one block per wording - `v1` first - the same columns as
 * `renderReport`'s two blocks, plus the stand-in picker's reference row
 * once at the bottom (docs/plans/wording.md Task 2, Step 3; AC-Q-103).
 */
export function renderWordingReport(runs: readonly WordingRunReport[]): string {
  const blocks = v1First(runs).flatMap((r) => {
    const label = `wording ${r.name}`;

    return [
      renderRun(label, r.board),
      renderVia(label, r.results),
      renderErrors(label, r.results),
      "",
    ];
  });

  return [...blocks, renderReferenceRows()].join("\n");
}
