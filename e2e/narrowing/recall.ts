/**
 * The recall computation itself (docs/specs/narrowing.md section 6;
 * docs/specs/retrieving.md section 6) — split out of `measure.ts` only for
 * eslint's `max-lines` (300): once `measure.ts` also carried the embedding
 * configurations' orchestration (`gatherReport`, docs/plans/retrieving.md
 * Task 2), the two concerns together no longer fit one file. `measure.ts`
 * re-exports everything below, so nothing outside these two files needs to
 * know the split happened.
 *
 * `measure()` knows nothing about *how* a ranking was produced — only that
 * a `Narrower` (`lexical.ts`) can turn a question into one, ranked
 * best-first with scores. `lexicalNarrowerOf` is the one narrower this file
 * knows how to build (over `lexical.ts`'s bigram index); `embedding/
 * narrower.ts` builds the other kind, over cached vectors, and `measure.ts`
 * is what hands either one to `measure()`.
 *
 * **A tie is not a hit.** Recall is computed on `rankRangeIn`'s `worst`: an
 * answer counts as recalled at K only when it is still inside K even when
 * every operation tied with it is ranked ahead of it. The optimistic figure
 * (`best`) is computed alongside — an answer counts there if some ordering
 * of the tie could put it first — and `report.ts` flags a line where the two
 * differ by more than a little: that gap is how much of the result is a coin
 * toss (spec section 6). Embedding vectors are floats and essentially never
 * tie, so the two columns should collapse onto each other for every
 * embedding configuration; both are still computed and printed for every
 * configuration regardless (docs/specs/retrieving.md section 6) — a column
 * that appears only for some rows cannot be compared across rows.
 *
 * **A question is excluded, not scored zero, when none of its answers are in
 * the catalogue being measured.** The corpus (`corpus/index.ts`) was written
 * against the full five-service fixture; at catalogue sizes 1-3 some
 * questions' answers are not there at all — a service the question's answer
 * lives in simply was not included. That is not the mechanism failing to
 * find something; it is the question not being askable of that catalogue,
 * so it is left out of every recall figure at that size and the excluded
 * count is reported instead of being silently averaged away.
 */
import { catalogOf, type FixtureOperation } from "./fixture/index.ts";
import {
  buildIndex,
  candidatesFor,
  rankRangeIn,
  type Narrower,
  type NarrowResult,
} from "./lexical.ts";
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
 * `question`'s rank against an already-computed `candidates` ranking: the
 * closest of its answers' best and worst ranks (spec section 6, "a question
 * has several answers; it is recalled if any of them is"). Ineligible when
 * none of its answers are in this catalogue at all — `best`/`worst` stay
 * undefined and it never reaches the per-K comparison.
 */
function rankOf(
  catalogIds: ReadonlySet<string>,
  candidates: readonly NarrowResult[],
  question: Question,
): QuestionRank {
  if (!isEligible(question, catalogIds)) {
    return { axis: question.axis, eligible: false, best: undefined, worst: undefined };
  }

  const ranges = question.answers.map((answerId) => rankRangeIn(candidates, answerId));

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

/** A `Narrower` over `catalog`'s bigram index — the lexical floor. */
export function lexicalNarrowerOf(catalog: readonly FixtureOperation[]): Narrower {
  const index = buildIndex(catalog);

  return { rank: (question) => Promise.resolve(candidatesFor(index, question)) };
}

/**
 * Measures `testQuestions` against `catalog` (spec section 6) using
 * `narrower` — the lexical index by default, so every existing caller keeps
 * working unchanged. For each question, `narrower.rank` is called exactly
 * once: the call is timed (the cost a real caller pays, since a narrowing
 * mechanism is asked once per question), and every answer's rank range is
 * read from that one ranked result rather than asking the narrower again.
 */
export async function measure(
  catalog: readonly FixtureOperation[],
  testQuestions: readonly Question[],
  kValues: readonly K[] = K_VALUES,
  narrower: Narrower = lexicalNarrowerOf(catalog),
): Promise<MeasureResult> {
  const catalogIds = new Set(catalog.map((operation) => operation.operationId));
  const queryMillis: number[] = [];
  const ranks: QuestionRank[] = [];

  for (const question of testQuestions) {
    const start = performance.now();
    const candidates = await narrower.rank(question.text);

    queryMillis.push(performance.now() - start);
    ranks.push(rankOf(catalogIds, candidates, question));
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

/** `narrowerFor(catalogOf(size))` measured at every catalogue size, against the same questions and K values. */
export async function measureAcrossSizes(
  narrowerFor: (catalog: readonly FixtureOperation[]) => Narrower,
  testQuestions: readonly Question[],
  kValues: readonly K[],
): Promise<readonly SizeResult[]> {
  const sizes: SizeResult[] = [];

  for (const size of CATALOG_SIZES) {
    const catalog = catalogOf(size);
    const result = await measure(catalog, testQuestions, kValues, narrowerFor(catalog));

    sizes.push({ size, result });
  }

  return sizes;
}

/** Measures every catalogue size against the full corpus, lexical mechanism only — `measure.ts`'s `gatherReport` is what adds the embedding configurations beside this. */
export function measureAll(
  testQuestions: readonly Question[] = questions(),
  kValues: readonly K[] = K_VALUES,
): Promise<readonly SizeResult[]> {
  return measureAcrossSizes(lexicalNarrowerOf, testQuestions, kValues);
}
