/**
 * The headline configuration (spec section 7): `e5-large-q8+reranker+both`
 * - retrieval scored with both utterance layers (`narrower.ts`'s `narrow`,
 * the max over an operation's own vector and every utterance's), then
 * reranked exactly as `rerank/narrower.ts`'s `twoStageNarrowerOf` reranks a
 * plain retrieval: the reranker still reads only `combinedTextOf` for each
 * retrieved operation (spec section 4) - utterances get an operation into
 * the fifty, nothing more.
 *
 * This does not live in `rerank/narrower.ts` because that module's
 * `twoStageNarrowerOf` is unchanged by this subproject (spec G1: "Retrieval,
 * reranking and the picker do not change") - this is a second, sibling
 * narrower that composes the same reranker with a different retrieval
 * stage, not a modification of the first one.
 */
import { combinedTextOf } from "../lexical.ts";
import type { FixtureOperation } from "../fixture/index.ts";
import type { CatalogueVectors } from "../embedding/cache.ts";
import type { EmbeddingVector, FetchLike } from "../embedding/client.ts";
import type { UtteranceVectors } from "../embedding/utterance-cache.ts";
import type { Narrower } from "../lexical.ts";
import { RETRIEVE_COUNT } from "../rerank/narrower.ts";
import { rerank, type RerankCandidate } from "../rerank/client.ts";
import { narrow } from "./narrower.ts";
import type { NarrowResult } from "../lexical.ts";

function operationsById(
  catalog: readonly FixtureOperation[],
): ReadonlyMap<string, FixtureOperation> {
  return new Map(catalog.map((operation) => [operation.operationId, operation]));
}

/** `retrieved` as reranker candidates: `combinedTextOf` for each, exactly as `rerank/narrower.ts`'s own `candidatesFor` does (spec section 4). */
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
 * A `Narrower` over `vectors` and `utterances`: retrieves `RETRIEVE_COUNT`
 * candidates by `narrower.ts`'s utterance-scored ranking, then reranks
 * exactly those candidates on their own `combinedTextOf` and returns them
 * in reranked order - the same shape and the same alternation discipline as
 * `rerank/narrower.ts`'s `twoStageNarrowerOf` (`questionVectors` must
 * already hold every question's vector, computed once before any `rank`
 * call).
 */
export function twoStageWithUtterancesNarrowerOf(
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
          `twoStageWithUtterancesNarrowerOf: no prefetched vector for question "${question}"`,
        );
      }

      const retrieved = narrow(vectors, utterances, queryVector, RETRIEVE_COUNT);
      const candidates = candidatesFor(retrieved, operations);
      const reranked = await rerank(question, candidates, fetchImpl, baseUrl);

      return reranked.map((result) => ({ operationId: result.id, score: result.score }));
    },
  };
}
