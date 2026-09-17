import { describe, expect, test } from "vite-plus/test";

import { catalogOf, services } from "./index.ts";
import { midCatalog, midServices } from "./mid.ts";
import { toOpenAPI } from "./openapi.ts";
import type { Operation, OpenAPIDocument } from "./openapi.ts";
import type { ServiceFixture } from "./types.ts";

/**
 * The mid subset (docs/specs/midsizing.md M1-M2, docs/plans/midsizing.md
 * Task 1): three services cut to two resources each, with a `pattern` on
 * every get/update/delete id parameter, while the full fixture is
 * byte-unaffected - no full document ever grows a `pattern`, and
 * `catalogOf(5)` still names all thousand operations.
 */

const EXPECTED_MID_OPERATION_IDS: ReadonlySet<string> = new Set([
  "listSalesOrders",
  "getSalesOrder",
  "createSalesOrder",
  "updateSalesOrder",
  "deleteSalesOrder",
  "listSalesPartners",
  "getSalesPartner",
  "createSalesPartner",
  "updateSalesPartner",
  "deleteSalesPartner",
  "listPurchasingOrders",
  "getPurchasingOrder",
  "createPurchasingOrder",
  "updatePurchasingOrder",
  "deletePurchasingOrder",
  "listPurchasingPartners",
  "getPurchasingPartner",
  "createPurchasingPartner",
  "updatePurchasingPartner",
  "deletePurchasingPartner",
  "listAttendanceEmployees",
  "getAttendanceEmployee",
  "createAttendanceEmployee",
  "updateAttendanceEmployee",
  "deleteAttendanceEmployee",
  "listAttendanceLeaveRequests",
  "getAttendanceLeaveRequest",
  "createAttendanceLeaveRequest",
  "updateAttendanceLeaveRequest",
  "deleteAttendanceLeaveRequest",
]);

function allOperationsOf(doc: OpenAPIDocument): readonly Operation[] {
  const found: Operation[] = [];

  for (const item of Object.values(doc.paths)) {
    for (const op of [item.get, item.post, item.put, item.delete]) {
      if (op !== undefined) found.push(op);
    }
  }

  return found;
}

/** Every id path parameter's `pattern`, from a mid service's generated document. */
function idPatternsOf(doc: OpenAPIDocument): readonly string[] {
  return allOperationsOf(doc).flatMap((op) =>
    (op.parameters ?? [])
      .filter((param) => param.name === "id")
      .map((param) => param.schema.pattern),
  );
}

function idVerbOperationsOf(doc: OpenAPIDocument): readonly Operation[] {
  return allOperationsOf(doc).filter((op) =>
    ["get", "update", "delete"].some((verb) => op.operationId.startsWith(verb)),
  );
}

function nonIdVerbOperationsOf(doc: OpenAPIDocument): readonly Operation[] {
  return allOperationsOf(doc).filter((op) =>
    ["list", "create"].some((verb) => op.operationId.startsWith(verb)),
  );
}

/** The mid subset always sets this - never undefined at runtime. */
function requireIdPrefix(service: ServiceFixture): string {
  const { idPrefix } = service;

  if (idPrefix === undefined) throw new Error(`${service.name}: mid subset missing idPrefix`);

  return idPrefix;
}

function fullDocumentsSerialized(): readonly string[] {
  return services().map((service) => JSON.stringify(toOpenAPI(service)));
}

function anyFullDocumentContainsPattern(): boolean {
  return fullDocumentsSerialized().some((doc) => doc.includes("pattern"));
}

function midOperationIds(): ReadonlySet<string> {
  return new Set(midCatalog().map((op) => op.operationId));
}

function countOfService(name: string): number {
  return midCatalog().filter((op) => op.service === name).length;
}

test("the full fixture stays byte-identical: catalogOf(5) still has 1000 operations", () => {
  expect(catalogOf(5).length).toBe(1000);
});

test("no full service's generated document contains a pattern", () => {
  expect(anyFullDocumentContainsPattern()).toBe(false);
});

test("midCatalog has exactly 30 operations", () => {
  expect(midCatalog().length).toBe(30);
});

test("midCatalog has 10 sales operations", () => {
  expect(countOfService("sales")).toBe(10);
});

test("midCatalog has 10 purchasing operations", () => {
  expect(countOfService("purchasing")).toBe(10);
});

test("midCatalog has 10 attendance operations", () => {
  expect(countOfService("attendance")).toBe(10);
});

test("midCatalog has the expected thirty operation ids", () => {
  expect(midOperationIds()).toStrictEqual(EXPECTED_MID_OPERATION_IDS);
});

test("every mid operation keeps its two written examples", () => {
  expect(midCatalog().every((op) => op.examples.length === 2)).toBe(true);
});

describe.each(midServices())("$name", (service: ServiceFixture) => {
  test("declares a pattern matching its prefix on every get/update/delete", () => {
    const doc = toOpenAPI(service);
    const patterns = idPatternsOf(doc);
    const idOps = idVerbOperationsOf(doc);
    const expectedPattern = `^${requireIdPrefix(service)}[0-9]+$`;

    expect(patterns.length).toBe(idOps.length);
    expect(patterns.length).toBeGreaterThan(0);
    expect(patterns.every((pattern) => pattern === expectedPattern)).toBe(true);
  });

  test("list and create operations carry no id pattern", () => {
    const doc = toOpenAPI(service);
    const nonIdOps = nonIdVerbOperationsOf(doc);

    expect(nonIdOps.length).toBeGreaterThan(0);
    expect(nonIdOps.every((op) => op.parameters === undefined)).toBe(true);
  });
});
