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
test("no service's operations carry any examples yet", () => {
  const counts = services().flatMap((service) =>
    operationsOf(service).map((op) => op.examples.length),
  );

  expect(counts.every((count) => count === 0)).toBe(true);
});

// AC-G-104: FixtureOperation.examples round-trips a definition table
// entry's examples field, and is an empty array (not undefined) when the
// entry declares none.
const EXAMPLES_FIXTURE: ServiceFixture = {
  name: "example",
  displayName: "サンプル",
  resources: [
    {
      id: "Thing",
      plural: "Things",
      noun: "モノ",
      verbs: ["list", "get"],
      examples: ["モノを見せて"],
    },
  ],
  aggregates: [{ id: "Overview", kind: "summarize", noun: "概要" }],
  settings: [],
  workflows: [],
};

test("FixtureOperation.examples carries the definition table's examples", () => {
  const list = operationsOf(EXAMPLES_FIXTURE).find((op) => op.operationId === "listExampleThings");

  expect(list?.examples).toEqual(["モノを見せて"]);
});

test("FixtureOperation.examples is empty, not undefined, when the entry declares none", () => {
  const summarize = operationsOf(EXAMPLES_FIXTURE).find(
    (op) => op.operationId === "summarizeExampleOverview",
  );

  expect(summarize?.examples).toEqual([]);
});
