import type { PlanResult } from "@/shared/api/client";

/** The person's question, shown as one turn in the conversation. */
export interface QuestionTurn {
  readonly id: string;
  readonly role: "question";
  readonly text: string;
}

/**
 * The planner's answer to a question, shown as the following turn.
 *
 * `result` is the raw `PlanResult` from `POST /api/plan`. How it renders by
 * `kind`/`component` (table, detail, form, choice) is Task 13-15's job; this
 * slice only carries the value.
 */
export interface AnswerTurn {
  readonly id: string;
  readonly role: "answer";
  readonly result: PlanResult;
}

/**
 * A person picking one of a `result`'s alternatives
 * (docs/specs/shortlisting.md, section 4, H5) - not a new question. `text`
 * is the original question `preferred` re-asks with, so `nearestQuestion`
 * (`TurnList`) can still find it for any answer that follows; `label` is
 * the alternative's own `displayName`, which is what this turn actually
 * shows. Kept distinct from `QuestionTurn` so a chip click never reads, to
 * the person, as the question being sent a second time.
 */
export interface ChoiceTurn {
  readonly id: string;
  readonly role: "choice";
  readonly text: string;
  readonly label: string;
}

export type Turn = QuestionTurn | AnswerTurn | ChoiceTurn;
