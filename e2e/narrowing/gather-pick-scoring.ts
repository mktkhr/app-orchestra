/**
 * Runs the picker over every shortlist `gather-pick-shortlists.ts` gathered,
 * in one pass, and turns the outcomes into the three pick rows'
 * `PickConfigurationResult`s (`gather-pick.ts`). One `PickGuardState` is
 * shared across the whole pass, so the thinking-budget guard
 * (`pick/client.ts`) checks the very first response of the run and never
 * again - not once per variant or per size.
 */
import { catalogOf } from "./fixture/index.ts";
import type { CatalogSize } from "./recall.ts";
import type { FetchLike } from "./embedding/client.ts";
import type { Axis, Question } from "./corpus/index.ts";
import type { PickAxisResult, PickConfigurationResult, PickSizeResult } from "./report-types.ts";
import {
  PICK_SYSTEM_PROMPT,
  PICK_SYSTEM_PROMPT_WITH_EXAMPLES,
  newPickGuardState,
  pick,
  type PickGuardState,
} from "./pick/index.ts";
import type { ShortlistedQuestion, VariantShortlists } from "./gather-pick-shortlists.ts";

const AXES: readonly Axis[] = ["A", "B", "C", "D", "E"];

interface PickOutcome {
  readonly question: Question;
  readonly correct: boolean;
  readonly flagged: boolean;
}

/** Whether at least one of `question`'s answers is an operation this catalogue actually serves — the same rule `recall.ts`'s `isEligible` uses. */
function isEligible(question: Question, catalogIds: ReadonlySet<string>): boolean {
  return question.answers.some((answerId) => catalogIds.has(answerId));
}

/**
 * Picks every row in order, timing each call — the picker's own
 * per-question wall-clock. `systemPrompt` is the same for every row in
 * `rows` (one per variant, chosen by the caller from `VariantShortlists.showExamples`).
 */
async function pickAllFor(
  rows: readonly ShortlistedQuestion[],
  guardState: PickGuardState,
  fetchImpl: FetchLike | undefined,
  baseUrl: string | undefined,
  systemPrompt: string,
): Promise<{ readonly outcomes: readonly PickOutcome[]; readonly averageQueryMillis: number }> {
  const outcomes: PickOutcome[] = [];
  const queryMillis: number[] = [];

  for (const row of rows) {
    const start = performance.now();
    const result = await pick(
      row.question.text,
      row.candidates,
      guardState,
      fetchImpl,
      baseUrl,
      systemPrompt,
    );

    queryMillis.push(performance.now() - start);
    outcomes.push({
      question: row.question,
      correct: row.question.answers.includes(result.operationId),
      flagged: result.ambiguous,
    });
  }

  const totalMillis = queryMillis.reduce((sum, ms) => sum + ms, 0);

  return {
    outcomes,
    averageQueryMillis: queryMillis.length === 0 ? 0 : totalMillis / queryMillis.length,
  };
}

/** One axis's (or `"overall"`'s) pick figures, out of `outcomes` already scoped to one catalogue size. */
function axisResultOf(
  axis: Axis | "overall",
  outcomes: readonly PickOutcome[],
  catalogIds: ReadonlySet<string>,
): PickAxisResult {
  const scoped = axis === "overall" ? outcomes : outcomes.filter((o) => o.question.axis === axis);
  const eligible = scoped.filter((o) => isEligible(o.question, catalogIds));

  return {
    axis,
    total: eligible.length,
    excluded: scoped.length - eligible.length,
    correct: eligible.filter((o) => o.correct).length,
    flagged: eligible.filter((o) => o.flagged).length,
  };
}

/** One catalogue size's `PickSizeResult`, from that size's already-picked outcomes. */
function sizeResultOf(
  size: CatalogSize,
  outcomes: readonly PickOutcome[],
  averageQueryMillis: number,
): PickSizeResult {
  const catalog = catalogOf(size);
  const catalogIds = new Set(catalog.map((operation) => operation.operationId));
  const axisResults = [
    ...AXES.map((axis) => axisResultOf(axis, outcomes, catalogIds)),
    axisResultOf("overall", outcomes, catalogIds),
  ];

  return { size, operationCount: catalog.length, axisResults, averageQueryMillis };
}

/** Every variant's `PickConfigurationResult`, scored from `shortlists` in one picker pass shared across all of them. */
export async function scorePickRows(
  shortlists: readonly VariantShortlists[],
  fetchImpl: FetchLike | undefined,
  baseUrl: string | undefined,
): Promise<readonly PickConfigurationResult[]> {
  const guardState = newPickGuardState();
  const results: PickConfigurationResult[] = [];

  for (const variant of shortlists) {
    const sizes: PickSizeResult[] = [];
    const systemPrompt = variant.showExamples
      ? PICK_SYSTEM_PROMPT_WITH_EXAMPLES
      : PICK_SYSTEM_PROMPT;

    for (const [size, rows] of variant.bySize) {
      const { outcomes, averageQueryMillis } = await pickAllFor(
        rows,
        guardState,
        fetchImpl,
        baseUrl,
        systemPrompt,
      );

      sizes.push(sizeResultOf(size, outcomes, averageQueryMillis));
    }

    results.push({ configId: variant.configId, sizes });
  }

  return results;
}
