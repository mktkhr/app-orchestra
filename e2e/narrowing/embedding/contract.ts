/**
 * The contract check (docs/specs/retrieving.md section 4, AC-V-101). A
 * configuration's pooling and prefixes are declared, not proven - spec V2 -
 * and a wrong one produces numbers that look like a bad model: measured,
 * Ruri v3 scored 6/25 on axis B with CLS pooling and 22/25 with mean
 * pooling, same model, same corpus. So before any configuration's ranking
 * numbers count as evidence, it has to pass the cheapest demonstration that
 * does not beg the question: embed an operation's own text as a query, and
 * the same operation must come back first among the whole catalogue.
 *
 * The sample is 20 operations, every 50th across the catalogue, chosen this
 * way rather than at random - a check that differs between runs cannot be
 * cited in `DECISIONS.md`. Every text embedded here - catalogue documents
 * and sample queries alike - goes through the same `embedMany` the rest of
 * this subproject uses, over a fake transport in tests, so `make check`
 * still calls no model of any kind (AC-V-106).
 */
import { combinedTextOf } from "../lexical.ts";
import type { FixtureOperation } from "../fixture/index.ts";
import { embedMany, type FetchLike } from "./client.ts";
import type { EmbeddingConfig } from "./configs.ts";

/** How many operations `checkContract` samples. */
export const CONTRACT_SAMPLE_SIZE = 20;

/**
 * Up to `sampleSize` operations spread evenly across `catalog`, in
 * catalogue order, starting at index 0 and stepping by
 * `floor(catalog.length / sampleSize)` - every 50th across a 1000-operation
 * catalogue. Deterministic: the same catalogue always yields the same
 * sample.
 */
export function sampleOperations(
  catalog: readonly FixtureOperation[],
  sampleSize: number = CONTRACT_SAMPLE_SIZE,
): readonly FixtureOperation[] {
  if (catalog.length === 0 || sampleSize <= 0) return [];

  const step = Math.max(1, Math.floor(catalog.length / sampleSize));
  const sample: FixtureOperation[] = [];

  for (let index = 0; index < catalog.length && sample.length < sampleSize; index += step) {
    const operation = catalog[index];

    if (operation !== undefined) sample.push(operation);
  }

  return sample;
}

function dot(a: readonly number[], b: readonly number[]): number {
  let sum = 0;

  for (let index = 0; index < a.length && index < b.length; index += 1) {
    sum += (a[index] ?? 0) * (b[index] ?? 0);
  }

  return sum;
}

/**
 * The operation id whose catalogue vector best matches `queryVector` -
 * vectors are normalised (`client.ts`), so a dot product is a cosine
 * similarity.
 */
function topOperationId(
  catalog: readonly FixtureOperation[],
  catalogueVectors: readonly (readonly number[])[],
  queryVector: readonly number[],
): string {
  let bestId = "";
  let bestScore = -Infinity;

  catalog.forEach((operation, index) => {
    const score = dot(catalogueVectors[index] ?? [], queryVector);

    if (score > bestScore) {
      bestScore = score;
      bestId = operation.operationId;
    }
  });

  return bestId;
}

/** The outcome of AC-V-101 for one configuration: which sampled operations could not retrieve themselves. */
export interface ContractCheckResult {
  readonly configId: string;
  readonly passed: boolean;
  readonly sampledOperationIds: readonly string[];
  readonly failedOperationIds: readonly string[];
}

/**
 * Embeds `catalog` as documents and a deterministic sample of it as
 * queries (`sampleOperations`), then checks that each sampled operation's
 * own text retrieves itself first. Reaches the transport twice - once for
 * the whole catalogue, once for the sample of queries, both batched by
 * `embedMany` - never through `cache.ts`, so a stale cached vector can
 * never make a contract look sound.
 */
export async function checkContract(
  config: EmbeddingConfig,
  catalog: readonly FixtureOperation[],
  fetchImpl?: FetchLike,
  baseUrl?: string,
): Promise<ContractCheckResult> {
  const documentTexts = catalog.map((operation) => combinedTextOf(operation));
  const catalogueVectors = await embedMany(config, documentTexts, "document", fetchImpl, baseUrl);

  const sample = sampleOperations(catalog);
  const sampleTexts = sample.map((operation) => combinedTextOf(operation));
  const queryVectors = await embedMany(config, sampleTexts, "query", fetchImpl, baseUrl);

  const failedOperationIds = sample
    .filter((operation, index) => {
      const queryVector = queryVectors[index] ?? [];

      return topOperationId(catalog, catalogueVectors, queryVector) !== operation.operationId;
    })
    .map((operation) => operation.operationId);

  return {
    configId: config.id,
    passed: failedOperationIds.length === 0,
    sampledOperationIds: sample.map((operation) => operation.operationId),
    failedOperationIds,
  };
}
