/**
 * The measurement (docs/specs/narrowing.md section 6; docs/plans/narrowing.md
 * Task 4, Steps 4-5). recall@K over the fixture catalogue and the corpus, and
 * nothing else: no LLM is involved (spec T2) — a narrowing mechanism (today,
 * `lexical.ts`) produces a shortlist, and this module only asks whether each
 * question's answer is in it.
 *
 * **A tie is not a hit.** Recall is computed on `rankRangeOf`'s `worst`: an
 * answer counts as recalled at K only when it is still inside K even when
 * every operation tied with it is ranked ahead of it. The optimistic figure
 * (`best`) is computed alongside — an answer counts there if some ordering
 * of the tie could put it first — and `report.ts` flags a line where the two
 * differ by more than a little: that gap is how much of the result is a coin
 * toss (spec section 6).
 *
 * **A question is excluded, not scored zero, when none of its answers are in
 * the catalogue being measured.** The corpus (`corpus/index.ts`) was written
 * against the full five-service fixture; at catalogue sizes 1-3 some
 * questions' answers are not there at all — a service the question's answer
 * lives in simply was not included. That is not the mechanism failing to
 * find something; it is the question not being askable of that catalogue,
 * so it is left out of every recall figure at that size and the excluded
 * count is reported instead of being silently averaged away.
 *
 * Run directly — `node e2e/narrowing/measure.ts` (`make narrowing`) — to
 * print the full report. Every other export here is a plain function so
 * `measure.test.ts` can exercise the recall computation without paying for
 * the whole corpus and all four catalogue sizes.
 */
import { pathToFileURL } from "node:url";

import { catalogOf, type FixtureOperation } from "./fixture/index.ts";
import { buildIndex, narrow, rankRangeOf, type LexicalIndex } from "./lexical.ts";
import { printReport } from "./report.ts";
import { questions, type Axis, type Question } from "./corpus/index.ts";

/** The four catalogue sizes the fixture defines (spec T6): 200/400/600/1000 operations. */
export const CATALOG_SIZES = [1, 2, 3, 5] as const;
export type CatalogSize = (typeof CATALOG_SIZES)[number];

/** The three K values the spec asks for (section 6): the report's default. */
export const K_VALUES = [10, 20, 50] as const;

/**
 * A shortlist size. Left as plain `number` rather than the `K_VALUES`
 * literal union so `measure.test.ts` can probe a tie at a K small enough to
 * work out by hand (K=1) without widening the report's own column set.
 */
export type K = number;

const AXES: readonly Axis[] = ["A", "B", "C", "D", "E"];

/** One question's outcome against one catalogue: whether it could be asked at all, and where its closest answer ranked. */
interface QuestionRank {
  readonly axis: Axis;
  readonly eligible: boolean;
  readonly best: number | undefined;
  readonly worst: number | undefined;
}

/** Whether at least one of `question`'s answers is an operation this catalogue actually serves. */
function isEligible(question: Question, catalogIds: ReadonlySet<string>): boolean {
  return question.answers.some((answerId) => catalogIds.has(answerId));
}

/** The smallest of a set of possibly-absent numbers, or undefined when none are present. */
function minDefined(values: readonly (number | undefined)[]): number | undefined {
  const present = values.filter((value): value is number => value !== undefined);

  return present.length === 0 ? undefined : Math.min(...present);
}

/**
 * `question`'s rank against `index`: the closest of its answers' best and
 * worst ranks (spec section 6, "a question has several answers; it is
 * recalled if any of them is"). Ineligible when none of its answers are in
 * this catalogue at all — `best`/`worst` stay undefined and it never
 * reaches the per-K comparison.
 */
function rankOf(
  index: LexicalIndex,
  catalogIds: ReadonlySet<string>,
  question: Question,
): QuestionRank {
  if (!isEligible(question, catalogIds)) {
    return { axis: question.axis, eligible: false, best: undefined, worst: undefined };
  }

  const ranges = question.answers.map((answerId) => rankRangeOf(index, question.text, answerId));

  return {
    axis: question.axis,
    eligible: true,
    best: minDefined(ranges.map((range) => range?.best)),
    worst: minDefined(ranges.map((range) => range?.worst)),
  };
}

