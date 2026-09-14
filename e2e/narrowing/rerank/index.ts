/**
 * The rerank module's public surface (docs/plans/retrieving.md Task 3).
 * `measure.ts` reads this file, not the modules behind it directly.
 */
export { RERANK_MODEL, rerank, type RerankCandidate, type RerankResult } from "./client.ts";
export { RETRIEVE_COUNT, twoStageNarrowerOf } from "./narrower.ts";
