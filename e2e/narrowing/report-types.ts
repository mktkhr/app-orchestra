/**
 * The shapes `measure.ts`'s `gatherReport` produces and `report.ts` prints,
 * pulled out on their own so `measure.ts` and `utterances/rows.ts` can both
 * build a `ConfigurationResult` without one importing the other (Task 2
 * adds four new rows built in `utterances/rows.ts`, which needs this same
 * shape). `measure.ts` re-exports everything here, so nothing outside these
 * two files needs to know the split happened.
 */
import type { ContractCheckResult } from "./embedding/index.ts";
import type { UtteranceContractResult } from "./utterances/index.ts";
import type { SizeResult } from "./recall.ts";
import type { FetchLike } from "./embedding/client.ts";

/** One configuration's full report: every catalogue size, and the contract check when it has one (embedding configurations only — the lexical floor has no contract to check). */
export interface ConfigurationResult {
  readonly configId: string;
  readonly contractCheck?: ContractCheckResult;
  /** The utterance contract check (spec section 6, AC-G-103) — set only for the generated, written and "both" rows, never for a plain embedding or two-stage row. */
  readonly utteranceContractCheck?: UtteranceContractResult;
  readonly sizes: readonly SizeResult[];
}

/** A configuration that could not be measured at all — llama-swap was unreachable — and why. */
export interface SkippedConfiguration {
  readonly configId: string;
  readonly reason: string;
}

/** How `gatherReport` reaches the transport and the vector cache — overridable so tests never reach llama-swap (AC-V-106). */
export interface GatherOptions {
  readonly fetchImpl?: FetchLike;
  readonly baseUrl?: string;
  readonly vectorsDir?: string;
  /** Where the generated layer's utterances are cached (spec section 5) — overridable only so a test never reads or writes the real `.utterances/` directory. */
  readonly utterancesDir?: string;
}
