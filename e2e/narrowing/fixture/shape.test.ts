import { expect, test } from "vite-plus/test";

import { catalogOf, operationCount, operationsOf, services } from "./index.ts";
import type { ServiceFixture } from "./types.ts";

const SHARED_KEYS = ["order", "line", "approval", "employee", "partner"];

function catalogSizes(): readonly number[] {
  return ([1, 2, 3, 5] as const).map((n) => catalogOf(n).length);
}

/** How many distinct services use each axis B shared key. */
function sharedKeyServiceCounts(): readonly number[] {
  const byKey = new Map<string, Set<string>>();

  for (const service of services()) {
    for (const resource of service.resources) {
      const key = resource.shared ?? "";
      const set = byKey.get(key) ?? new Set<string>();

      set.add(service.name);
      byKey.set(key, set);
    }
  }

  return SHARED_KEYS.map((key) => byKey.get(key)?.size ?? 0);
}

/** Whether each axis B shared key reaches two or more services. */
function sharedKeyMeetsMinimum(): readonly boolean[] {
  return sharedKeyServiceCounts().map((count) => count >= 2);
}

/** Per service, how many axis C groups reach four or more resources. */
function bigGroupCountsByService(): readonly number[] {
  return services().map((service) => {
    const sizeByGroup = new Map<string, number>();

    for (const resource of service.resources) {
      const key = resource.group ?? "";

      sizeByGroup.set(key, (sizeByGroup.get(key) ?? 0) + 1);
    }

    sizeByGroup.delete("");

    return [...sizeByGroup.values()].filter((size) => size >= 4).length;
  });
}

/** Whether each service has three or more such groups. */
function eachServiceHasThreeBigGroups(): readonly boolean[] {
  return bigGroupCountsByService().map((count) => count >= 3);
}

test("every service declares exactly 200 operations", () => {
  expect(services().map((s) => operationCount(s))).toEqual([200, 200, 200, 200, 200]);
});

test("the four sizes are 200, 400, 600 and 1000", () => {
  expect(catalogSizes()).toEqual([200, 400, 600, 1000]);
});

test("every axis B shared key reaches two or more services", () => {
  expect(sharedKeyMeetsMinimum()).toEqual(SHARED_KEYS.map(() => true));
});

test("every service has three or more near-neighbour groups of four or more (axis C)", () => {
  expect(eachServiceHasThreeBigGroups()).toEqual(services().map(() => true));
});

// G6: no fixture examples are written in this task - every real operation
// carries an empty examples array, not undefined, until Task 4 lands.
// AC-G-105: the written layer covers the whole fixture. Each service's own
// examples-<service>.test.ts checks its 200 operations in detail; this is
// the union - no operation anywhere is left without at least one example,
// which is what lets the "+written" rows of the report claim to measure the
// written layer rather than a fraction of it.
test("every operation in every service carries at least one written example", () => {
  const uncovered = services().flatMap((service) =>
    operationsOf(service)
      .filter((op) => op.examples.length === 0)
      .map((op) => op.operationId),
  );

  expect(uncovered).toEqual([]);
});

// AC-G-104: FixtureOperation.examples round-trips a definition table
// entry's per-verb/per-action examples, and is an empty array (not
// undefined) for an operation whose own verb/action carries none - a
// resource's "list" examples must not leak onto its "get" operation (G7:
// the verb is what separates candidates on axes B, C and E).
const EXAMPLES_FIXTURE: ServiceFixture = {
  name: "example",
  displayName: "サンプル",
  resources: [
    {
      id: "Thing",
      plural: "Things",
      noun: "モノ",
      verbs: ["list", "get"],
      examples: { list: ["モノを見せて"] },
    },
  ],
  aggregates: [{ id: "Overview", kind: "summarize", noun: "概要" }],
  settings: [],
  workflows: [],
};

test("FixtureOperation.examples carries the definition table's examples for that verb", () => {
  const list = operationsOf(EXAMPLES_FIXTURE).find((op) => op.operationId === "listExampleThings");

  expect(list?.examples).toEqual(["モノを見せて"]);
});

test("FixtureOperation.examples is empty on a verb the definition table gave no examples", () => {
  const get = operationsOf(EXAMPLES_FIXTURE).find((op) => op.operationId === "getExampleThing");

  expect(get?.examples).toEqual([]);
});

test("FixtureOperation.examples is empty, not undefined, when the entry declares none", () => {
  const summarize = operationsOf(EXAMPLES_FIXTURE).find(
    (op) => op.operationId === "summarizeExampleOverview",
  );

  expect(summarize?.examples).toEqual([]);
});
