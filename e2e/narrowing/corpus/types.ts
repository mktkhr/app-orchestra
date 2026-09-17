/**
 * The corpus's own types (docs/plans/narrowing.md Task 4, Step 1). A
 * `Question` is what a person would type, not a restatement of an
 * operation's summary — see docs/specs/narrowing.md section 4 for the five
 * axes this corpus is built from.
 */

export type Axis = "A" | "B" | "C" | "D" | "E";

export interface Question {
  readonly id: string;
  readonly axis: Axis;
  /** The question, in Japanese, as a person would type it. */
  readonly text: string;
  /** Every operationId that would be a defensible answer. Axis B has several. */
  readonly answers: readonly string[];
  /**
   * Axis E only: the operation that is lexically closer than the answer and
   * is wrong. Asserted to actually out-score the answer, so the axis is
   * measured rather than claimed.
   */
  readonly decoy?: string;
}

/**
 * The mid corpus's own question shape (docs/specs/midsizing.md section 4,
 * docs/plans/midsizing.md Task 2). `axis` is reused from `Question` but is
 * informational here: the mid subset has no settings/aggregate operations
 * to build an axis E decoy from, and no same-service near-neighbour group
 * for axis C, so only A (single defensible answer), B (the subset's
 * sales/purchasing collision) and D (paraphrase) occur in practice, and the
 * impossible half is uniformly "A".
 */
export interface MidQuestion extends Question {
  /** "answerable": `answers` names the operations. "impossible": `answers` is empty and refusal is expected. */
  readonly expect: "answerable" | "impossible";
  /** Impossible only: `list_capabilities` is the correct refusal, not `none`. */
  readonly capability?: boolean;
}
