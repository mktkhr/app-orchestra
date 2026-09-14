import { expect, test } from "vite-plus/test";

import { services } from "./index.ts";
import { toOpenAPI } from "./openapi.ts";
import type { OpenAPIDocument, Operation } from "./openapi.ts";
import type { ServiceFixture } from "./types.ts";

function operationsOf(doc: OpenAPIDocument): readonly Operation[] {
  const found: Operation[] = [];

  for (const item of Object.values(doc.paths)) {
    for (const op of [item.get, item.post, item.put, item.delete]) {
      if (op !== undefined) found.push(op);
    }
  }

  return found;
}

function allOperations(): readonly Operation[] {
  return services().flatMap((service) => operationsOf(toOpenAPI(service)));
}

/** Every enum schema across all five documents, `x-enum-labels` and `enum` lengths paired up. */
function enumLabelLengths(): readonly { readonly labels: number; readonly values: number }[] {
  const found: { labels: number; values: number }[] = [];

  for (const service of services()) {
    for (const schema of Object.values(toOpenAPI(service).components.schemas)) {
      if ("enum" in schema) {
        found.push({
          labels: Object.keys(schema["x-enum-labels"]).length,
          values: schema.enum.length,
        });
      }
    }
  }

  return found;
}

test("operation ids are unique across all five services", () => {
  const ids = allOperations().map((op) => op.operationId);

  expect(new Set(ids).size).toBe(ids.length);
  expect(ids.length).toBe(1000);
});

test("every exposed operation is marked x-orchestra-expose", () => {
  expect(allOperations().every((op) => op["x-orchestra-expose"])).toBe(true);
});

test("every exposed operation has a non-empty x-ui-hint.displayName", () => {
  expect(allOperations().every((op) => op["x-ui-hint"].displayName.length > 0)).toBe(true);
});

test("every enum carries x-enum-labels of the same length as its enum", () => {
  const lengths = enumLabelLengths();

  expect(lengths.length).toBeGreaterThan(0);
  expect(lengths.every((pair) => pair.labels === pair.values)).toBe(true);
});

function hasResponseContent(op: Operation): boolean {
  return Object.values(op.responses).some((r) => r.content !== undefined);
}

function hasContractSurface(op: Operation): boolean {
  return op.requestBody !== undefined || hasResponseContent(op);
}

test("every exposed operation has a response schema or a request body", () => {
  expect(allOperations().every((op) => hasContractSurface(op))).toBe(true);
});

// AC-G-104: x-orchestra-examples round-trips through toOpenAPI when the
// definition table's entry declares it, and is absent (not an empty
// array) when it does not - neither the fixture nor any real service
// writes examples in this task (G6), so this is a synthetic fixture.
//
// examples is keyed per verb/action, not shared across a resource's or a
// workflow's whole set of operations: "list" carries examples here, "get"
// and the other three CRUD verbs on the same resource do not, and a
// generator whose examples cross verbs would manufacture exactly the false
// match spec G7 warns about on axes B, C and E.
const EXAMPLE_FIXTURE: ServiceFixture = {
  name: "example",
  displayName: "サンプル",
  resources: [
    {
      id: "Thing",
      plural: "Things",
      noun: "モノ",
      verbs: ["list", "get"],
      examples: { list: ["モノを見せて", "モノの一覧"] },
    },
  ],
  aggregates: [{ id: "Overview", kind: "summarize", noun: "概要" }],
  settings: [{ id: "Rule", verb: "get", summary: "ルール取得", displayName: "ルール" }],
  workflows: [
    {
      id: "Ticket",
      noun: "チケット",
      actions: ["submit", "approve"],
      examples: { submit: ["チケットを申請して"] },
    },
  ],
};

function findOperation(doc: OpenAPIDocument, operationId: string): Operation {
  const op = operationsOf(doc).find((candidate) => candidate.operationId === operationId);

  if (op === undefined) throw new Error(`no operation ${operationId} in the synthesised document`);

  return op;
}

test("toOpenAPI emits x-orchestra-examples on the verb the definition table keys them by", () => {
  const doc = toOpenAPI(EXAMPLE_FIXTURE);
  const list = findOperation(doc, "listExampleThings");

  expect(list["x-orchestra-examples"]).toEqual(["モノを見せて", "モノの一覧"]);
});

test("toOpenAPI omits x-orchestra-examples on a resource's other verbs", () => {
  const doc = toOpenAPI(EXAMPLE_FIXTURE);
  const get = findOperation(doc, "getExampleThing");

  expect(get["x-orchestra-examples"]).toBeUndefined();
});

test("toOpenAPI emits x-orchestra-examples on the action a workflow's examples key by", () => {
  const doc = toOpenAPI(EXAMPLE_FIXTURE);
  const submit = findOperation(doc, "submitExampleTicket");

  expect(submit["x-orchestra-examples"]).toEqual(["チケットを申請して"]);
});

test("toOpenAPI omits x-orchestra-examples on a workflow's other actions", () => {
  const doc = toOpenAPI(EXAMPLE_FIXTURE);
  const approve = findOperation(doc, "approveExampleTicket");

  expect(approve["x-orchestra-examples"]).toBeUndefined();
});

test("toOpenAPI omits x-orchestra-examples when the definition table declares none", () => {
  const doc = toOpenAPI(EXAMPLE_FIXTURE);
  const summarize = findOperation(doc, "summarizeExampleOverview");

  expect(summarize["x-orchestra-examples"]).toBeUndefined();
});
