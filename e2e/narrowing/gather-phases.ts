/**
 * `gatherReport`'s phases (`measure.ts`), pulled out only for eslint's
 * `max-lines`/`max-lines-per-function`: generation, the base embedding
 * configurations, the four utterance rows, and the two reranked rows - one
 * function per phase, each appending to the same `configurations`/`skipped`
 * arrays and threading "is the transport still reachable" through to the
 * next. `measure.ts`'s `gatherReport` is a plain sequence of calls into
 * this file.
 */
import {
  EMBEDDING_CONFIGS,
  checkContract,
  embedCatalogue,
  vectorNarrowerOf,
  type EmbeddingConfig,
} from "./embedding/index.ts";
import { twoStageNarrowerOf } from "./rerank/index.ts";
import { catalogOf } from "./fixture/index.ts";
import { measureAcrossSizes, type K } from "./recall.ts";
import type { Question } from "./corpus/index.ts";
import {
  TWO_STAGE_BOTH_CONFIG_ID,
  generateUtterances,
  measureTwoStageWithUtterancesConfiguration,
  measureUtteranceConfiguration,
  mergeUtterances,
  writtenUtterancesOf,
  type OperationUtterances,
} from "./utterances/index.ts";
import {
  embedQuestionsOnce,
  retrieverConfig,
  sliceVectors,
  transportOptionsOf,
  unreachableReason,
  utteranceCacheOptionsOf,
  type ConfigurationResult,
  type GatherOptions,
  type SkippedConfiguration,
} from "./gather-helpers.ts";

/** One embedding configuration's full report: its catalogue vectors cached, its contract checked, and its recall measured at every size. Throws when the transport cannot be reached at all — the phase functions below turn that into a skip. */
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

/** The two-stage configuration's id in the report — one block, not two (spec V6). */
export const TWO_STAGE_CONFIG_ID = "e5-large-q8+reranker";

/**
 * The two-stage configuration's full report (spec V6, AC-V-104): `e5-large-
 * q8`'s catalogue vectors (already cached from its own row above this one,
 * so this call is a disk read, not a network one), its contract check, and
 * every test question's vector — all embedded before any rerank call, so the
 * whole run alternates between the retriever and the reranker exactly once
 * rather than once per question.
 */
async function measureTwoStageConfiguration(
  testQuestions: readonly Question[],
  kValues: readonly K[],
  options: GatherOptions,
): Promise<ConfigurationResult> {
  const config = retrieverConfig();
  const fullCatalog = catalogOf(5);
  const transportOptions = transportOptionsOf(options);
  const fullVectors = await embedCatalogue(config, fullCatalog, transportOptions);
  const contractCheck = await checkContract(
    config,
    fullCatalog,
    transportOptions.fetchImpl,
    transportOptions.baseUrl,
  );
  const questionVectors = await embedQuestionsOnce(config, testQuestions, transportOptions);

  const sizes = await measureAcrossSizes(
    (catalog) =>
      twoStageNarrowerOf(
        sliceVectors(fullVectors, catalog),
        catalog,
        questionVectors,
        transportOptions.fetchImpl,
        transportOptions.baseUrl,
      ),
    testQuestions,
    kValues,
  );

  return { configId: TWO_STAGE_CONFIG_ID, contractCheck, sizes };
}

/** One row of the utterance sweep: which layer, and its cache's set name (`embedUtteranceVectors`). */
interface UtteranceRow {
  readonly configId: string;
  readonly setName: string;
  readonly utterances: OperationUtterances;
}

/** What every later phase needs: whether the transport is still reachable, and the three utterance layers (spec section 3, section 7). */
export interface GenerationPhaseResult {
  readonly reachable: boolean;
  readonly generatedUtterances: OperationUtterances;
  readonly writtenUtterances: OperationUtterances;
  readonly bothUtterances: OperationUtterances;
}

/**
 * Generation, alone and first (spec section 5): a thousand cached (or
 * freshly generated) chat calls before any embedding call, so a run never
 * alternates between the chat model and the embedder (docs/plans/
 * describing.md, "Details that decide"). A warm cache — every `make
 * narrowing` after the first — reaches the transport zero times here
 * (AC-G-101). The written layer needs no call at all; it is already on
 * every `FixtureOperation`.
 */
