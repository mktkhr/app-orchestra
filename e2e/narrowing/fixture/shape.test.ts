import { expect, test } from "vite-plus/test";

import { catalogOf, operationCount, services } from "./index.ts";

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
