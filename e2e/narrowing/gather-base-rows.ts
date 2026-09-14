/**
 * The base rows `gather-phases.ts` builds on: one plain embedding
 * configuration's full report, and the plain two-stage (retrieve-then-
 * rerank) configuration's. Pulled into their own file only for eslint's
 * `max-lines`, once the utterance-scored reranked rows pushed
 * `gather-phases.ts` over budget.
 */
import {
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
  embedQuestionsOnce,
  retrieverConfig,
  sliceVectors,
  transportOptionsOf,
  type ConfigurationResult,
  type GatherOptions,
} from "./gather-helpers.ts";

/** One embedding configuration's full report: its catalogue vectors cached, its contract checked, and its recall measured at every size. Throws when the transport cannot be reached at all — `gather-phases.ts` turns that into a skip. */
export async function measureEmbeddingConfiguration(
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
export async function measureTwoStageConfiguration(
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