export async function runGenerationPhase(
  options: GatherOptions,
  skipped: SkippedConfiguration[],
): Promise<GenerationPhaseResult> {
  const fullCatalog = catalogOf(5);
  const writtenUtterances = writtenUtterancesOf(fullCatalog);

  try {
    const generatedUtterances = await generateUtterances(
      fullCatalog,
      utteranceCacheOptionsOf(options),
    );

    return {
      reachable: true,
      generatedUtterances,
      writtenUtterances,
      bothUtterances: mergeUtterances(generatedUtterances, writtenUtterances),
    };
  } catch (error) {
    skipped.push({ configId: "utterances", reason: unreachableReason(error) });

    return {
      reachable: false,
      generatedUtterances: new Map(),
      writtenUtterances,
      bothUtterances: writtenUtterances,
    };
  }
}

/** Every embedding configuration in `EMBEDDING_CONFIGS` that could be reached, appended to `configurations`; the rest recorded in `skipped` as not retried once the transport is found unreachable. Returns whether it still is. */
export async function runEmbeddingConfigurations(
  testQuestions: readonly Question[],
  kValues: readonly K[],
  options: GatherOptions,
  configurations: ConfigurationResult[],
  skipped: SkippedConfiguration[],
  reachable: boolean,
): Promise<boolean> {
  let stillReachable = reachable;

  for (const config of EMBEDDING_CONFIGS) {
    if (!stillReachable) {
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
      stillReachable = false;
      skipped.push({ configId: config.id, reason: unreachableReason(error) });
    }
  }

  return stillReachable;
}

/** The three utterance rows (spec section 7), appended to `configurations` when reachable, else recorded as not retried. Returns whether it still is. */
export async function runUtteranceRows(
  phase: GenerationPhaseResult,
  testQuestions: readonly Question[],
  kValues: readonly K[],
  options: GatherOptions,
  configurations: ConfigurationResult[],
  skipped: SkippedConfiguration[],
  reachable: boolean,
): Promise<boolean> {
  let stillReachable = reachable;
  const rows: readonly UtteranceRow[] = [
    {
      configId: "e5-large-q8+generated",
      setName: "generated",
      utterances: phase.generatedUtterances,
    },
    { configId: "e5-large-q8+written", setName: "written", utterances: phase.writtenUtterances },
    { configId: "e5-large-q8+both", setName: "both", utterances: phase.bothUtterances },
  ];

  for (const row of rows) {
    if (!stillReachable) {
      skipped.push({
        configId: row.configId,
        reason: "llama-swap was unreachable for an earlier configuration; not retried",
      });
      continue;
    }

    try {
      configurations.push(
        await measureUtteranceConfiguration(
          row.configId,
          row.setName,
          row.utterances,
          testQuestions,
          kValues,
          options,
        ),
      );
    } catch (error) {
      stillReachable = false;
      skipped.push({ configId: row.configId, reason: unreachableReason(error) });
    }
  }

  return stillReachable;
}

/** The two reranked rows — the plain two-stage and the one scored with both utterance layers (spec section 7, the headline). */
export async function runRerankedRows(
  bothUtterances: OperationUtterances,
  testQuestions: readonly Question[],
  kValues: readonly K[],
  options: GatherOptions,
  configurations: ConfigurationResult[],
  skipped: SkippedConfiguration[],
  reachable: boolean,
): Promise<void> {
  let stillReachable = reachable;

  if (stillReachable) {
    try {
      configurations.push(await measureTwoStageConfiguration(testQuestions, kValues, options));
    } catch (error) {
      stillReachable = false;
      skipped.push({ configId: TWO_STAGE_CONFIG_ID, reason: unreachableReason(error) });
    }
  } else {
    skipped.push({
      configId: TWO_STAGE_CONFIG_ID,
      reason: "llama-swap was unreachable for an earlier configuration; not retried",
    });
  }

  if (stillReachable) {
    try {
      configurations.push(
        await measureTwoStageWithUtterancesConfiguration(
          bothUtterances,
          testQuestions,
          kValues,
          options,
        ),
      );
    } catch (error) {
      skipped.push({ configId: TWO_STAGE_BOTH_CONFIG_ID, reason: unreachableReason(error) });
    }
  } else {
    skipped.push({
      configId: TWO_STAGE_BOTH_CONFIG_ID,
      reason: "llama-swap was unreachable for an earlier configuration; not retried",
    });
  }
}
