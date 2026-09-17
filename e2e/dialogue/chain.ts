import type { CaseTurn, PlanOutcome } from "../eval/types.ts";

/** One question already asked in a dialogue and the platform's real answer to it. */
export interface DialogueStep {
  readonly question: string;
  readonly outcome: PlanOutcome;
}

/**
 * Builds the `turns` to send with the next question, from every question
 * asked so far in the same dialogue and the platform's actual outcome to
 * each - exactly the rule `web/src/features/conversation/model/toContextTurns.ts`
 * uses to turn the browser's own turns into `PlanRequest.turns`: the
 * question's text, the answer's `kind`, and `service`/`operationId`/`args`
 * from whichever of `source` (a `result`) or `target` (a `form`) the
 * outcome carries. An `ask` or `none` outcome carries neither, so its turn
 * is sent with `kind` alone - a fair record of what the platform actually
 * did, not a made-up decision.
 *
 * Unlike toContextTurns, nothing here is ever dropped: every question a
 * dialogue chains through this file already has a real outcome (run.ts
 * calls postPlanRaw and waits for it before moving on), where the browser
 * can have a question still in flight or one whose request failed.
 */
export function buildTurns(steps: readonly DialogueStep[]): readonly CaseTurn[] {
  return steps.map(({ question, outcome }) => {
    const decision = outcome.source ?? outcome.target;

    return {
      question,
      kind: outcome.kind,
      ...(decision?.service !== undefined && { service: decision.service }),
      ...(decision?.operationId !== undefined && { operationId: decision.operationId }),
      ...(decision?.args !== undefined && { args: decision.args }),
    };
  });
}
