import { describe, expect, it } from "vite-plus/test";

import { catalogOf, type FixtureOperation } from "./index.ts";

const MAX_EXAMPLE_LENGTH = 40;

function inventoryOperations(): readonly FixtureOperation[] {
  return catalogOf(5).filter((op) => op.service === "inventory");
}

/** Operations that carry no example at all. */
function operationsMissingExamples(
  operations: readonly FixtureOperation[],
): readonly FixtureOperation[] {
  return operations.filter((op) => op.examples.length === 0);
}

interface SubstringViolation {
  readonly operationId: string;
  readonly example: string;
  readonly against: string;
}

/** Every (operation, example) pair where the example and its summary or displayName overlap by substring, in either direction. */
function substringViolations(
  operations: readonly FixtureOperation[],
): readonly SubstringViolation[] {
  const violations: SubstringViolation[] = [];

  for (const op of operations) {
    for (const example of op.examples) {
      for (const against of [op.summary, op.displayName]) {
        if (example.includes(against) || against.includes(example)) {
          violations.push({ operationId: op.operationId, example, against });
        }
      }
    }
  }

  return violations;
}

interface LengthViolation {
  readonly operationId: string;
  readonly example: string;
  readonly length: number;
}

/** Every example longer than `MAX_EXAMPLE_LENGTH` characters. */
function lengthViolations(operations: readonly FixtureOperation[]): readonly LengthViolation[] {
  return operations.flatMap((op) =>
    op.examples
      .filter((example) => example.length > MAX_EXAMPLE_LENGTH)
      .map((example) => ({ operationId: op.operationId, example, length: example.length })),
  );
}

describe("inventory service examples", () => {
  it("covers all 200 operations", () => {
    const operations = inventoryOperations();

    expect(operations.length).toBe(200);
  });

  it("gives every operation at least one example", () => {
    const missing = operationsMissingExamples(inventoryOperations());

    expect(missing).toEqual([]);
  });

  it("keeps examples and their summary/displayName from overlapping by substring", () => {
    const violations = substringViolations(inventoryOperations());

    expect(violations).toEqual([]);
  });

  it("keeps every example at most 40 characters", () => {
    const violations = lengthViolations(inventoryOperations());

    expect(violations).toEqual([]);
  });
});
