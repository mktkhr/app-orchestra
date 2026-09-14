/**
 * Task 4 acceptance for the purchasing service: every one of its 200
 * operations carries examples, none of them lean on the operation's own
 * wording, and all of them stay short.
 */
import { expect, test } from "vite-plus/test";

import { catalogOf } from "./index.ts";
import type { FixtureOperation } from "./index.ts";

const MAX_EXAMPLE_LENGTH = 40;

function purchasingOperations(): readonly FixtureOperation[] {
  return catalogOf(5).filter((op) => op.service === "purchasing");
}

function operationsWithoutExamples(): readonly string[] {
  return purchasingOperations()
    .filter((op) => op.examples.length === 0)
    .map((op) => op.operationId);
}

/** An example that shares a literal substring with its own summary or displayName, either way. */
function operationsWithLeakingExamples(): readonly string[] {
  return purchasingOperations()
    .filter((op) =>
      op.examples.some(
        (example) =>
          op.summary.includes(example) ||
          example.includes(op.summary) ||
          op.displayName.includes(example) ||
          example.includes(op.displayName),
      ),
    )
    .map((op) => op.operationId);
}

function operationsWithOverlongExamples(): readonly string[] {
  return purchasingOperations()
    .filter((op) => op.examples.some((example) => example.length > MAX_EXAMPLE_LENGTH))
    .map((op) => op.operationId);
}

test("purchasing declares 200 operations", () => {
  expect(purchasingOperations().length).toBe(200);
});

test("every purchasing operation has at least one example", () => {
  expect(operationsWithoutExamples()).toEqual([]);
});

test("no purchasing example shares a literal substring with its own summary or displayName", () => {
  expect(operationsWithLeakingExamples()).toEqual([]);
});

test("every purchasing example is at most 40 characters", () => {
  expect(operationsWithOverlongExamples()).toEqual([]);
});
