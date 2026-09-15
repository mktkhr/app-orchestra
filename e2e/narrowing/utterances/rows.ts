/**
 * The five new rows (spec section 7; docs/plans/describing.md Task 2 Step
 * 4, and the coordinator's follow-up): `e5-large-q8+generated`, `+written`,
 * `+both`, and the two reranked rows `+reranker+both` and
 * `+reranker+written` — the second is what the product would actually use
 * (axis D 80% on the written layer alone versus 67% once the generated
 * layer's noise is mixed in, D-review after the first `make narrowing`).
 * Kept out of `measure.ts` so that file stays under the line budget;
 * `gatherReport` there calls the functions below exactly as it calls its
 * own `measureEmbeddingConfiguration`/`measureTwoStageConfiguration`.
 *
 * A sixth row, `+reranker(w)+written` (TODO.md item 1), lives in its own
 * sibling file, `reranker-written-row.ts`, rather than here — this file's
 * import count is already at the harness's per-file dependency cap, and
 * that row's `measureTwoStageWithWrittenRerankerConfiguration` needs one
 * more import (`reranker-written.ts`) than fits under it.
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

/** The two-stage configuration's id when retrieval is scored with both utterance layers. */
export const TWO_STAGE_BOTH_CONFIG_ID = "e5-large-q8+reranker+both";

/** The two-stage configuration's id when retrieval is scored with the written layer alone — the row the product would actually use (the coordinator's follow-up: written alone beats "both" on axis D because the generated layer drags it down). */
export const TWO_STAGE_WRITTEN_CONFIG_ID = "e5-large-q8+reranker+written";

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
 * A reranked configuration's full report (spec section 7): retrieval
 * scored with `utterances` (`setName` picks its cache: "both" or
 * "written"), then reranked exactly as the plain two-stage row reranks a
 * plain retrieval — the reranker still reads only `combinedTextOf` (spec
 * section 4). Reuses whatever `measureUtteranceConfiguration` already
 * cached for the same `setName`, so this call embeds nothing new when that
 * row ran first. `includeContractCheck` attaches the same layer's
 * retrieval/novelty rate (AC-G-103) to this row too, without recomputing
 * the catalogue embedding — used for `+reranker+written` so the row the
 * product would use carries its own contract rates, not left implicit in
 * the plain `+written` row above it.
 */
export async function measureTwoStageWithUtterancesConfiguration(
  configId: string,
  setName: string,
  utterances: OperationUtterances,
  includeContractCheck: boolean,
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
  const utteranceContractCheck = includeContractCheck
    ? await checkUtteranceContract(config, fullCatalog, utterances, transportOptions)
    : undefined;
  const contractCheckField = utteranceContractCheck === undefined ? {} : { utteranceContractCheck };
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

  return { configId, ...contractCheckField, sizes };
}
