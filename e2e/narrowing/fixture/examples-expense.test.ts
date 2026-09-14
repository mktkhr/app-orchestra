/**
 * Structural checks on the expense service's `x-orchestra-examples`: every
 * operation the service generates carries at least one example, no example
 * degenerates into (or absorbs) the operation's summary/displayName, and
 * every example stays short. Content quality (everyday phrasing, verb
 * distinctness) is judged by the measurement this fixture feeds, not here.
 */
import { expect, test } from "vite-plus/test";
import { expense } from "./expense.ts";
import type { FixtureOperation } from "./index.ts";
import { operationsOf } from "./index.ts";

const MAX_EXAMPLE_LENGTH = 40;

const operations: readonly FixtureOperation[] = operationsOf(expense);

/** operationIds carrying zero examples. */
function operationsMissingExamples(ops: readonly FixtureOperation[]): readonly string[] {
  return ops.filter((op) => op.examples.length === 0).map((op) => op.operationId);
}

/** "<operationId>: <example>" for every example over the length limit. */
function overlongExamples(ops: readonly FixtureOperation[]): readonly string[] {
  return ops.flatMap((op) =>
    op.examples
      .filter((example) => example.length > MAX_EXAMPLE_LENGTH)
      .map((example) => `${op.operationId}: ${example} (${example.length} chars)`),
  );
}

/** Whether either string contains the other. */
function overlaps(a: string, b: string): boolean {
  return a.includes(b) || b.includes(a);
}

/**
 * "<operationId>: <example>" for every example that is a substring of its
 * own operation's summary or displayName, or that has one as a substring.
 */
function collidingExamples(ops: readonly FixtureOperation[]): readonly string[] {
  return ops.flatMap((op) =>
    op.examples
      .filter((example) => overlaps(example, op.summary) || overlaps(example, op.displayName))
      .map((example) => `${op.operationId}: ${example}`),
  );
}

test("the expense service exposes exactly 200 operations", () => {
  expect(operations.length).toBe(200);
});

test("every expense operation carries at least one example", () => {
  expect(operationsMissingExamples(operations)).toEqual([]);
});

test("no expense example is longer than 40 characters", () => {
  expect(overlongExamples(operations)).toEqual([]);
});

test("no expense example collides with its operation's summary or displayName", () => {
  expect(collidingExamples(operations)).toEqual([]);
});
