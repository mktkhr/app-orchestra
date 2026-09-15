import type { Turn } from "../model/turn";

/**
 * The text of the nearest `role: "question"` turn before `index`, walking
 * backward from it.
 *
 * A `kind: "ask"` answer carries the planner's own disambiguation question
 * (`PlanResult.question`), not what the person actually typed, and
 * resubmitting `answers` to `/api/plan` needs exactly that original text
 * (`docs/specs/orchestration.md` section 6). Rather than thread a `query`
 * field through `Turn` - every producer of an answer turn would have to
 * remember to set it, including `ResultForm`'s `onSubmitted` path, which has
 * no query to give - this reads it back out of the turn list `Conversation`
 * already keeps. Walking backward instead of just taking `turns[index - 1]`
 * keeps this correct once a choice has already been answered once: the
 * turn right before a second `ask` may be another answer, but the nearest
 * question is still the one to resend.
 *
 * A `role: "choice"` turn (a chip click, docs/specs/shortlisting.md,
 * section 4, H5) is skipped the same way an answer turn is: it carries the
 * original question's text too, but only `role: "question"` counts here,
 * so an answer that follows a choice still finds the question the choice
 * itself resent, not the choice's own label.
 *
 * Split out of `TurnList.tsx` (a component file, subject to
 * `react/only-export-components`) so this pure function can be exported and
 * unit-tested directly.
 */
export function nearestQuestion(turns: readonly Turn[], index: number): string {
  for (let cursor = index - 1; cursor >= 0; cursor -= 1) {
    const candidate = turns[cursor];

    if (candidate?.role === "question") {
      return candidate.text;
    }
  }

  return "";
}
