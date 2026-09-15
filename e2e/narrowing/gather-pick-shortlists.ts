/**
 * Gathers every shortlist the pick rows score from (`gather-pick.ts`), for
 * every catalogue size, before any picker call is made — the picker's own
 * scoring pass is `gather-pick-scoring.ts`. Split out on its own only
 * because `gather-pick.ts` orchestrates both passes and stays over budget
 * otherwise.
 *
 * Each variant reuses `twoStageNarrowerOf` / `twoStageWithUtterancesNarrowerOf`
 * / `twoStageWithWrittenRerankerNarrowerOf` exactly as the corresponding
 * recall row builds them (`gather-base-rows.ts`, `utterances/rows.ts`) - the
 * same reranked order, cut to the top `PICK_SHORTLIST_K`. Nothing here
 * re-embeds the catalogue or its utterance layers: `gather-pick.ts` passes
 * in vectors already read from the on-disk cache the recall rows warmed. The
 * reranker call itself is not cached anywhere in this codebase, so it does
 * run again here, once per (variant, size, question) - the same cost every
 * existing row already pays once for its own recall figures.
 *
 * TODO.md item 1 adds two variants: one whose shortlist is built with
 * `twoStageWithWrittenRerankerNarrowerOf` (`useWrittenReranker`) rather than
 * the plain reranker, and both of them set `showExamples`, so
 * `candidatesFrom` attaches each candidate's own written examples for the
 * picker to read (`pick/client.ts`'s `e.g.` column) - the pick-side half of
 * the same finding: a candidate that entered the shortlist through its
 * written example looks, to the picker, like a plain summary mismatch,
 * because the thing that matched is invisible to it.
 */
import { twoStageNarrowerOf } from "./rerank/index.ts";
import {
  twoStageWithUtterancesNarrowerOf,
  twoStageWithWrittenRerankerNarrowerOf,
} from "./utterances/index.ts";
import { catalogOf, type FixtureOperation } from "./fixture/index.ts";
import { CATALOG_SIZES, type CatalogSize } from "./recall.ts";
import type {
  CatalogueVectors,
  EmbeddingVector,
  FetchLike,
  UtteranceVectors,
} from "./embedding/index.ts";
import type { Narrower } from "./lexical.ts";
import type { Question } from "./corpus/index.ts";
import { sliceUtteranceVectors, sliceVectors } from "./gather-helpers.ts";
import type { PickCandidate } from "./pick/index.ts";

/**
 * How many of the reranked shortlist's candidates the picker sees - the K
 * DECISIONS.md 2026-09-15 ("The local picker reads the shortlist in
 * reranker order: 78 → 83, for nothing") measured as the best of 10/20/50 on
 * the hand-run corpus: barely different from 50 on accuracy, but well ahead
 * of both on ambiguity and cost. Not the same constant as `rerank/
 * narrower.ts`'s `RETRIEVE_COUNT` (50), which is how many candidates the
 * reranker itself scores - this is how many of those the picker is shown.
 */
export const PICK_SHORTLIST_K = 20;

/**
 * One variant's inputs: its report id, the utterance layer its retrieval
 * is scored with (`undefined` for the plain two-stage row),
 * `useWrittenReranker` to rerank on the written examples rather than
 * `combinedTextOf` alone (only meaningful with written `utterances` -
 * TODO.md item 1), and `showExamples` to attach those same examples to the
 * candidates the picker is shown.
 */
export interface PickVariant {
  readonly configId: string;
  readonly utterances: UtteranceVectors | undefined;
  readonly useWrittenReranker?: boolean;
  readonly showExamples?: boolean;
}

/** One question's shortlist for one (variant, catalogue size) pair. */
export interface ShortlistedQuestion {
  readonly question: Question;
  readonly candidates: readonly PickCandidate[];
}

/**
 * One variant's shortlists at every catalogue size, plus whether its
 * candidates carry the `e.g.` column - `gather-pick-scoring.ts` reads
 * `showExamples` to choose the picker's system prompt
 * (`PICK_SYSTEM_PROMPT_WITH_EXAMPLES` vs. the untouched
 * `PICK_SYSTEM_PROMPT` - TODO.md item 1's byte-identity note in
 * `pick/client.ts`).
 */
export interface VariantShortlists {
  readonly configId: string;
  readonly showExamples: boolean;
  readonly bySize: ReadonlyMap<CatalogSize, readonly ShortlistedQuestion[]>;
}

function operationsById(
  catalog: readonly FixtureOperation[],
): ReadonlyMap<string, FixtureOperation> {
  return new Map(catalog.map((operation) => [operation.operationId, operation]));
}

