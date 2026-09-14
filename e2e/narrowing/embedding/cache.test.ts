import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { afterEach, beforeEach, expect, test } from "vite-plus/test";

import type { FixtureOperation } from "../fixture/index.ts";
import { embedCatalogue } from "./cache.ts";
import type { EmbeddingConfig } from "./configs.ts";
import type { FetchLike } from "./client.ts";

/**
 * The cache's own tests (docs/plans/retrieving.md Task 1 Steps 4-5). Every
 * test uses a fake transport and a throwaway directory, so the miss is
 * tested and not only the hit (AC-V-102): a cache that never invalidates
 * looks correct right up until the fixture changes.
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
  };
}

const CATALOG: readonly FixtureOperation[] = [
  operation("opA", "summary a"),
  operation("opB", "summary b"),
];

let dir: string;

beforeEach(() => {
  dir = mkdtempSync(join(tmpdir(), "retrieving-cache-test-"));
});

afterEach(() => {
  rmSync(dir, { recursive: true, force: true });
});

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

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

test("a first call reaches the transport", async () => {
  const { fetchImpl, callCount } = fakeTransport();

  await embedCatalogue(CONFIG, CATALOG, { dir, fetchImpl, baseUrl: "http://fake" });

  expect(callCount()).toBe(1);
});

test("a second call with the same config and catalogue serves from disk", async () => {
  const { fetchImpl, callCount } = fakeTransport();

  await embedCatalogue(CONFIG, CATALOG, { dir, fetchImpl, baseUrl: "http://fake" });
  await embedCatalogue(CONFIG, CATALOG, { dir, fetchImpl, baseUrl: "http://fake" });

  expect(callCount()).toBe(1);
});

test("the served vectors match what the transport returned", async () => {
  const { fetchImpl } = fakeTransport();

  await embedCatalogue(CONFIG, CATALOG, { dir, fetchImpl, baseUrl: "http://fake" });
  const second = await embedCatalogue(CONFIG, CATALOG, { dir, fetchImpl, baseUrl: "http://fake" });

  expect(second.get("opA")).toEqual([1, 0]);
});

test("changing one operation's text misses the cache", async () => {
  const first = fakeTransport();
  const second = fakeTransport();
  const changedCatalog = [
    operation("opA", "a different summary entirely"),
    operation("opB", "summary b"),
  ];

  await embedCatalogue(CONFIG, CATALOG, {
    dir,
    fetchImpl: first.fetchImpl,
    baseUrl: "http://fake",
  });
  await embedCatalogue(CONFIG, changedCatalog, {
    dir,
    fetchImpl: second.fetchImpl,
    baseUrl: "http://fake",
  });

  expect(second.callCount()).toBe(1);
});

test("changing the configuration's document prefix misses the cache", async () => {
  const first = fakeTransport();
  const second = fakeTransport();
  const changedConfig: EmbeddingConfig = { ...CONFIG, documentPrefix: "OTHER> " };

  await embedCatalogue(CONFIG, CATALOG, {
    dir,
    fetchImpl: first.fetchImpl,
    baseUrl: "http://fake",
  });
  await embedCatalogue(changedConfig, CATALOG, {
    dir,
    fetchImpl: second.fetchImpl,
    baseUrl: "http://fake",
  });

  expect(second.callCount()).toBe(1);
});
