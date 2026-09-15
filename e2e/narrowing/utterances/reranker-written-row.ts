/**
 * The reranker-reads-written configuration's full report (TODO.md item 1;
 * `reranker-written.ts`): retrieval scored with the written layer, exactly
 * as `rows.ts`'s `measureTwoStageWithUtterancesConfiguration`'s own
 * `"written"` row builds it, then reranked on `combinedTextOf` plus the
 * operation's own written examples rather than `combinedTextOf` alone.
 * Reuses whatever the plain `+written` row already cached for `setName`
 * `"written"`, so this call embeds nothing new when that row ran first.
 * Always carries its own contract rates, the same as `+reranker+written`
 * does — the row the product would compare this one against.
 *
 * Kept out of `rows.ts` on its own file: that module's own import count is
 * already at the harness's per-file dependency cap (`import/max-
 * dependencies`), and this row's own narrower (`reranker-written.ts`) would
 * push it over.
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
import { twoStageWithWrittenRerankerNarrowerOf } from "./reranker-written.ts";

export async function measureTwoStageWithWrittenRerankerConfiguration(
  configId: string,
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
    "written",
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
  const questionVectors = await embedQuestionsOnce(config, testQuestions, transportOptions);

  const sizes: readonly SizeResult[] = await measureAcrossSizes(
    (catalog) =>
      twoStageWithWrittenRerankerNarrowerOf(
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

  return { configId, utteranceContractCheck, sizes };
}
