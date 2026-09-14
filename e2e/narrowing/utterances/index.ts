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
  type UtteranceContractFailure,
  type UtteranceContractResult,
} from "./contract.ts";
