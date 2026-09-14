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
 * The top `k` operation ids for `question`, scored against `index`.
 *
 * Holds no state between calls: everything it reads comes from `index` or
 * from its arguments, it allocates a fresh result array every call, and it
 * never writes to `index`.
 */
export function narrow(index: LexicalIndex, question: string, k: number): readonly NarrowResult[] {
  const queryBigrams = bigramsOf(question);
  const results: NarrowResult[] = [];

  for (const { operation, bigrams } of index.values()) {
    const score =
      queryBigrams.size === 0 ? 0 : overlapCount(queryBigrams, bigrams) / queryBigrams.size;

    results.push({ operationId: operation.operationId, score });
  }

  results.sort((a, b) => b.score - a.score);

  return results.slice(0, k);
}
