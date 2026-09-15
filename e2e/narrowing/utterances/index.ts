/**
 * The utterances module's public surface (docs/plans/describing.md Task 1).
 * Task 2's narrower and `report.ts` read this file, not the modules behind
 * it directly - the same convention `embedding/index.ts` follows.
 */
export {
  UTTERANCE_MAX_TOKENS,
  UTTERANCE_MODEL,
  UTTERANCE_PROMPT,
  generateUtterancesFor,
} from "./generate.ts";
export {
  DEFAULT_UTTERANCES_DIR,
  generateUtterances,
  type OperationUtterances,
  type UtteranceCacheOptions,
} from "./cache.ts";
export {
  checkUtteranceContract,
  type NonNovelUtterance,
  type UtteranceContractFailure,
  type UtteranceContractResult,
} from "./contract.ts";
export { narrow, utteranceNarrowerOf } from "./narrower.ts";
export { twoStageWithUtterancesNarrowerOf } from "./two-stage.ts";
export {
  TWO_STAGE_WRITTEN_RERANKER_CONFIG_ID,
  twoStageWithWrittenRerankerNarrowerOf,
} from "./reranker-written.ts";
export { mergeUtterances, writtenUtterancesOf } from "./layers.ts";
export {
  TWO_STAGE_BOTH_CONFIG_ID,
  TWO_STAGE_WRITTEN_CONFIG_ID,
  measureTwoStageWithUtterancesConfiguration,
  measureUtteranceConfiguration,
} from "./rows.ts";
export { measureTwoStageWithWrittenRerankerConfiguration } from "./reranker-written-row.ts";
