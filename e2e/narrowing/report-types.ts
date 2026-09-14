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
import type { CatalogSize, SizeResult } from "./recall.ts";
import type { FetchLike } from "./embedding/client.ts";
import type { Axis } from "./corpus/index.ts";

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

/**
 * One axis's pick figures at one catalogue size, or `"overall"` across all
 * five (TODO.md "Measure the pick, not only the recall"): `correct` counts
 * a picked operationId among the question's `answers`; `flagged` counts the
 * picker returning `ambiguous` - both out of `total`, the same eligibility
 * rule the recall rows use (`recall.ts`'s `isEligible`). Unlike `AxisRecall`
 * there is no worst/best split (the picker's chat output is not a scored
 * ranking to take a tie range over) and no per-K sweep (the pick rows read
 * one shortlist size, `gather-pick-shortlists.ts`'s `PICK_SHORTLIST_K`).
 */
export interface PickAxisResult {
  readonly axis: Axis | "overall";
  readonly total: number;
  readonly excluded: number;
  readonly correct: number;
  readonly flagged: number;
}

/** One catalogue size's pick figures, plus the picker's own per-question wall-clock at that size. */
export interface PickSizeResult {
  readonly size: CatalogSize;
  readonly operationCount: number;
  /** One entry per axis (A-E), then one for `"overall"` — six in total, the same shape `MeasureResult.axisRecalls` uses. */
  readonly axisResults: readonly PickAxisResult[];
  readonly averageQueryMillis: number;
}

/** One pick row's full report: every catalogue size (`gather-pick.ts`). */
export interface PickConfigurationResult {
  readonly configId: string;
  readonly sizes: readonly PickSizeResult[];
}

/** How `gatherReport` reaches the transport and the vector cache — overridable so tests never reach llama-swap (AC-V-106). */
export interface GatherOptions {
  readonly fetchImpl?: FetchLike;
  readonly baseUrl?: string;
  readonly vectorsDir?: string;
  /** Where the generated layer's utterances are cached (spec section 5) — overridable only so a test never reads or writes the real `.utterances/` directory. */
  readonly utterancesDir?: string;
}
