/**
 * The utterance narrower (spec G3, section 4; docs/plans/describing.md
 * Task 2 Step 2): an operation's score against a question is the **max**
 * over its own catalogue vector and every one of its utterance vectors, not
 * their sum or mean - five paraphrases folded into an average dilute a
 * strong match, five vectors beside the operation do not (spec G3).
 *
 * Built the same shape `embedding/narrower.ts` is: a plain ranking function
 * over already-embedded vectors, plus a `Narrower` that embeds the question
 * at question time and ranks the whole catalogue - never a fixed shortlist,
 * so `measure.ts`'s shared `rankRangeIn` can find any operation's rank.
 */
import type { CatalogueVectors } from "../embedding/cache.ts";
import { embedQuestion, type EmbeddingVector, type FetchLike } from "../embedding/client.ts";
import type { EmbeddingConfig } from "../embedding/configs.ts";
import type { UtteranceVectors } from "../embedding/utterance-cache.ts";
import type { Narrower, NarrowResult } from "../lexical.ts";

function dot(a: EmbeddingVector, b: EmbeddingVector): number {
  let sum = 0;

  for (let index = 0; index < a.length && index < b.length; index += 1) {
    sum += (a[index] ?? 0) * (b[index] ?? 0);
  }

  return sum;
}

/**
 * `operationVector`'s cosine similarity to `queryVector`, or that same
 * similarity for whichever of `utteranceVectors` scores highest, whichever
 * is larger (spec G3). Adding more utterances can only raise this, never
 * lower it - it is a max over a growing set.
 */
function bestScore(
  operationVector: EmbeddingVector,
  utteranceVectors: readonly EmbeddingVector[],
  queryVector: EmbeddingVector,
): number {
  let best = dot(operationVector, queryVector);

  for (const utteranceVector of utteranceVectors) {
    const score = dot(utteranceVector, queryVector);

    if (score > best) best = score;
  }

  return best;
}

/**
 * Every operation in `vectors`, ranked best first by `bestScore` against
 * `queryVector`, capped at `k`. An operation with no entry in `utterances`
 * is scored on its own vector alone - the same as `embedding/narrower.ts`'s
 * plain `narrow`.
 */
export function narrow(
  vectors: CatalogueVectors,
  utterances: UtteranceVectors,
  queryVector: EmbeddingVector,
  k: number,
): readonly NarrowResult[] {
  const scored = Array.from(vectors, ([operationId, vector]) => ({
    operationId,
    score: bestScore(vector, utterances.get(operationId) ?? [], queryVector),
  }));

  scored.sort((a, b) => b.score - a.score);

  return scored.slice(0, k);
}

/**
 * A `Narrower` over `vectors` and `utterances`: embeds the question at
 * question time (never cached, spec V4) and ranks the whole catalogue by
 * `bestScore`.
 */
export function utteranceNarrowerOf(
  vectors: CatalogueVectors,
  utterances: UtteranceVectors,
  config: EmbeddingConfig,
  fetchImpl?: FetchLike,
  baseUrl?: string,
): Narrower {
  return {
    rank: async (question) => {
      const queryVector = await embedQuestion(config, question, fetchImpl, baseUrl);

      return narrow(vectors, utterances, queryVector, vectors.size);
    },
  };
}
