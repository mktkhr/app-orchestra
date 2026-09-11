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

export type Turn = QuestionTurn | AnswerTurn;
