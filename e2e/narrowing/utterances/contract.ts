/**
 * The utterance contract check (docs/specs/describing.md section 6,
 * AC-G-103): each utterance embedded as a query must retrieve its own
 * operation first among the whole catalogue - the same demonstration
 * `embedding/contract.ts` applies to an operation's own text, applied here
 * to what the generator wrote for it. A generated layer whose own
 * utterances mostly cannot find their operation is not evidence about
 * anything (spec section 6), so this is not a boolean: it returns a rate
 * and the failing (utterance, operation, what-won) triples.
 *
 * The sample is the same deterministic every-50th selection
 * `embedding/contract.ts`'s `sampleOperations` already makes, reused here
 * rather than reimplemented, so the utterance rate and the text rate are
 * always taken over the same operations. The catalogue side is ranked
 * against `embedding/cache.ts`'s cached document vectors - already on disk
 * under `.vectors/` once `make narrowing` has embedded the catalogue - so
 * this check pays for embedding the sampled utterances as queries and
 * nothing else. A question's vector is never cached (spec V4); neither is
 * an utterance's, here - this module measures what Task 2's narrower will
 * do, it does not build the cache Task 2 uses.
 */
import { embedCatalogue, type CacheOptions, type CatalogueVectors } from "../embedding/cache.ts";
import { embedMany } from "../embedding/client.ts";
import type { EmbeddingConfig } from "../embedding/configs.ts";
import { sampleOperations } from "../embedding/contract.ts";
import type { FixtureOperation } from "../fixture/index.ts";
import type { OperationUtterances } from "./cache.ts";

/** One utterance that did not retrieve its own operation first. */
export interface UtteranceContractFailure {
  readonly utterance: string;
  readonly operationId: string;
  readonly wonBy: string;
}

/** The outcome of AC-G-103 for one configuration's generated utterances. */
export interface UtteranceContractResult {
  readonly configId: string;
  readonly sampledOperationIds: readonly string[];
  readonly totalUtterances: number;
  /** The share of sampled utterances that retrieved their own operation first. `0` when there were none to check. */
  readonly rate: number;
  readonly failures: readonly UtteranceContractFailure[];
}

interface FlatUtterance {
  readonly operationId: string;
  readonly utterance: string;
}

/** Every utterance belonging to a sampled operation, flattened in sample order. */
function flatten(
  sample: readonly FixtureOperation[],
  utterances: OperationUtterances,
): readonly FlatUtterance[] {
  const flat: FlatUtterance[] = [];

  for (const operation of sample) {
    for (const utterance of utterances.get(operation.operationId) ?? []) {
      flat.push({ operationId: operation.operationId, utterance });
    }
  }

  return flat;
}

function dot(a: readonly number[], b: readonly number[]): number {
  let sum = 0;

  for (let index = 0; index < a.length && index < b.length; index += 1) {
    sum += (a[index] ?? 0) * (b[index] ?? 0);
  }

  return sum;
}

/** The operation id whose catalogue vector best matches `queryVector`. */
function topOperationId(
  catalog: readonly FixtureOperation[],
  vectors: CatalogueVectors,
  queryVector: readonly number[],
): string {
  let bestId = "";
  let bestScore = -Infinity;

  for (const operation of catalog) {
    const score = dot(vectors.get(operation.operationId) ?? [], queryVector);

    if (score > bestScore) {
      bestScore = score;
      bestId = operation.operationId;
    }
  }

  return bestId;
}

/**
 * Embeds a deterministic sample of `catalog`'s operations' utterances as
 * queries and ranks each against `catalog`'s cached document vectors
 * (`embedCatalogue`), reporting the rate at which an utterance's own
 * operation comes back first. `options` is the same shape `embedCatalogue`
 * takes - a test supplies a throwaway `dir` and a fake transport, exactly
 * as `embedding/cache.test.ts` does.
 */
export async function checkUtteranceContract(
  config: EmbeddingConfig,
  catalog: readonly FixtureOperation[],
  utterances: OperationUtterances,
  options: CacheOptions = {},
): Promise<UtteranceContractResult> {
  const sample = sampleOperations(catalog);
  const sampledOperationIds = sample.map((operation) => operation.operationId);
  const flat = flatten(sample, utterances);

  if (flat.length === 0) {
    return { configId: config.id, sampledOperationIds, totalUtterances: 0, rate: 0, failures: [] };
  }

  const catalogueVectors = await embedCatalogue(config, catalog, options);
  const queryVectors = await embedMany(
    config,
    flat.map((item) => item.utterance),
    "query",
    options.fetchImpl,
    options.baseUrl,
  );

  const failures = flat
    .map((item, index) => ({
      item,
      winner: topOperationId(catalog, catalogueVectors, queryVectors[index] ?? []),
    }))
    .filter(({ item, winner }) => winner !== item.operationId)
    .map(({ item, winner }) => ({
      utterance: item.utterance,
      operationId: item.operationId,
      wonBy: winner,
    }));

  return {
    configId: config.id,
    sampledOperationIds,
    totalUtterances: flat.length,
    rate: (flat.length - failures.length) / flat.length,
    failures,
  };
}
