import { expect, test } from "vite-plus/test";

import { catalogOf } from "./index.ts";
import type { FixtureOperation } from "./index.ts";

function attendanceOperations(): readonly FixtureOperation[] {
  return catalogOf(5).filter((op) => op.service === "attendance");
}

function operationsMissingExamples(): readonly string[] {
  return attendanceOperations()
    .filter((op) => op.examples.length === 0)
    .map((op) => op.operationId);
}

interface Overlap {
  readonly operationId: string;
  readonly example: string;
  readonly against: string;
}

function overlappingExamples(): readonly Overlap[] {
  return attendanceOperations().flatMap((op) =>
    op.examples
      .filter(
        (example) =>
          op.summary.includes(example) ||
          op.displayName.includes(example) ||
          example.includes(op.summary) ||
          example.includes(op.displayName),
      )
      .map((example) => ({ operationId: op.operationId, example, against: op.summary })),
  );
}

function overlongExamples(): readonly string[] {
  return attendanceOperations().flatMap((op) =>
    op.examples.filter((example) => example.length > 40),
  );
}

test("attendance has 200 operations", () => {
  expect(attendanceOperations().length).toBe(200);
});

test("every attendance operation has at least one example", () => {
  expect(operationsMissingExamples()).toEqual([]);
});

test("no attendance example overlaps its operation's summary or displayName", () => {
  expect(overlappingExamples()).toEqual([]);
});

test("every attendance example is at most 40 characters", () => {
  expect(overlongExamples()).toEqual([]);
});
