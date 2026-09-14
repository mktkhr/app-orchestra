/**
 * The lexical baseline (docs/specs/narrowing.md section 7): character-bigram
 * overlap over each operation's summary, description, display name and
 * service display name, normalised by the number of bigrams in the query.
 *
 * No tokeniser, no stop-word list, no synonym table, no per-field boost, no
 * IDF weighting — this is the floor the rest of the subproject measures
 * against (spec section 7, plan Task 3). The one departure from raw overlap
 * is that a text's bigrams are deduplicated before counting: whether a
 * bigram occurs once or five times in a summary should not change whether it
 * counts as "present", because overlap here means coverage of the query's
 * vocabulary, not term frequency. That is a structural definition of what
 * "bigram overlap" means, decided before any question was scored against it
 * — not a boost added because it helped on an example.
 */
import type { FixtureOperation } from "./fixture/index.ts";

/** Every distinct two-code-point substring of `text`, deduplicated. */
export function bigramsOf(text: string): ReadonlySet<string> {
  const chars = Array.from(text);
  const bigrams = new Set<string>();

  for (let i = 0; i < chars.length - 1; i += 1) {
    bigrams.add(chars.slice(i, i + 2).join(""));
  }

  return bigrams;
}

/**
 * The text a lexical mechanism reads for one operation: its summary,
 * description, display name and service display name (spec section 7),
 * joined so `bigramsOf` can be taken over all four fields at once. Exported
 * so Task 4 can assert an axis-D question shares no bigram with the text
 * that is supposed to answer it, without reaching into this module's
 * internals.
 */
export function combinedTextOf(operation: FixtureOperation): string {
  return [
    operation.summary,
    operation.description,
    operation.displayName,
    operation.serviceDisplayName,
  ].join("\n");
}

/** How many members of `needle` also appear in `haystack`. */
function overlapCount(needle: ReadonlySet<string>, haystack: ReadonlySet<string>): number {
  let count = 0;

  for (const bigram of needle) {
    if (haystack.has(bigram)) count += 1;
  }

  return count;
}

/**
 * One operation's score against a question: the share of the question's
 * bigrams that also occur somewhere in the operation's text. Independent of
 * any index, so Task 4 can use it directly to assert an axis-E decoy
 * out-scores its answer without building a catalogue-wide index for a single
 * comparison. `narrow` computes the same quantity per catalogue entry, using
 * the index's precomputed bigram sets instead of recomputing them.
 */
export function scoreOperation(operation: FixtureOperation, question: string): number {
  const queryBigrams = bigramsOf(question);

  if (queryBigrams.size === 0) return 0;

  return overlapCount(queryBigrams, bigramsOf(combinedTextOf(operation))) / queryBigrams.size;
}

interface IndexedOperation {
  readonly operation: FixtureOperation;
  readonly bigrams: ReadonlySet<string>;
}

/**
 * The catalogue's bigram sets, computed once when the catalogue is read
 * (spec section 7: "the index is built when the catalogue is read and is a
 * map"). Keyed by `operationId`. `narrow` only ever reads from it.
 */
export type LexicalIndex = ReadonlyMap<string, IndexedOperation>;

/** Builds the index once. Holds nothing beyond what `catalog` already says. */
export function buildIndex(catalog: readonly FixtureOperation[]): LexicalIndex {
  const index = new Map<string, IndexedOperation>();

  for (const operation of catalog) {
    index.set(operation.operationId, {
      operation,
      bigrams: bigramsOf(combinedTextOf(operation)),
    });
  }

  return index;
}

export interface NarrowResult {
  readonly operationId: string;
  readonly score: number;
}

/**
 * The seam a narrowing mechanism exposes to `measure.ts`
 * (docs/plans/retrieving.md Task 2 Step 1): given a question, the whole
 * catalogue ranked best-first with scores — enough for `rankRangeIn` to
 * compute a worst and a best rank, whichever kind of narrower produced it.
 * `measure.ts` wraps this file's index-based scoring in one; `embedding/
 * narrower.ts` implements the same shape over cached vectors instead of
 * bigram counts.
 */
