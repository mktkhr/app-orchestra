/**
 * Shapes shared by every file under e2e/eval/ (docs/specs/eval.md).
 *
 * A case names a question and the decisions that would answer it - not the
 * platform's wire format directly, so a case reads the way docs/specs/eval.md
 * section 3 writes one. `plan-client.ts` maps the actual `/api/plan` response
 * onto {@link PlanOutcome}; `match.ts` compares an outcome against an
 * {@link ExpectedOutcome}.
 */

/** One earlier turn a case's question follows, exactly as PlanRequest.turns carries it. */
export interface CaseTurn {
  readonly question: string;
  readonly kind: string;
  readonly service?: string;
  readonly operationId?: string;
  readonly args?: Record<string, unknown>;
}

/**
 * A decision that would count as `accept` or `reject` for a case - the same
 * fields docs/specs/eval.md section 3's YAML shows, matched loosely against
 * whichever of `source`/`target`/`initial`/`param` the actual `kind` carries
 * (see match.ts). A field left out of an expected outcome is not checked;
 * `args: {}` is checked exactly, meaning "no arguments at all".
 */
export interface ExpectedOutcome {
  readonly kind: "result" | "form" | "ask" | "none";
  readonly service?: string;
  readonly operationId?: string;
  readonly args?: Record<string, unknown>;
  readonly param?: string;
}

/** One case: a question (with, optionally, the conversation before it) and what would answer it. */
export interface Case {
  readonly id: string;
  readonly question: string;
  readonly turns?: readonly CaseTurn[];
  readonly accept: readonly ExpectedOutcome[];
  readonly reject?: readonly ExpectedOutcome[];
}

/** /api/plan's response, narrowed to the fields a case's expected outcomes can name. */
export interface PlanOutcome {
  readonly kind: string;
  readonly source?: {
    readonly service?: string;
    readonly operationId?: string;
    readonly args?: Record<string, unknown>;
  };
  readonly target?: { readonly service?: string; readonly operationId?: string };
  readonly initial?: Record<string, unknown>;
  readonly param?: string;
}

/** How one case's N runs came out: how many matched accept, reject, or neither. */
export interface CaseTally {
  readonly id: string;
  readonly total: number;
  readonly accept: number;
  readonly reject: number;
}
