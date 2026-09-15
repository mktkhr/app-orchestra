import type { PlanResult } from "@/shared/api/client";

import type { Alternative } from "./alternatives";

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

/**
 * A `result`'s further shortlist candidates, shown as their own assistant
 * turn after the answer - not a row inside it - reading 「違いましたか？」
 * (docs/specs/shortlisting.md, section 4, H5). The user's own words: two
 * chips clicked one after another used to stack two operations under one
 * question, because each click appended a new answer under the same
 * result; a person could tell the two operations apart from `AlternativesRow`
 * but not which question either belonged to.
 *
 * `chosen` is the picked alternative's `operationId`, set by the store the
 * moment a chip is clicked, before the re-plan request that follows even
 * resolves. It is what makes this turn "answered": once set, the chosen
 * chip renders selected and every other chip renders disabled, and a
 * second click on this turn is impossible because there is no enabled chip
 * left to click. `chosen` lives here, not in a component's own state, so
 * it survives whatever re-renders the store's own updates cause and so a
 * test can assert it without simulating a click twice.
 *
 * Skipped by `nearestQuestion` the same way `answer`/`choice` are: it
 * carries no question text of its own to resend.
 */
export interface AlternativesTurn {
  readonly id: string;
  readonly role: "alternatives";
  readonly text: "違いましたか？";
  readonly alternatives: readonly Alternative[];
  readonly chosen?: string;
}

export type Turn = QuestionTurn | AnswerTurn | ChoiceTurn | AlternativesTurn;
