/**
 * Pure scoring for the shortlist measurement (docs/plans/shortlisting.md
 * Task 4, Step 3; docs/specs/shortlisting.md H6, section 6, AC-H-106). No
 * I/O here - `run.ts` gathers `QuestionResult`s from a running platform,
 * `boot.ts` starts it; this file only turns those results into numbers, so
 * it is what `score.test.ts` exercises with fakes and what `make check`
 * runs without a server or a model.
 */
import type { Axis } from "../narrowing/corpus/index.ts";

export type { Axis };

/** The shape of one /api/plan answer, reduced to what scoring needs. */
export type Kind = "result" | "ask" | "none" | "form" | "proposal" | "error";

/** One question's outcome against a running platform, one run (on or off). */
export interface QuestionResult {
  readonly id: string;
  readonly axis: Axis;
  readonly answers: readonly string[];
  readonly kind: Kind;
  readonly operationId?: string;
  readonly alternatives?: readonly string[];
  readonly latencyMs: number;
  readonly narrowingMs?: number;
}

/** One question's outcome, scored. */
export interface Scored {
  readonly id: string;
  readonly axis: Axis;
  readonly correctAt1: boolean;
  readonly correctAtShown: boolean;
  readonly asked: boolean;
  readonly none: boolean;
  readonly error: boolean;
  readonly latencyMs: number;
}

/** Scores one question's result against its answer key. */
export function score(result: QuestionResult): Scored {
  const correctAt1 = result.kind === "result" && result.answers.includes(result.operationId ?? "");
  const correctAtShown =
    correctAt1 || (result.alternatives ?? []).some((id) => result.answers.includes(id));

  return {
    id: result.id,
    axis: result.axis,
    correctAt1,
    correctAtShown,
    asked: result.kind === "ask",
    none: result.kind === "none",
    error: result.kind === "error",
    latencyMs: result.latencyMs,
  };
}

/** Counts and rates over one group of scored questions (one axis, or overall). */
export interface Tally {
  readonly total: number;
  readonly correctAt1: number;
  readonly correctAtShown: number;
  readonly asked: number;
  readonly none: number;
  readonly error: number;
  readonly correctAt1Rate: number;
  readonly correctAtShownRate: number;
}

/** Aggregates a group of scored questions into a Tally. Zero-length group reads every rate as 0, not NaN. */
export function tally(scored: readonly Scored[]): Tally {
  const total = scored.length;
  const correctAt1 = scored.filter((s) => s.correctAt1).length;
  const correctAtShown = scored.filter((s) => s.correctAtShown).length;
  const asked = scored.filter((s) => s.asked).length;
  const none = scored.filter((s) => s.none).length;
  const error = scored.filter((s) => s.error).length;

  return {
    total,
    correctAt1,
    correctAtShown,
    asked,
    none,
    error,
    correctAt1Rate: total === 0 ? 0 : correctAt1 / total,
    correctAtShownRate: total === 0 ? 0 : correctAtShown / total,
  };
}

export const AXES: readonly Axis[] = ["A", "B", "C", "D", "E"];

/** One run's full scoreboard: overall and per-axis tallies, plus latency stats. */
export interface Scoreboard {
  readonly overall: Tally;
  readonly byAxis: Readonly<Record<Axis, Tally>>;
  readonly latency: LatencyStats;
}

export interface LatencyStats {
  readonly meanMs: number;
  readonly p50Ms: number;
  readonly maxMs: number;
  readonly over5000ms: number;
}

/** Mean, p50 and max of a run's per-question latencies, and how many exceeded 5000ms (AC-H-107). */
export function latencyStats(latenciesMs: readonly number[]): LatencyStats {
  if (latenciesMs.length === 0) {
    return { meanMs: 0, p50Ms: 0, maxMs: 0, over5000ms: 0 };
  }

  const sorted = latenciesMs.toSorted((a, b) => a - b);
  const mid = Math.floor(sorted.length / 2);
  // Even-length median averages the two middle values; odd-length reads the
  // single middle one - `sorted[mid - 1] ?? sorted[mid]` degrades to that
  // single value when mid - 1 is out of range (length 1: mid is 0).
  const p50 =
    sorted.length % 2 === 0
      ? ((sorted[mid - 1] ?? sorted[mid] ?? 0) + (sorted[mid] ?? 0)) / 2
      : (sorted[mid] ?? 0);

  return {
    meanMs: latenciesMs.reduce((a, b) => a + b, 0) / latenciesMs.length,
    p50Ms: p50,
    maxMs: Math.max(...latenciesMs),
    over5000ms: latenciesMs.filter((ms) => ms > 5000).length,
  };
}

/** Scores a whole run's results into a Scoreboard: overall, per-axis, and latency. */
export function scoreboard(results: readonly QuestionResult[]): Scoreboard {
  const scored = results.map((result) => score(result));
  const byAxis: Readonly<Record<Axis, Tally>> = {
    A: tally(scored.filter((s) => s.axis === "A")),
    B: tally(scored.filter((s) => s.axis === "B")),
    C: tally(scored.filter((s) => s.axis === "C")),
    D: tally(scored.filter((s) => s.axis === "D")),
    E: tally(scored.filter((s) => s.axis === "E")),
  };

  return {
    overall: tally(scored),
    byAxis,
    latency: latencyStats(results.map((r) => r.latencyMs)),
  };
}
