/**
 * The reranker-reads-written variant (TODO.md item 1; DECISIONS.md
 * 2026-09-15, "The catalogue says it" (d): `+written` alone reads axis D
 * 80% at K=10, but `+reranker+written` reads only 73% - the cross-encoder
 * rescores on `combinedTextOf` alone (`docs/specs/describing.md` section
 * 4), so an operation retrieved into the fifty by its written example is
 * pushed back down by a reranker that never read the example that put it
 * there.
 *
 * This narrower keeps retrieval exactly as `two-stage.ts`'s
 * `twoStageWithUtterancesNarrowerOf` does it for the written layer (the
 * same `narrow` scoring, the same `RETRIEVE_COUNT`), and changes only what
 * the reranker is handed: `combinedTextOf(operation)` plus that
 * operation's own written examples, one per line, after the rest of the
 * combined text. Generated utterances stay out of this document on
 * purpose (the 2026-09-15 record above shows them as noise on retrieval;
 * feeding that same noise to the reranker's document text would not be a
 * different experiment, just a repeat of the same mistake one layer over)
 * - so, unlike `two-stage.ts`, this file has no "both" sibling.
 *
 * `two-stage.ts` itself is untouched: that module's own row must stay
 * byte-identical for comparison against this one (TODO.md item 1), so this
 * is a second, sibling narrower, not a modification of the first.
 */
import { combinedTextOf } from "../lexical.ts";
import type { FixtureOperation } from "../fixture/index.ts";
import type { CatalogueVectors } from "../embedding/cache.ts";
import type { EmbeddingVector, FetchLike } from "../embedding/client.ts";
import type { UtteranceVectors } from "../embedding/utterance-cache.ts";
import type { Narrower, NarrowResult } from "../lexical.ts";
import { RETRIEVE_COUNT } from "../rerank/narrower.ts";
import { rerank, type RerankCandidate } from "../rerank/client.ts";
import { narrow } from "./narrower.ts";

/** The two-stage-with-written-reranker configuration's report id (spec section 7's naming: `(w)` marks a reranker that reads the written layer). */
export const TWO_STAGE_WRITTEN_RERANKER_CONFIG_ID = "e5-large-q8+reranker(w)+written";

function operationsById(
  catalog: readonly FixtureOperation[],
): ReadonlyMap<string, FixtureOperation> {
  return new Map(catalog.map((operation) => [operation.operationId, operation]));
}

/** `combinedTextOf(operation)` plus its written examples, one per line - the document text this variant's reranker reads instead of `combinedTextOf` alone. */
function documentTextWithExamplesOf(operation: FixtureOperation): string {
  return [combinedTextOf(operation), ...operation.examples].join("\n");
}

/** `retrieved` as reranker candidates, on `documentTextWithExamplesOf` rather than `combinedTextOf` alone. An id `operations` has no text for is dropped, exactly as `two-stage.ts`'s own `candidatesFor` drops one. */
function candidatesWithExamplesFor(
  retrieved: readonly NarrowResult[],
  operations: ReadonlyMap<string, FixtureOperation>,
): readonly RerankCandidate[] {
  return retrieved.flatMap((result) => {
    const operation = operations.get(result.operationId);

    return operation === undefined
      ? []
      : [{ id: result.operationId, text: documentTextWithExamplesOf(operation) }];
  });
}

/**
 * A `Narrower` over `vectors` and the written `utterances`: retrieves
 * `RETRIEVE_COUNT` candidates by `narrower.ts`'s utterance-scored ranking
 * (identical to `two-stage.ts`'s own retrieval stage), then reranks those
 * candidates on `documentTextWithExamplesOf` and returns them in reranked
 * order. `questionVectors` must already hold every question's vector, the
 * same alternation discipline `two-stage.ts` and `rerank/narrower.ts`
 * follow.
 */
export function twoStageWithWrittenRerankerNarrowerOf(
  vectors: CatalogueVectors,
  utterances: UtteranceVectors,
  catalog: readonly FixtureOperation[],
  questionVectors: ReadonlyMap<string, EmbeddingVector>,
  fetchImpl?: FetchLike,
  baseUrl?: string,
): Narrower {
  const operations = operationsById(catalog);

  return {
    rank: async (question) => {
      const queryVector = questionVectors.get(question);

      if (queryVector === undefined) {
        throw new Error(
          `twoStageWithWrittenRerankerNarrowerOf: no prefetched vector for question "${question}"`,
        );
      }

      const retrieved = narrow(vectors, utterances, queryVector, RETRIEVE_COUNT);
      const candidates = candidatesWithExamplesFor(retrieved, operations);
      const reranked = await rerank(question, candidates, fetchImpl, baseUrl);

      return reranked.map((result) => ({ operationId: result.id, score: result.score }));
    },
  };
}
