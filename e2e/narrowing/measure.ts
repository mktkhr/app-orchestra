/**
 * The whole report `make narrowing` prints (docs/plans/narrowing.md Task 4;
 * docs/plans/retrieving.md Task 2; docs/plans/describing.md Task 2). The
 * recall computation is `recall.ts`, the result shapes are `report-types.ts`
 * (re-exported by `gather-helpers.ts`), and every phase `gatherReport` runs
 * is `gather-phases.ts` - all split out only for eslint's `max-lines`, and
 * re-exported below so nothing outside these files needs to know the split
 * happened. What this file adds is `gatherReport` itself: the lexical
 * floor, then generation, then the base embedding configurations, the four
 * utterance rows, then the two reranked rows - one call into
 * `gather-phases.ts` per phase, threading "is the transport still
 * reachable" from one call to the next (AC-V-104).
 *
 * Run directly — `node e2e/narrowing/measure.ts` (`make narrowing`) — to
 * print the full report. Every other export here is a plain function so
 * `measure.test.ts` can exercise it without paying for the whole corpus,
 * and without reaching llama-swap at all (AC-V-106): `gatherReport` takes a
 * `fetchImpl`/`baseUrl`/`vectorsDir`/`utterancesDir` exactly so a test can
 * hand it a fake transport and throwaway cache directories instead.
 */
import { pathToFileURL } from "node:url";

import { measureAlternation } from "./loading.ts";
import { K_VALUES, lexicalNarrowerOf, measureAcrossSizes, type K } from "./recall.ts";
import { printReport } from "./report.ts";
import { questions, type Question } from "./corpus/index.ts";
import {
  runEmbeddingConfigurations,
  runGenerationPhase,
  runRerankedRows,
  runUtteranceRows,
} from "./gather-phases.ts";
import { runPickRows } from "./gather-pick.ts";
import type { ConfigurationResult, GatherOptions, SkippedConfiguration } from "./gather-helpers.ts";
import type { PickConfigurationResult } from "./report-types.ts";

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
export type { ConfigurationResult, GatherOptions, SkippedConfiguration } from "./gather-helpers.ts";
export type { PickAxisResult, PickConfigurationResult, PickSizeResult } from "./report-types.ts";

/**
 * Every configuration `make narrowing` reports, phase by phase: the lexical
 * floor (unconditional), generation, the base embedding configurations, the
 * four utterance rows, then the two reranked rows (`gather-phases.ts`). The
 * first configuration to fail to reach the transport is read as llama-swap
 * being down rather than that one configuration being broken, and every
 * configuration from that point on is recorded as skipped instead of
 * retried (docs/plans/retrieving.md Task 2 Step 5) — the lexical row still
 * runs either way, and nothing here throws. The phase order is deliberate:
 * llama-swap holds one model at a time, and interleaving per question would
 * pay a model load on every question rather than once per phase
 * (docs/plans/describing.md, "Details that decide").
 */
export async function gatherReport(
  testQuestions: readonly Question[],
  kValues: readonly K[],
  options: GatherOptions = {},
): Promise<{
  readonly configurations: readonly ConfigurationResult[];
  readonly pickConfigurations: readonly PickConfigurationResult[];
  readonly skipped: readonly SkippedConfiguration[];
}> {
  const lexicalSizes = await measureAcrossSizes(lexicalNarrowerOf, testQuestions, kValues);
  const configurations: ConfigurationResult[] = [{ configId: "lexical", sizes: lexicalSizes }];
  const skipped: SkippedConfiguration[] = [];

  const phase = await runGenerationPhase(options, skipped);
  const reachableAfterEmbedding = await runEmbeddingConfigurations(
    testQuestions,
    kValues,
    options,
    configurations,
    skipped,
    phase.reachable,
  );
  const reachableAfterUtterances = await runUtteranceRows(
    phase,
    testQuestions,
    kValues,
    options,
    configurations,
    skipped,
    reachableAfterEmbedding,
  );

  const reachableAfterReranked = await runRerankedRows(
    phase,
    testQuestions,
    kValues,
    options,
    configurations,
    skipped,
    reachableAfterUtterances,
  );

  const pickConfigurations: PickConfigurationResult[] = [];

  await runPickRows(
    phase,
    testQuestions,
    options,
    pickConfigurations,
    skipped,
    reachableAfterReranked,
  );

  return { configurations, pickConfigurations, skipped };
}

async function main(): Promise<void> {
  const { configurations, pickConfigurations, skipped } = await gatherReport(questions(), K_VALUES);
  const alternation = await measureAlternation();

  printReport(configurations, pickConfigurations, skipped, K_VALUES, alternation);
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
