/**
 * The corpus's public surface: 100 questions, 25/25/25/15/10 across the five
 * axes of difficulty (docs/specs/narrowing.md section 4, section 5).
 */
import { AXIS_A } from "./axis-a.ts";
import { AXIS_B } from "./axis-b.ts";
import { AXIS_C } from "./axis-c.ts";
import { AXIS_D } from "./axis-d.ts";
import { AXIS_D2 } from "./axis-d2.ts";
import { AXIS_E } from "./axis-e.ts";
import { AXIS_E2 } from "./axis-e2.ts";
import type { Question } from "./types.ts";

export type { Axis, Question } from "./types.ts";

export function questions(): readonly Question[] {
  return [...AXIS_A, ...AXIS_B, ...AXIS_C, ...AXIS_D, ...AXIS_E];
}

/**
 * 50 more questions, 25 axis D and 25 axis E (`axis-d2.ts`, `axis-e2.ts`),
 * held apart from `questions()` so the original 100 questions — and the
 * numbers already recorded against them — stay comparable across
 * measurements. A mechanism that wants a larger axis D/E sample reads this
 * instead of, or in addition to, `questions()`.
 */
export function extensionQuestions(): readonly Question[] {
  return [...AXIS_D2, ...AXIS_E2];
}
