/**
 * The corpus's public surface: 100 questions, 25/25/25/15/10 across the five
 * axes of difficulty (docs/specs/narrowing.md section 4, section 5).
 */
import { AXIS_A } from "./axis-a.ts";
import { AXIS_B } from "./axis-b.ts";
import { AXIS_C } from "./axis-c.ts";
import { AXIS_D } from "./axis-d.ts";
import { AXIS_E } from "./axis-e.ts";
import type { Question } from "./types.ts";

export type { Axis, Question } from "./types.ts";

export function questions(): readonly Question[] {
  return [...AXIS_A, ...AXIS_B, ...AXIS_C, ...AXIS_D, ...AXIS_E];
}
