import { expect, test } from "vite-plus/test";

import type { FixtureOperation } from "../fixture/index.ts";
import { checkContract, sampleOperations } from "./contract.ts";
import type { EmbeddingConfig } from "./configs.ts";
import type { FetchLike } from "./client.ts";

/**
 * The contract check's own tests (docs/plans/retrieving.md Task 1 Steps
 * 6-7, AC-V-101). Every test uses a fake transport, both a shape that
 * satisfies the contract and one that does not - a check that only ever
 * exercises the passing path would not prove it can fail.
 */

const CONFIG: EmbeddingConfig = {
  id: "test-config",
  model: "test-model",
  endpoint: "/v1/embeddings",
  pooling: "mean",
  queryPrefix: "QUERY> ",
  documentPrefix: "DOC> ",
};

function operation(index: number): FixtureOperation {
  return {
    operationId: `op${String(index)}`,
    service: "svc",
    serviceDisplayName: "Service",
    summary: `summary op${String(index)}`,
    description: "a description",
    displayName: `op${String(index)}`,
    isSetting: false,
    examples: [],
  };
}

const CATALOG: readonly FixtureOperation[] = [
  operation(0),
  operation(1),
  operation(2),
  operation(3),
];

/** The index a piece of embedded text belongs to, read off the `opN` marker every fixture text carries. */
function indexOf(text: string): number {
  const match = /op(\d+)/u.exec(text);

  return match?.[1] === undefined ? -1 : Number(match[1]);
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

/** The `input` array a request body carried, or `[]` when it carried none. */
function extractInput(body: unknown): readonly string[] {
  if (!isRecord(body)) return [];

  const { input } = body;

  if (!Array.isArray(input)) return [];

  return input.filter((item): item is string => typeof item === "string");
}

/** A fake transport where each text's vector is a one-hot vector at its own index - every text retrieves itself. */
function faithfulTransport(): FetchLike {
  return (_url, init) => {
    const body: unknown = JSON.parse(typeof init?.body === "string" ? init.body : "{}");
    const input = extractInput(body);
    const data = input.map((text, position) => ({
      index: position,
      embedding: CATALOG.map((_op, slot) => (slot === indexOf(text) ? 1 : 0)),
    }));

    return Promise.resolve(new Response(JSON.stringify({ data })));
  };
}

/** A fake transport where every text gets the same vector - only the first catalogue operation is ever retrieved. */
function collapsedTransport(): FetchLike {
  return (_url, init) => {
    const body: unknown = JSON.parse(typeof init?.body === "string" ? init.body : "{}");
    const input = extractInput(body);
    const data = input.map((_text, position) => ({ index: position, embedding: [1, 0, 0, 0] }));

    return Promise.resolve(new Response(JSON.stringify({ data })));
  };
}

test("sampleOperations returns the whole catalogue when it is smaller than the sample size", () => {
  expect(sampleOperations(CATALOG, 20).map((op) => op.operationId)).toEqual([
    "op0",
    "op1",
    "op2",
    "op3",
  ]);
});

test("sampleOperations steps by catalogue length over sample size", () => {
  const bigCatalog = Array.from({ length: 1000 }, (_unused, index) => operation(index));
  const sample = sampleOperations(bigCatalog, 20);

  expect(sample.map((op) => op.operationId)).toEqual(
    Array.from({ length: 20 }, (_u, i) => `op${String(i * 50)}`),
  );
});

test("a configuration that retrieves every sample by its own text passes", async () => {
  const result = await checkContract(CONFIG, CATALOG, faithfulTransport(), "http://fake");

  expect(result.passed).toBe(true);
});

test("a passing configuration reports no failures", async () => {
  const result = await checkContract(CONFIG, CATALOG, faithfulTransport(), "http://fake");

  expect(result.failedOperationIds).toEqual([]);
});

test("a configuration that collapses every vector fails", async () => {
  const result = await checkContract(CONFIG, CATALOG, collapsedTransport(), "http://fake");

  expect(result.passed).toBe(false);
});

test("a failing configuration names the operations it could not retrieve", async () => {
  const result = await checkContract(CONFIG, CATALOG, collapsedTransport(), "http://fake");

  expect(result.failedOperationIds).toEqual(["op1", "op2", "op3"]);
});
