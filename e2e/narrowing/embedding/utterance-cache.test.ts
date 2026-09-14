import { mkdtempSync, readdirSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { afterEach, beforeEach, expect, test } from "vite-plus/test";

import type { FixtureOperation } from "../fixture/index.ts";
import { embedCatalogue } from "./cache.ts";
import type { EmbeddingConfig } from "./configs.ts";
import type { FetchLike } from "./client.ts";
import { embedUtteranceVectors } from "./utterance-cache.ts";

/**
 * The utterance vector cache's own tests (docs/plans/describing.md Task 2
 * Step 3): a second document set beside the catalogue's own vectors, never
 * invalidating them.
 */

const CONFIG: EmbeddingConfig = {
  id: "test-config",
  model: "test-model",
  endpoint: "/v1/embeddings",
  pooling: "mean",
  queryPrefix: "QUERY> ",
  documentPrefix: "DOC> ",
};

function operation(id: string, summary: string): FixtureOperation {
  return {
    operationId: id,
    service: "svc",
    serviceDisplayName: "Service",
    summary,
    description: "a description",
    displayName: id,
    isSetting: false,
    examples: [],
  };
}

const CATALOG: readonly FixtureOperation[] = [
  operation("opA", "summary a"),
  operation("opB", "summary b"),
];

const UTTERANCES = new Map([
  ["opA", ["utterance a1", "utterance a2"]],
  ["opB", ["utterance b1"]],
]);

let dir: string;

beforeEach(() => {
  dir = mkdtempSync(join(tmpdir(), "describing-utterance-cache-test-"));
});

afterEach(() => {
  rmSync(dir, { recursive: true, force: true });
});

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

/** A fake transport that counts calls and answers each input with a distinct vector. */
function fakeTransport(): { readonly fetchImpl: FetchLike; callCount: () => number } {
  let calls = 0;

  const fetchImpl: FetchLike = (_url, init) => {
    calls += 1;

    const body: unknown = JSON.parse(typeof init?.body === "string" ? init.body : "{}");
    const input = isRecord(body) && Array.isArray(body["input"]) ? body["input"] : [];
    const data = input.map((_text: unknown, index: number) => ({
      index,
      embedding: [index + 1, 0],
    }));

    return Promise.resolve(new Response(JSON.stringify({ data })));
  };

  return { fetchImpl, callCount: () => calls };
}

test("a first call reaches the transport once, batched", async () => {
  const { fetchImpl, callCount } = fakeTransport();

  await embedUtteranceVectors(CONFIG, "generated", CATALOG, UTTERANCES, {
    dir,
    fetchImpl,
    baseUrl: "http://fake",
  });

  expect(callCount()).toBe(1);
});

test("a second call with the same layer serves from disk", async () => {
  const { fetchImpl, callCount } = fakeTransport();

  await embedUtteranceVectors(CONFIG, "generated", CATALOG, UTTERANCES, {
    dir,
    fetchImpl,
    baseUrl: "http://fake",
  });
  await embedUtteranceVectors(CONFIG, "generated", CATALOG, UTTERANCES, {
    dir,
    fetchImpl,
    baseUrl: "http://fake",
  });

  expect(callCount()).toBe(1);
});

test("an operation's utterances come back in the order they were given, one vector each", async () => {
  const { fetchImpl } = fakeTransport();

  const result = await embedUtteranceVectors(CONFIG, "generated", CATALOG, UTTERANCES, {
    dir,
    fetchImpl,
    baseUrl: "http://fake",
  });

  expect(result.get("opA")?.length).toBe(2);
});

test("changing one operation's utterances misses only that layer's cache", async () => {
  const first = fakeTransport();
  const second = fakeTransport();
  const changed = new Map([...UTTERANCES, ["opA", ["a different utterance"]]]);

  await embedUtteranceVectors(CONFIG, "generated", CATALOG, UTTERANCES, {
    dir,
    fetchImpl: first.fetchImpl,
    baseUrl: "http://fake",
  });
  await embedUtteranceVectors(CONFIG, "generated", CATALOG, changed, {
    dir,
    fetchImpl: second.fetchImpl,
    baseUrl: "http://fake",
  });

  expect(second.callCount()).toBe(1);
});

test("a different setName is a separate cache entry, embedded independently", async () => {
  const first = fakeTransport();
  const second = fakeTransport();

  await embedUtteranceVectors(CONFIG, "generated", CATALOG, UTTERANCES, {
    dir,
    fetchImpl: first.fetchImpl,
    baseUrl: "http://fake",
  });
  await embedUtteranceVectors(CONFIG, "written", CATALOG, UTTERANCES, {
    dir,
    fetchImpl: second.fetchImpl,
    baseUrl: "http://fake",
  });

  expect(second.callCount()).toBe(1);
});

test("embedding utterance vectors does not touch the catalogue's own cache files", async () => {
  const catalogueTransport = fakeTransport();

  await embedCatalogue(CONFIG, CATALOG, {
    dir,
    fetchImpl: catalogueTransport.fetchImpl,
    baseUrl: "http://fake",
  });

  const before = readdirSync(dir).toSorted();

  const utteranceTransport = fakeTransport();

  await embedUtteranceVectors(CONFIG, "generated", CATALOG, UTTERANCES, {
    dir,
    fetchImpl: utteranceTransport.fetchImpl,
    baseUrl: "http://fake",
  });

  const catalogueVectorsAgain = await embedCatalogue(CONFIG, CATALOG, {
    dir,
    fetchImpl: () => Promise.reject(new Error("should not reach the transport")),
    baseUrl: "http://fake",
  });

  const after = new Set(readdirSync(dir));

  expect(before.every((name) => after.has(name))).toBe(true);
  expect(catalogueVectorsAgain.get("opA")).toEqual([1, 0]);
});
