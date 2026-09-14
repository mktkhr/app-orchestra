/**
 * A vector narrower (docs/plans/retrieving.md Task 2, spec section 6): the
 * same `Narrower` shape `measure.ts` wraps `lexical.ts`'s index in
 * (`lexical.ts`'s `Narrower`), implemented as a dot product against cached
 * catalogue vectors instead of bigram overlap.
 *
 * No vector store is built - spec section 7 says a thousand dot products
 * *is* the measurement, and a store that indexes them solves a problem
 * nobody has yet demonstrated. Vectors are normalised to unit length by
 * `client.ts`, so a dot product is exactly a cosine similarity.
 */
import type { CatalogueVectors } from "./cache.ts";
import { embedQuestion, type EmbeddingVector, type FetchLike } from "./client.ts";
import type { EmbeddingConfig } from "./configs.ts";
import type { Narrower, NarrowResult } from "../lexical.ts";

function dot(a: EmbeddingVector, b: EmbeddingVector): number {
  let sum = 0;

  for (let index = 0; index < a.length && index < b.length; index += 1) {
    sum += (a[index] ?? 0) * (b[index] ?? 0);
  }

  return sum;
}

/**
 * Every operation in `vectors`, ranked best first by cosine similarity to
 * `queryVector`, capped at `k`. Fewer than `k` results when `vectors` holds
 * fewer than `k` entries - a catalogue smaller than the shortlist asked for
 * has nothing more to offer, the same rule `lexical.ts`'s `narrow` follows.
 */
export function narrow(
  vectors: CatalogueVectors,
  queryVector: EmbeddingVector,
  k: number,
): readonly NarrowResult[] {
  const scored = Array.from(vectors, ([operationId, vector]) => ({
    operationId,
    score: dot(vector, queryVector),
  }));

  scored.sort((a, b) => b.score - a.score);

  return scored.slice(0, k);
}

/**
 * A `Narrower` over `vectors`: embeds the question at question time - never
 * cached (spec V4, AC-V-103) - and ranks the whole catalogue by cosine
 * similarity, so `measure.ts`'s shared `rankRangeIn` can find any operation's
 * rank rather than only the ones inside some fixed shortlist size.
 */
export function vectorNarrowerOf(
  vectors: CatalogueVectors,
  config: EmbeddingConfig,
  fetchImpl?: FetchLike,
  baseUrl?: string,
): Narrower {
  return {
    rank: async (question) => {
      const queryVector = await embedQuestion(config, question, fetchImpl, baseUrl);

      return narrow(vectors, queryVector, vectors.size);
    },
  };
}
