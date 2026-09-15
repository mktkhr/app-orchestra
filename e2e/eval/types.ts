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

/**
 * Which rate a case is judged on (docs/specs/eval.md section 4): `"accept"`
 * (default) fails the run when the accept rate drops: the case wants to see
 * that any of the defensible answers still gets chosen. `"reject"` fails it
 * when the reject rate rises instead - the direction is flipped because for
 * a case like `no-enum-value` the accept rate is free to swing between
 * asking and guessing (both are fine); what regressing looks like is the
 * specific wrong outcome in `reject` becoming more common, which a rising
 * accept rate would not catch and a falling one would not mean.
 */
export type CaseMetric = "accept" | "reject";

/** One case: a question (with, optionally, the conversation before it) and what would answer it. */
export interface Case {
  readonly id: string;
  readonly question: string;
  readonly turns?: readonly CaseTurn[];
  readonly accept: readonly ExpectedOutcome[];
  readonly reject?: readonly ExpectedOutcome[];
  /** Which rate this case is judged on. Defaults to `"accept"` when left out. */
  readonly metric?: CaseMetric;
  /**
   * How many times to run this case, overriding `run.ts`'s default
   * (`ORCHESTRA_EVAL_N`). Left out for every case whose rate holds steady at
   * n=10; set explicitly for one measured to need more runs to tell noise
   * from a regression (DECISIONS.md, no-enum-value judged on reject).
   */
  readonly runs?: number;
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
  /** Which of accept/reject this case is judged on (Case.metric, defaulted). */
  readonly metric: CaseMetric;
  /**
   * The runs that matched neither `accept` nor `reject`, counted by what
   * they were - `"ask"`, `"none"`, `"error"`, a `result` naming a different
   * operation, and so on.
   *
   * `docs/specs/eval.md` section 3 says such a run "is the model doing
   * something new, and that is worth seeing on its own", and until now
   * there was no way to see it: the report printed accept and reject, and
   * everything else was a gap between two numbers. A case reading 6/10
   * accept and 0/10 reject said nothing about whether the other four runs
   * asked a question, answered a different operation, or failed outright -
   * which is the difference between a model being careful and a model
   * being wrong.
   */
  readonly others: Readonly<Record<string, number>>;
}
