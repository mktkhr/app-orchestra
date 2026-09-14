import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { afterEach, beforeEach, expect, test } from "vite-plus/test";

import type { FetchLike } from "../embedding/client.ts";
import type { EmbeddingConfig } from "../embedding/configs.ts";
import type { FixtureOperation } from "../fixture/index.ts";
import type { OperationUtterances } from "./cache.ts";
import { checkUtteranceContract } from "./contract.ts";

/**
 * The utterance contract check's own tests (docs/plans/describing.md Task 1
 * Steps 6-7, AC-G-103). Every test uses a fake transport and a throwaway
 * vectors directory, both a shape that satisfies the contract and one that
 * does not - a check that only ever exercises the passing path would not
 * prove it can fail.
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

const CATALOG: readonly FixtureOperation[] = Array.from({ length: 1000 }, (_unused, index) =>
  operation(index),
);

/** The `opN` marker a piece of text carries, or `-1` when it carries none. */
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

/** A fake transport where every text retrieves the operation whose `opN` marker it contains. */
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

let dir: string;

beforeEach(() => {
  dir = mkdtempSync(join(tmpdir(), "utterance-contract-test-"));
});

afterEach(() => {
  rmSync(dir, { recursive: true, force: true });
});

test("an utterance that names its own operation passes", async () => {
  const utterances: OperationUtterances = new Map([["op0", ["op0を見たい"]]]);

  const result = await checkUtteranceContract(CONFIG, CATALOG, utterances, {
    dir,
    fetchImpl: faithfulTransport(),
    baseUrl: "http://fake",
  });

  expect(result.rate).toBe(1);
});

test("a passing check reports no failures", async () => {
  const utterances: OperationUtterances = new Map([["op0", ["op0を見たい"]]]);

  const result = await checkUtteranceContract(CONFIG, CATALOG, utterances, {
    dir,
    fetchImpl: faithfulTransport(),
    baseUrl: "http://fake",
  });

  expect(result.failures).toEqual([]);
});

test("an utterance that names a different operation fails and is reported", async () => {
  // op0 is sampled (index 0), but its utterance points to op1's marker instead.
  const utterances: OperationUtterances = new Map([["op0", ["op1みたいな話"]]]);

  const result = await checkUtteranceContract(CONFIG, CATALOG, utterances, {
    dir,
    fetchImpl: faithfulTransport(),
    baseUrl: "http://fake",
  });

  expect(result.failures).toEqual([
    { utterance: "op1みたいな話", operationId: "op0", wonBy: "op1" },
  ]);
});

test("a failure still counts toward the rate", async () => {
  const utterances: OperationUtterances = new Map([["op0", ["op1みたいな話"]]]);

  const result = await checkUtteranceContract(CONFIG, CATALOG, utterances, {
    dir,
    fetchImpl: faithfulTransport(),
    baseUrl: "http://fake",
  });

  expect(result.rate).toBe(0);
});

test("only sampled operations' utterances count toward the rate", async () => {
  // op1 is not in the every-50th sample (0, 50, 100, ...), so its utterance is ignored.
  const utterances: OperationUtterances = new Map([["op1", ["op1を見たい"]]]);

  const result = await checkUtteranceContract(CONFIG, CATALOG, utterances, {
    dir,
    fetchImpl: faithfulTransport(),
    baseUrl: "http://fake",
  });

  expect(result.totalUtterances).toBe(0);
});

test("an operation with no utterances contributes nothing to the sample total", async () => {
  const utterances: OperationUtterances = new Map();

  const result = await checkUtteranceContract(CONFIG, CATALOG, utterances, {
    dir,
    fetchImpl: faithfulTransport(),
    baseUrl: "http://fake",
  });

  expect(result.totalUtterances).toBe(0);
});