export interface Narrower {
  readonly rank: (question: string) => Promise<readonly NarrowResult[]>;
}

/**
 * Every operation with a non-zero score, best first. Operations the question
 * shares no bigram with are **not** in the result at all.
 *
 * That exclusion is a definition, not a filter added to flatter the numbers.
 * Measured on the fixture, 「休みたい」 scores 0 against all 1000 operations -
 * it is an axis-D question and the baseline cannot answer it (spec section
 * 7). Had zero-scored operations stayed in, a top-K cut would still have
 * returned K of them, chosen by whatever order the catalogue happened to be
 * in, and axis-D recall would have measured the fixture's ordering rather
 * than the mechanism. A shortlist is evidence; padding it is not.
 *
 * Holds no state between calls: everything it reads comes from `index` or
 * from its arguments, it allocates a fresh array every call, and it never
 * writes to `index`.
 */
function scoredCandidates(index: LexicalIndex, question: string): readonly NarrowResult[] {
  const queryBigrams = bigramsOf(question);
  const results: NarrowResult[] = [];

  if (queryBigrams.size === 0) return results;

  for (const { operation, bigrams } of index.values()) {
    const score = overlapCount(queryBigrams, bigrams) / queryBigrams.size;

    if (score > 0) results.push({ operationId: operation.operationId, score });
  }

  results.sort((a, b) => b.score - a.score);

  return results;
}

/**
 * The top `k` operation ids for `question`. Fewer than `k` when fewer than
 * `k` operations share any bigram with it - including none at all.
 */
export function narrow(index: LexicalIndex, question: string, k: number): readonly NarrowResult[] {
  return scoredCandidates(index, question).slice(0, k);
}

/**
 * Every candidate for `question`, ranked best first - the same list `narrow`
 * slices and `rankRangeOf` below searches, exposed so measure.ts's `Narrower`
 * seam (docs/plans/retrieving.md Task 2 Step 1) can call it once per
 * question and derive both a shortlist and every answer's rank range from
 * one result, rather than recomputing the score for each answer separately.
 */
export function candidatesFor(index: LexicalIndex, question: string): readonly NarrowResult[] {
  return scoredCandidates(index, question);
}

/**
 * Where `operationId` sits inside an already-ranked `candidates` list, as a
 * range, because scores tie in bulk: measured on the fixture, 186 operations
 * share the score at 「注文を一覧」's tenth place. A single rank would be
 * decided by whatever order the list happens to be in, so this reports both
 * ends.
 *
 * `best` counts only the candidates that score strictly higher; `worst` also
 * counts every other candidate with the same score. A measurement should be
 * read on `worst` - a mechanism that cannot separate an answer from 185
 * other operations has not found it - and the gap between the two is how
 * much of the result is a coin toss. Undefined when `operationId` is not in
 * `candidates` at all - not a candidate, whatever the reason.
 *
 * Shared by both kinds of narrower: `rankRangeOf` below is this function
 * applied to `scoredCandidates(index, question)`, and `embedding/
 * narrower.ts` applies it to a vector narrower's ranking instead.
 */
export function rankRangeIn(
  candidates: readonly NarrowResult[],
  operationId: string,
): { readonly best: number; readonly worst: number } | undefined {
  const found = candidates.find((c) => c.operationId === operationId);

  if (found === undefined) return undefined;

  const better = candidates.filter((c) => c.score > found.score).length;
  const equal = candidates.filter((c) => c.score === found.score).length;

  return { best: better + 1, worst: better + equal };
}

/**
 * Where `operationId` sits in the ranking for `question` against `index` -
 * `rankRangeIn` applied to this module's own `scoredCandidates`. Kept as its
 * own export because `lexical.test.ts` and the lexical row's own numbers
 * depend on this exact call shape; behaviour is unchanged by the refactor
 * that added `rankRangeIn` underneath it.
 */
export function rankRangeOf(
  index: LexicalIndex,
  question: string,
  operationId: string,
): { readonly best: number; readonly worst: number } | undefined {
  return rankRangeIn(scoredCandidates(index, question), operationId);
}
