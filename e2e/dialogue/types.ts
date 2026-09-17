import type { CaseTurn, ExpectedOutcome, PlanOutcome } from "../eval/types.ts";

/**
 * Shapes shared by every file under e2e/dialogue/, mirroring how
 * e2e/eval/types.ts anchors eval/ - except a dialogue is a *sequence* of
 * questions asked in one conversation, not one independent question, so
 * scoring needs to know a turn's position within its dialogue as well as
 * whether it passed.
 */

/** One question in a dialogue and what would answer it, in the order it is asked. */
export interface DialogueTurn {
  readonly question: string;
  readonly accept: readonly ExpectedOutcome[];
}

/** One multi-turn conversation, run once against the real dummy services and the real planner. */
export interface Dialogue {
  readonly id: string;
  readonly turns: readonly DialogueTurn[];
}

/**
 * One turn as it was actually run: the request sent, what the platform
 * answered, what would have counted as correct, and whether it did.
 * `turnIndex` is 0-based - the same turn a `DialogueTurn` occupies in its
 * `Dialogue.turns` - so score.ts can tell a dialogue's first turn (no
 * conversation before it) apart from a follow-up without re-deriving it
 * from `requestTurns.length`.
 */
export interface TurnRecord {
  readonly dialogueId: string;
  readonly turnIndex: number;
  readonly question: string;
  /** The exact `turns` sent alongside `question` - built by chain.ts's buildTurns from every earlier turn's real outcome. */
  readonly requestTurns: readonly CaseTurn[];
  readonly outcome: PlanOutcome;
  /** The unparsed /api/plan response body, for a record a person can read without re-deriving it from outcome. */
  readonly rawResponse: unknown;
  readonly expected: readonly ExpectedOutcome[];
  readonly pass: boolean;
  readonly latencyMs: number;
}
