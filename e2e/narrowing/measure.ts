/**
 * The whole report `make narrowing` prints (docs/plans/narrowing.md Task 4;
 * docs/plans/retrieving.md Task 2). The recall computation itself is
 * `recall.ts` (split out only for eslint's `max-lines`; re-exported below so
 * nothing outside these two files needs to know that). What this file adds
 * is `gatherReport`: running the lexical floor, then every embedding
 * configuration in `embedding/configs.ts` that llama-swap can actually
 * serve, catching the point where it stops being reachable rather than
 * throwing (AC-V-104; docs/plans/retrieving.md Task 2 Step 5).
 *
 * Run directly — `node e2e/narrowing/measure.ts` (`make narrowing`) — to
 * print the full report. Every other export here is a plain function so
 * `measure.test.ts` can exercise it without paying for the whole corpus,
 * and without reaching llama-swap at all (AC-V-106): `gatherReport` takes a
 * `fetchImpl`/`baseUrl`/`vectorsDir` exactly so a test can hand it a fake
 * transport and a throwaway cache directory instead.
 */
import { pathToFileURL } from "node:url";

import {
  EMBEDDING_CONFIGS,
  checkContract,
  embedCatalogue,
  vectorNarrowerOf,
  type CatalogueVectors,
  type ContractCheckResult,
  type EmbeddingConfig,
} from "./embedding/index.ts";
import type { FetchLike } from "./embedding/client.ts";
import { catalogOf, type FixtureOperation } from "./fixture/index.ts";
import {
  K_VALUES,
  lexicalNarrowerOf,
  measureAcrossSizes,
  type K,
  type SizeResult,
} from "./recall.ts";
import { printReport } from "./report.ts";
import { questions, type Question } from "./corpus/index.ts";

export {
  CATALOG_SIZES,
  K_VALUES,
  measure,
  measureAll,
  type AxisRecall,
  type CatalogSize,
  type K,
  type MeasureResult,
  type RecallAtK,
  type SizeResult,
} from "./recall.ts";

/** One configuration's full report: every catalogue size, and the contract check when it has one (embedding configurations only — the lexical floor has no contract to check). */
export interface ConfigurationResult {
  readonly configId: string;
  readonly contractCheck?: ContractCheckResult;
  readonly sizes: readonly SizeResult[];
}

/** A configuration that could not be measured at all — llama-swap was unreachable — and why. */
export interface SkippedConfiguration {
  readonly configId: string;
  readonly reason: string;
}

/** `fullVectors`, restricted to the operations `catalog` actually holds — the smaller catalogue sizes are prefixes of the full one, so this slices the cache rather than re-embedding (docs/plans/retrieving.md Task 2). */
function sliceVectors(
  fullVectors: CatalogueVectors,
  catalog: readonly FixtureOperation[],
): CatalogueVectors {
  const catalogIds = new Set(catalog.map((operation) => operation.operationId));

  return new Map(Array.from(fullVectors).filter(([operationId]) => catalogIds.has(operationId)));
}

/** How `gatherReport` reaches the transport and the vector cache — overridable so tests never reach llama-swap (AC-V-106). */
export interface GatherOptions {
  readonly fetchImpl?: FetchLike;
  readonly baseUrl?: string;
  readonly vectorsDir?: string;
}

/** `options`, as the subset of optional fields `embedCatalogue`/`checkContract` accept — built by spreading only the ones actually set, because `exactOptionalPropertyTypes` (tsconfig.base.json) treats an explicit `undefined` differently from an absent key. */
function transportOptionsOf(options: GatherOptions): {
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

function unreachableReason(error: unknown): string {
  const message = error instanceof Error ? error.message : String(error);

  return `llama-swap was unreachable — ${message}`;
}

/** One embedding configuration's full report: its catalogue vectors cached, its contract checked, and its recall measured at every size. Throws when the transport cannot be reached at all — `gatherReport` is what turns that into a skip. */
async function measureEmbeddingConfiguration(
  config: EmbeddingConfig,
  testQuestions: readonly Question[],
  kValues: readonly K[],
  options: GatherOptions,
): Promise<ConfigurationResult> {
  const fullCatalog = catalogOf(5);
  const transportOptions = transportOptionsOf(options);
  const fullVectors = await embedCatalogue(config, fullCatalog, transportOptions);
  const contractCheck = await checkContract(
    config,
    fullCatalog,
    transportOptions.fetchImpl,
    transportOptions.baseUrl,
  );
  const sizes = await measureAcrossSizes(
    (catalog) =>
      vectorNarrowerOf(
        sliceVectors(fullVectors, catalog),
        config,
        transportOptions.fetchImpl,
        transportOptions.baseUrl,
      ),
    testQuestions,
    kValues,
  );

  return { configId: config.id, contractCheck, sizes };
}

/**
 * Every configuration `make narrowing` reports: the lexical floor, measured
 * unconditionally, plus every embedding configuration in `EMBEDDING_CONFIGS`
 * that could actually be reached. The first embedding configuration to fail
 * to embed its catalogue is read as llama-swap being down rather than that
 * one configuration being broken, and every configuration from that point on
 * is recorded as skipped instead of retried (docs/plans/retrieving.md Task 2
 * Step 5) — the lexical row still runs either way, and nothing here throws.
 */
export async function gatherReport(
  testQuestions: readonly Question[],
  kValues: readonly K[],
  options: GatherOptions = {},
): Promise<{
  readonly configurations: readonly ConfigurationResult[];
  readonly skipped: readonly SkippedConfiguration[];
}> {
  const lexicalSizes = await measureAcrossSizes(lexicalNarrowerOf, testQuestions, kValues);
  const configurations: ConfigurationResult[] = [{ configId: "lexical", sizes: lexicalSizes }];
  const skipped: SkippedConfiguration[] = [];
  let reachable = true;

  for (const config of EMBEDDING_CONFIGS) {
    if (!reachable) {
      skipped.push({
        configId: config.id,
        reason: "llama-swap was unreachable for an earlier configuration; not retried",
      });
      continue;
    }

    try {
      configurations.push(
        await measureEmbeddingConfiguration(config, testQuestions, kValues, options),
      );
    } catch (error) {
      reachable = false;
      skipped.push({ configId: config.id, reason: unreachableReason(error) });
    }
  }

  return { configurations, skipped };
}

async function main(): Promise<void> {
  const { configurations, skipped } = await gatherReport(questions(), K_VALUES);

  printReport(configurations, skipped, K_VALUES);
}

// Runs only when this file is the process's entry point (`node
// e2e/narrowing/measure.ts`, i.e. `make narrowing`) — not when
// `measure.test.ts` imports the functions above.
const entryArgument = process.argv[1];

if (entryArgument !== undefined && import.meta.url === pathToFileURL(entryArgument).href) {
  try {
    await main();
  } catch (error) {
    console.error(error);
    process.exitCode = 1;
  }
}
