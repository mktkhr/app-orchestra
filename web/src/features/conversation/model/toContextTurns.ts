import type { components } from "@/shared/api/gen/platform";

import type { Turn } from "./turn";

/** The platform's `Turn` (docs/specs/context.md section 3) - what `PlanRequest.turns` carries. */
export type ContextTurn = components["schemas"]["Turn"];

/**
 * Turns the browser's own alternating question/answer turns into the
 * platform's `Turn`: the question's text as it was typed, and the
 * `kind`/`service`/`operationId`/`args` the following answer's `source`
 * (a `result`) or `target` (a `form`) already named. Oldest first, matching
 * the order `turns` is already held in (docs/specs/context.md section 5).
 *
 * Nothing is derived beyond picking `source` or `target` apart, and `data`
 * is never read - the answer turn may not even carry it (`kind: "ask"` or
 * `"none"`), and section 3 is explicit that no row of any answer is sent.
 *
 * A question with no answer after it yet - one still in flight, or one
 * whose request failed - is dropped: there is no decision to report for it.
 * The same is true of an answer with no question right before it, which
 * happens when a form's submission (`submitForm`) appends a second answer
 * turn after the first: it continues the question before it rather than
 * asking a new one, so it has no text of its own to send as a `Turn.question`.
 */
export function toContextTurns(turns: readonly Turn[]): readonly ContextTurn[] {
  const result: ContextTurn[] = [];

  for (let index = 0; index < turns.length - 1; index += 1) {
    const question = turns[index];
    const answer = turns[index + 1];

    if (question?.role !== "question" || answer?.role !== "answer") {
      continue;
    }

    const decision = answer.result.source ?? answer.result.target;

    result.push({
      question: question.text,
      kind: answer.result.kind,
      ...(decision === undefined
        ? {}
        : { service: decision.service, operationId: decision.operationId }),
      ...(decision?.args === undefined ? {} : { args: decision.args }),
    });

    index += 1;
  }

  return result;
}
