import { expect, test } from "vite-plus/test";

import { services } from "./index.ts";
import { toOpenAPI } from "./openapi.ts";
import type { OpenAPIDocument, Operation } from "./openapi.ts";

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
