import { expect, test } from "vite-plus/test";

import type { EmbeddingConfig } from "./configs.ts";
import type { CatalogueVectors } from "./cache.ts";
import { narrow, vectorNarrowerOf } from "./narrower.ts";
import type { FetchLike } from "./client.ts";

/**
 * The vector narrower's own tests (docs/plans/retrieving.md Task 2 Step 2).
 * A fake, tiny catalogue of hand-picked vectors where cosine similarity can
 * be worked out by hand, rather than the real fixture and a real model.
 */

const CONFIG: EmbeddingConfig = {
  id: "test-config",
  model: "test-model",
  endpoint: "/v1/embeddings",
  pooling: "mean",
  queryPrefix: "QUERY> ",
  documentPrefix: "DOC> ",
};

// Three orthonormal-ish axes, so cosine similarity to [1, 0, 0] ranks them
// unambiguously: opA matches exactly, opB is orthogonal, opC points away.
const VECTORS: CatalogueVectors = new Map([
  ["opA", [1, 0, 0]],
  ["opB", [0, 1, 0]],
  ["opC", [-1, 0, 0]],
]);

test("narrow orders operations by cosine similarity, best first", () => {
  const result = narrow(VECTORS, [1, 0, 0], 3);

  expect(result.map((r) => r.operationId)).toEqual(["opA", "opB", "opC"]);
});

test("narrow returns fewer than k when the catalogue is smaller than k", () => {
  const result = narrow(VECTORS, [1, 0, 0], 10);

  expect(result.length).toBe(3);
});

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

/** How many inputs a request body asked to embed, or 1 when that cannot be read. */
function inputCountOf(body: unknown): number {
  if (!isRecord(body)) return 1;

  const { input } = body;

  return Array.isArray(input) ? input.length : 1;
}

/** A fake transport that always answers with `vector`, whatever it was asked to embed. */
function fakeTransport(vector: readonly number[]): FetchLike {
  return (_url, init) => {
    const body: unknown = JSON.parse(typeof init?.body === "string" ? init.body : "{}");
    const count = inputCountOf(body);
    const data = Array.from({ length: count }, (_v, index) => ({ index, embedding: vector }));

    return Promise.resolve(new Response(JSON.stringify({ data })));
  };
}

test("a vectorNarrowerOf ranks the whole catalogue by the embedded question", async () => {
  const narrower = vectorNarrowerOf(VECTORS, CONFIG, fakeTransport([1, 0, 0]), "http://fake");
  const ranked = await narrower.rank("does not matter, the transport is fake");

  expect(ranked.map((r) => r.operationId)).toEqual(["opA", "opB", "opC"]);
});

test("a vectorNarrowerOf ranks the whole catalogue, not a fixed shortlist", async () => {
  const narrower = vectorNarrowerOf(VECTORS, CONFIG, fakeTransport([1, 0, 0]), "http://fake");
  const ranked = await narrower.rank("does not matter, the transport is fake");

  expect(ranked.length).toBe(3);
});