/** Recall at one K, both ends of the range (spec section 6). */
export interface RecallAtK {
  readonly k: K;
  /** Hits on `worst` — an answer still inside K even behind every tie. */
  readonly pessimisticHits: number;
  /** Hits on `best` — an answer inside K given the most favourable tie order. */
  readonly optimisticHits: number;
}

/** Recall for one axis, or `"overall"` across all five, at every K. */
export interface AxisRecall {
  readonly axis: Axis | "overall";
  /** Eligible questions — the denominator every hit count above is read against. */
  readonly total: number;
  /** Questions left out because none of their answers are in this catalogue. */
  readonly excluded: number;
  readonly recalls: readonly RecallAtK[];
}

/** One axis's recall figures, built from the ranks of the questions that belong to it. */
function recallOf(
  axis: Axis | "overall",
  ranks: readonly QuestionRank[],
  kValues: readonly K[],
): AxisRecall {
  const eligible = ranks.filter((rank) => rank.eligible);
  const excluded = ranks.length - eligible.length;

  const recalls = kValues.map((k) => ({
    k,
    pessimisticHits: eligible.filter((rank) => rank.worst !== undefined && rank.worst <= k).length,
    optimisticHits: eligible.filter((rank) => rank.best !== undefined && rank.best <= k).length,
  }));

  return { axis, total: eligible.length, excluded, recalls };
}

/** The result of measuring one catalogue against one set of questions. */
export interface MeasureResult {
  readonly operationCount: number;
  /** One entry per axis (A-E), then one for `"overall"` — six in total. */
  readonly axisRecalls: readonly AxisRecall[];
  readonly averageQueryMillis: number;
}

/**
 * Measures `testQuestions` against `catalog` (spec section 6). Builds one
 * index, then for every question times a `narrow` call at the largest K
 * requested — the cost a real caller pays, since a narrowing mechanism is
 * asked once per question and the result is cut to size afterwards, not
 * asked once per K — and separately reads `rankRangeOf` for every answer to
 * decide whether it is recalled at each K.
 */
export function measure(
  catalog: readonly FixtureOperation[],
  testQuestions: readonly Question[],
  kValues: readonly K[] = K_VALUES,
): MeasureResult {
  const index = buildIndex(catalog);
  const catalogIds = new Set(catalog.map((operation) => operation.operationId));
  const widestK = Math.max(...kValues);
  const queryMillis: number[] = [];
  const ranks: QuestionRank[] = [];

  for (const question of testQuestions) {
    const start = performance.now();

    narrow(index, question.text, widestK);
    queryMillis.push(performance.now() - start);
    ranks.push(rankOf(index, catalogIds, question));
  }

  const axisRecalls = [
    ...AXES.map((axis) =>
      recallOf(
        axis,
        ranks.filter((rank) => rank.axis === axis),
        kValues,
      ),
    ),
    recallOf("overall", ranks, kValues),
  ];

  const totalMillis = queryMillis.reduce((sum, ms) => sum + ms, 0);

  return {
    operationCount: catalog.length,
    axisRecalls,
    averageQueryMillis: testQuestions.length === 0 ? 0 : totalMillis / testQuestions.length,
  };
}

/** One catalogue size's result, paired with the size that produced it. */
export interface SizeResult {
  readonly size: CatalogSize;
  readonly result: MeasureResult;
}

/** Measures every catalogue size against the full corpus (the report's input). */
export function measureAll(
  testQuestions: readonly Question[] = questions(),
  kValues: readonly K[] = K_VALUES,
): readonly SizeResult[] {
  return CATALOG_SIZES.map((size) => ({
    size,
    result: measure(catalogOf(size), testQuestions, kValues),
  }));
}

function main(): void {
  printReport(measureAll(), K_VALUES);
}

// Runs only when this file is the process's entry point (`node
// e2e/narrowing/measure.ts`, i.e. `make narrowing`) — not when
// `measure.test.ts` imports the functions above.
const entryArgument = process.argv[1];

if (entryArgument !== undefined && import.meta.url === pathToFileURL(entryArgument).href) {
  main();
}
