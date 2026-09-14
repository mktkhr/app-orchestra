/**
 * Small pieces `measure.ts`'s `gatherReport` and `utterances/rows.ts` both
 * need, pulled out so neither file has to import the other: which
 * configuration the two-stage narrowers retrieve with, catalogue-size
 * slicing for both kinds of cached vectors, and the transport options every
 * `embed*`/`check*` call in this subproject takes.
 */
import {
  EMBEDDING_CONFIGS,
  type CatalogueVectors,
  type EmbeddingConfig,
} from "./embedding/index.ts";
import { embedMany, type EmbeddingVector, type FetchLike } from "./embedding/client.ts";
import type { UtteranceVectors } from "./embedding/utterance-cache.ts";
import type { FixtureOperation } from "./fixture/index.ts";
import type { GatherOptions } from "./report-types.ts";
import type { Question } from "./corpus/index.ts";

export type { ConfigurationResult, GatherOptions, SkippedConfiguration } from "./report-types.ts";

/** The embedding configuration id the two-stage narrowers retrieve with — `rerank/narrower.ts` says why `e5-large-q8`. */
export const TWO_STAGE_RETRIEVER_ID = "e5-large-q8";

/** `EMBEDDING_CONFIGS`'s own `e5-large-q8` entry — the two-stage narrowers reuse its prefix and pooling rather than redeclaring them. */
export function retrieverConfig(): EmbeddingConfig {
  const found = EMBEDDING_CONFIGS.find((config) => config.id === TWO_STAGE_RETRIEVER_ID);

  if (found === undefined) {
    throw new Error(
      `gather-helpers.ts: no embedding configuration named ${TWO_STAGE_RETRIEVER_ID}`,
    );
  }

  return found;
}

/** `fullVectors`, restricted to the operations `catalog` actually holds — the smaller catalogue sizes are prefixes of the full one, so this slices the cache rather than re-embedding. */
export function sliceVectors(
  fullVectors: CatalogueVectors,
  catalog: readonly FixtureOperation[],
): CatalogueVectors {
  const catalogIds = new Set(catalog.map((operation) => operation.operationId));

  return new Map(Array.from(fullVectors).filter(([operationId]) => catalogIds.has(operationId)));
}

/** `sliceVectors`, for a catalogue's utterance vectors instead of its own. */
export function sliceUtteranceVectors(
  fullVectors: UtteranceVectors,
  catalog: readonly FixtureOperation[],
): UtteranceVectors {
  const catalogIds = new Set(catalog.map((operation) => operation.operationId));

  return new Map(Array.from(fullVectors).filter(([operationId]) => catalogIds.has(operationId)));
}

/** `options`, as the subset of optional fields `embedCatalogue`/`checkContract`/`embedUtteranceVectors` accept — built by spreading only the ones actually set, because `exactOptionalPropertyTypes` (tsconfig.base.json) treats an explicit `undefined` differently from an absent key. */
export function transportOptionsOf(options: GatherOptions): {
  readonly dir?: string;
  readonly fetchImpl?: FetchLike;
  readonly baseUrl?: string;
} {
  return {
    ...(options.vectorsDir === undefined ? {} : { dir: options.vectorsDir }),
    ...(options.fetchImpl === undefined ? {} : { fetchImpl: options.fetchImpl }),
    ...(options.baseUrl === undefined ? {} : { baseUrl: options.baseUrl }),
  };
}

/** `options`, as the subset `generateUtterances` accepts — `dir` names its own default (`.utterances/`), never `vectorsDir`. */
export function utteranceCacheOptionsOf(options: GatherOptions): {
  readonly dir?: string;
  readonly fetchImpl?: FetchLike;
  readonly baseUrl?: string;
} {
  return {
    ...(options.utterancesDir === undefined ? {} : { dir: options.utterancesDir }),
    ...(options.fetchImpl === undefined ? {} : { fetchImpl: options.fetchImpl }),
    ...(options.baseUrl === undefined ? {} : { baseUrl: options.baseUrl }),
  };
}

export function unreachableReason(error: unknown): string {
  const message = error instanceof Error ? error.message : String(error);

  return `llama-swap was unreachable — ${message}`;
}

/** Every test question's vector, keyed by its own text — computed once, in one batch, before any rerank call (docs/plans/retrieving.md Task 3, "things that will bite"). */
export async function embedQuestionsOnce(
  config: EmbeddingConfig,
  testQuestions: readonly Question[],
  transportOptions: { readonly fetchImpl?: FetchLike; readonly baseUrl?: string },
): Promise<ReadonlyMap<string, EmbeddingVector>> {
  const texts = testQuestions.map((question) => question.text);
  const vectors = await embedMany(
    config,
    texts,
    "query",
    transportOptions.fetchImpl,
    transportOptions.baseUrl,
  );

  return new Map(texts.map((text, index) => [text, vectors[index] ?? []]));
}
