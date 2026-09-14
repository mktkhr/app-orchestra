/**
 * The four new rows (spec section 7; docs/plans/describing.md Task 2 Step
 * 4): `e5-large-q8+generated`, `+written`, `+both`, and the two-stage
 * `+reranker+both`. Kept out of `measure.ts` so that file stays under the
 * line budget; `gatherReport` there calls the two functions below exactly
 * as it calls its own `measureEmbeddingConfiguration`/
 * `measureTwoStageConfiguration`.
 */
import { embedCatalogue, embedUtteranceVectors } from "../embedding/index.ts";
import { catalogOf } from "../fixture/index.ts";
import { measureAcrossSizes, type K, type SizeResult } from "../recall.ts";
import type { ConfigurationResult, GatherOptions } from "../report-types.ts";
import type { Question } from "../corpus/index.ts";
import {
  embedQuestionsOnce,
  retrieverConfig,
  sliceUtteranceVectors,
  sliceVectors,
  transportOptionsOf,
} from "../gather-helpers.ts";
import { checkUtteranceContract } from "./contract.ts";
import type { OperationUtterances } from "./cache.ts";
import { utteranceNarrowerOf } from "./narrower.ts";
import { twoStageWithUtterancesNarrowerOf } from "./two-stage.ts";

/** The two-stage configuration's id when retrieval is scored with both utterance layers (spec section 7 — the headline number). */
export const TWO_STAGE_BOTH_CONFIG_ID = "e5-large-q8+reranker+both";

/**
 * One utterance layer's full report (spec section 7): `e5-large-q8`'s
 * catalogue vectors (already cached by its own row, so this is a disk read),
 * `utterances` embedded as documents and cached under `setName`
 * (`embedUtteranceVectors`), the utterance contract check (AC-G-103), and
 * recall measured at every size with `utteranceNarrowerOf`'s max-over-own-
 * and-utterance-vectors scoring (spec G3).
 */
export async function measureUtteranceConfiguration(
  configId: string,
  setName: string,
  utterances: OperationUtterances,
  testQuestions: readonly Question[],
  kValues: readonly K[],
  options: GatherOptions,
): Promise<ConfigurationResult> {
  const config = retrieverConfig();
  const fullCatalog = catalogOf(5);
  const transportOptions = transportOptionsOf(options);
  const fullVectors = await embedCatalogue(config, fullCatalog, transportOptions);
  const fullUtteranceVectors = await embedUtteranceVectors(
    config,
    setName,
    fullCatalog,
    utterances,
    transportOptions,
  );
  const utteranceContractCheck = await checkUtteranceContract(
    config,
    fullCatalog,
    utterances,
    transportOptions,
  );
  const sizes = await measureAcrossSizes(
    (catalog) =>
      utteranceNarrowerOf(
        sliceVectors(fullVectors, catalog),
        sliceUtteranceVectors(fullUtteranceVectors, catalog),
        config,
        transportOptions.fetchImpl,
        transportOptions.baseUrl,
      ),
    testQuestions,
    kValues,
  );

  return { configId, utteranceContractCheck, sizes };
}

/**
 * The headline configuration's full report (spec section 7): retrieval
 * scored with the union of both utterance layers, then reranked exactly as
 * the plain two-stage row reranks a plain retrieval — the reranker still
 * reads only `combinedTextOf` (spec section 4). Reuses whatever
 * `measureUtteranceConfiguration("e5-large-q8+both", ...)` already cached,
 * so this call embeds nothing new when that row ran first.
 */
export async function measureTwoStageWithUtterancesConfiguration(
  bothUtterances: OperationUtterances,
  testQuestions: readonly Question[],
  kValues: readonly K[],
  options: GatherOptions,
): Promise<ConfigurationResult> {
  const config = retrieverConfig();
  const fullCatalog = catalogOf(5);
  const transportOptions = transportOptionsOf(options);
  const fullVectors = await embedCatalogue(config, fullCatalog, transportOptions);
  const fullUtteranceVectors = await embedUtteranceVectors(
    config,
    "both",
    fullCatalog,
    bothUtterances,
    transportOptions,
  );
  const questionVectors = await embedQuestionsOnce(config, testQuestions, transportOptions);

  const sizes: readonly SizeResult[] = await measureAcrossSizes(
    (catalog) =>
      twoStageWithUtterancesNarrowerOf(
        sliceVectors(fullVectors, catalog),
        sliceUtteranceVectors(fullUtteranceVectors, catalog),
        catalog,
        questionVectors,
        transportOptions.fetchImpl,
        transportOptions.baseUrl,
      ),
    testQuestions,
    kValues,
  );

  return { configId: TWO_STAGE_BOTH_CONFIG_ID, sizes };
}
