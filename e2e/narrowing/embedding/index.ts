/**
 * The embedding module's public surface (docs/plans/retrieving.md Task 1).
 * Task 2's narrower and the report read this file, not the modules behind
 * it directly.
 */
export { EMBEDDING_CONFIGS, type EmbeddingConfig, type Pooling } from "./configs.ts";
export {
  DEFAULT_BASE_URL,
  EMBEDDING_BATCH_SIZE,
  embedMany,
  embedQuestion,
  type EmbeddingKind,
  type EmbeddingVector,
  type FetchLike,
} from "./client.ts";
export {
  DEFAULT_VECTORS_DIR,
  embedCatalogue,
  type CacheOptions,
  type CatalogueVectors,
} from "./cache.ts";
export {
  CONTRACT_SAMPLE_SIZE,
  checkContract,
  sampleOperations,
  type ContractCheckResult,
} from "./contract.ts";