/**
 * The top `PICK_SHORTLIST_K` of a reranked result, as `PickCandidate`s — an
 * id `operations` has no entry for is dropped rather than sent with blank
 * fields. `showExamples` attaches the operation's own written examples as
 * `examples` (TODO.md item 1); omitted (not sent as `[]`) when `false` or
 * when the operation carries none, so a variant that does not ask for the
 * column produces the exact candidate shape it always has.
 */
function candidatesFrom(
  ranked: readonly { readonly operationId: string }[],
  operations: ReadonlyMap<string, FixtureOperation>,
  showExamples: boolean,
): readonly PickCandidate[] {
  return ranked.slice(0, PICK_SHORTLIST_K).flatMap((result) => {
    const operation = operations.get(result.operationId);

    return operation === undefined
      ? []
      : [
          {
            operationId: operation.operationId,
            serviceDisplayName: operation.serviceDisplayName,
            summary: operation.summary,
            ...(showExamples && operation.examples.length > 0
              ? { examples: operation.examples }
              : {}),
          },
        ];
  });
}

/** The written-layer narrower a variant uses for retrieval: reranked on the written examples (`useWrittenReranker`) or on `combinedTextOf` alone, both scored the same way at retrieval. */
function writtenNarrowerFor(
  useWrittenReranker: boolean,
  vectors: CatalogueVectors,
  utterances: UtteranceVectors,
  catalog: readonly FixtureOperation[],
  questionVectors: ReadonlyMap<string, EmbeddingVector>,
  fetchImpl: FetchLike | undefined,
  baseUrl: string | undefined,
): Narrower {
  return useWrittenReranker
    ? twoStageWithWrittenRerankerNarrowerOf(
        vectors,
        utterances,
        catalog,
        questionVectors,
        fetchImpl,
        baseUrl,
      )
    : twoStageWithUtterancesNarrowerOf(
        vectors,
        utterances,
        catalog,
        questionVectors,
        fetchImpl,
        baseUrl,
      );
}

function narrowerFor(
  variant: PickVariant,
  vectors: CatalogueVectors,
  catalog: readonly FixtureOperation[],
  questionVectors: ReadonlyMap<string, EmbeddingVector>,
  fetchImpl: FetchLike | undefined,
  baseUrl: string | undefined,
): Narrower {
  return variant.utterances === undefined
    ? twoStageNarrowerOf(vectors, catalog, questionVectors, fetchImpl, baseUrl)
    : writtenNarrowerFor(
        variant.useWrittenReranker === true,
        vectors,
        sliceUtteranceVectors(variant.utterances, catalog),
        catalog,
        questionVectors,
        fetchImpl,
        baseUrl,
      );
}

/** One variant's shortlists at every catalogue size, in `CATALOG_SIZES` order. */
async function shortlistsFor(
  variant: PickVariant,
  fullVectors: CatalogueVectors,
  questionVectors: ReadonlyMap<string, EmbeddingVector>,
  testQuestions: readonly Question[],
  fetchImpl: FetchLike | undefined,
  baseUrl: string | undefined,
): Promise<ReadonlyMap<CatalogSize, readonly ShortlistedQuestion[]>> {
  const bySize = new Map<CatalogSize, readonly ShortlistedQuestion[]>();
  const showExamples = variant.showExamples === true;

  for (const size of CATALOG_SIZES) {
    const catalog = catalogOf(size);
    const operations = operationsById(catalog);
    const narrower = narrowerFor(
      variant,
      sliceVectors(fullVectors, catalog),
      catalog,
      questionVectors,
      fetchImpl,
      baseUrl,
    );
    const rows: ShortlistedQuestion[] = [];

    for (const question of testQuestions) {
      const ranked = await narrower.rank(question.text);

      rows.push({ question, candidates: candidatesFrom(ranked, operations, showExamples) });
    }

    bySize.set(size, rows);
  }

  return bySize;
}

/** Every variant's shortlists, at every catalogue size - the whole reranking pass, before any picker call. */
export async function gatherAllShortlists(
  variants: readonly PickVariant[],
  fullVectors: CatalogueVectors,
  questionVectors: ReadonlyMap<string, EmbeddingVector>,
  testQuestions: readonly Question[],
  fetchImpl: FetchLike | undefined,
  baseUrl: string | undefined,
): Promise<readonly VariantShortlists[]> {
  const results: VariantShortlists[] = [];

  for (const variant of variants) {
    const bySize = await shortlistsFor(
      variant,
      fullVectors,
      questionVectors,
      testQuestions,
      fetchImpl,
      baseUrl,
    );

    results.push({
      configId: variant.configId,
      showExamples: variant.showExamples === true,
      bySize,
    });
  }

  return results;
}
