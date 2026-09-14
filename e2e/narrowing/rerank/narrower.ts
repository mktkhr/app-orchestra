/**
 * The two-stage configuration (docs/specs/retrieving.md V6, section 9;
 * docs/plans/retrieving.md Task 3): retrieve `RETRIEVE_COUNT` candidates by
 * cosine similarity with `e5-large-q8` (`embedding/narrower.ts`'s `narrow`),
 * then rerank exactly those with `bge-reranker-v2-m3-q8` and keep the
 * reranked order. Measured and reported as **one** mechanism, not two
 * (spec V6): `measure.ts` hands this narrower's results to the same
 * `measure()`/`report.ts` path every other configuration goes through, so it
 * prints as one block.
 *
 * Why 50: measured on the full 1000-operation catalogue, `e5-large-q8` puts
 * the vocabulary-gap axis D answers inside the top 50 for 73% of those
 * questions and inside the top 10 for only 20% - every other configuration
 * measured in this subproject does worse at 50. The answers are found and
 * not ordered, which is exactly what a reranker is for.
 */
import { combinedTextOf } from "../lexical.ts";
import type { FixtureOperation } from "../fixture/index.ts";
import type { CatalogueVectors } from "../embedding/cache.ts";
import type { EmbeddingVector, FetchLike } from "../embedding/client.ts";
import { narrow } from "../embedding/narrower.ts";
import type { Narrower, NarrowResult } from "../lexical.ts";
import { rerank, type RerankCandidate } from "./client.ts";

/** How many candidates the retrieval stage hands to the reranker (see this file's header for why 50). */
export const RETRIEVE_COUNT = 50;

function operationsById(
  catalog: readonly FixtureOperation[],
): ReadonlyMap<string, FixtureOperation> {
  return new Map(catalog.map((operation) => [operation.operationId, operation]));
}

/**
 * `retrieved` as reranker candidates: `combinedTextOf` for each - the same
 * text the retriever's own vectors were built from (spec V3), so the
 * reranker sees what the retriever saw. An id `operations` has no text for
 * is dropped rather than sent as an empty document.
 */
function candidatesFor(
  retrieved: readonly NarrowResult[],
  operations: ReadonlyMap<string, FixtureOperation>,
): readonly RerankCandidate[] {
  return retrieved.flatMap((result) => {
    const operation = operations.get(result.operationId);

    return operation === undefined
      ? []
      : [{ id: result.operationId, text: combinedTextOf(operation) }];
  });
}

/**
 * A `Narrower` over `vectors`: retrieves `RETRIEVE_COUNT` candidates by
 * cosine similarity to `question`'s vector, then reranks exactly those
 * candidates and returns them in reranked order.
 *
 * `questionVectors` must already hold every question's vector, keyed by its
 * text, computed once before any `rank` call rather than per call - so a
 * whole run alternates between the retriever and the reranker exactly once,
 * not once per question (docs/plans/retrieving.md Task 3, "things that will
 * bite": a naive per-question alternation pays a model load - tens of
 * seconds, spec section 5 - on every question rather than once for the
 * whole run).
 */
export function twoStageNarrowerOf(
  vectors: CatalogueVectors,
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
        throw new Error(`twoStageNarrowerOf: no prefetched vector for question "${question}"`);
      }

      const retrieved = narrow(vectors, queryVector, RETRIEVE_COUNT);
      const candidates = candidatesFor(retrieved, operations);
      const reranked = await rerank(question, candidates, fetchImpl, baseUrl);

      return reranked.map((result) => ({ operationId: result.id, score: result.score }));
    },
  };
}
