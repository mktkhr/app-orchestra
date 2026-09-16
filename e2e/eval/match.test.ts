import { expect, test } from "vite-plus/test";

import { matches, matchesAny } from "./match.ts";
import type { PlanOutcome } from "./types.ts";

/**
 * match.ts's own tests: no server, no model - every PlanOutcome here is a
 * fake (mirrors e2e/shortlist/score.test.ts's style).
 *
 * Covers argsAbsent/argsPresent (ExpectedOutcome), added for the
 * real-catalogue "no fabrication" cases (docs/specs/eval.md "Real-catalogue
 * cases"): a form's `initial` needs to be asserted absent a key nobody gave
 * a value for, or present with a guessed one, not just equal to a fixed
 * value the way `args` already checks.
 */

const formOutcome = (initial: Record<string, unknown>): PlanOutcome => ({
  kind: "form",
  target: { service: "attendance", operationId: "CreateAttendanceRecord" },
  initial,
});

test.each([
  { name: "argsAbsent passes when the key is missing", initial: {}, want: true },
  {
    name: "argsAbsent passes when other keys are present",
    initial: { kind: "deemed" },
    want: true,
  },
  {
    name: "argsAbsent fails when the key is present",
    initial: { employee: "田中太郎" },
    want: false,
  },
])("$name", ({ initial, want }) => {
  expect(
    matches(formOutcome(initial), {
      kind: "form",
      service: "attendance",
      operationId: "CreateAttendanceRecord",
      argsAbsent: ["employee"],
    }),
  ).toBe(want);
});

test.each([
  { name: "argsPresent fails when the key is missing", initial: {}, want: false },
  {
    name: "argsPresent fails when only other keys are present",
    initial: { kind: "deemed" },
    want: false,
  },
  {
    name: "argsPresent passes when the key is present, any value",
    initial: { employee: "田中太郎" },
    want: true,
  },
])("$name", ({ initial, want }) => {
  expect(
    matches(formOutcome(initial), {
      kind: "form",
      service: "attendance",
      operationId: "CreateAttendanceRecord",
      argsPresent: ["employee"],
    }),
  ).toBe(want);
});

test("argsAbsent checks a result's args, not just a form's initial", () => {
  const resultOutcome: PlanOutcome = {
    kind: "result",
    source: { service: "inventory", operationId: "ListInventoryItems", args: { status: "staged" } },
  };

  expect(
    matches(resultOutcome, {
      kind: "result",
      service: "inventory",
      operationId: "ListInventoryItems",
      argsAbsent: ["status"],
    }),
  ).toBe(false);
  expect(
    matches(resultOutcome, {
      kind: "result",
      service: "inventory",
      operationId: "ListInventoryItems",
      argsPresent: ["status"],
    }),
  ).toBe(true);
});

test("matchesAny still matches on the first outcome that satisfies argsAbsent or argsPresent", () => {
  const outcome = formOutcome({});

  expect(
    matchesAny(outcome, [
      { kind: "ask", param: "kind" },
      { kind: "none" },
      {
        kind: "form",
        service: "attendance",
        operationId: "CreateAttendanceRecord",
        argsAbsent: ["kind"],
      },
    ]),
  ).toBe(true);
});
