import { expect, test } from "vite-plus/test";

import type { FixtureOperation } from "../fixture/index.ts";
import type { CatalogueVectors } from "../embedding/cache.ts";
import type { EmbeddingVector, FetchLike } from "../embedding/client.ts";
import { RETRIEVE_COUNT, twoStageNarrowerOf } from "./narrower.ts";

/**
 * The two-stage narrower's own tests (docs/plans/retrieving.md Task 3 Step
 * 3). A fake, tiny catalogue and a fake rerank transport, not the real
 * fixture or a real model - `make check` must call no model of any kind
 * (AC-V-106).
 */

function op(operationId: string, summary: string): FixtureOperation {
  return {
    operationId,
    service: "test",
    serviceDisplayName: "",
    summary,
    description: "",
    displayName: "",
    isSetting: false,
  };
}

const CATALOG: readonly FixtureOperation[] = [
  op("opA", "summary A"),
  op("opB", "summary B"),
  op("opC", "summary C"),
];

// Cosine similarity to [1, 0, 0] ranks opA first, opB second, opC last -
// the retrieval stage's order, which the fake reranker below then reverses,
// so a passing test proves the reranked order was actually used.
const VECTORS: CatalogueVectors = new Map([
  ["opA", [1, 0, 0]],
  ["opB", [0.5, 0.5, 0]],
  ["opC", [0, 1, 0]],
]);

const QUESTION_VECTORS: ReadonlyMap<string, EmbeddingVector> = new Map([
  ["question text", [1, 0, 0]],
]);

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

/** The `documents` array a `/v1/rerank` request body carried, or `[]` when it carried none. */
function extractDocuments(body: unknown): readonly unknown[] {
  if (!isRecord(body)) return [];

  const { documents } = body;

  return Array.isArray(documents) ? documents : [];
}

/** A fake `/v1/rerank` transport that reverses whatever document order it was sent, out of order in the response on purpose. */
function reversingRerankTransport(): FetchLike {
  return (_url, init) => {
    const body: unknown = JSON.parse(typeof init?.body === "string" ? init.body : "{}");
    const documents = extractDocuments(body);
    // Score index 0 lowest and the last index highest, and hand the results
    // back in input order rather than score order - both departures from
    // the retrieval order are deliberate.
    const results = documents.map((_document, index) => ({ index, relevance_score: index }));

    return Promise.resolve(new Response(JSON.stringify({ results })));
  };
}

test("a twoStageNarrowerOf returns candidates in reranked order, not retrieval order", async () => {
  const narrower = twoStageNarrowerOf(
    VECTORS,
    CATALOG,
    QUESTION_VECTORS,
    reversingRerankTransport(),
    "http://fake",
  );
  const ranked = await narrower.rank("question text");

  expect(ranked.map((result) => result.operationId)).toEqual(["opC", "opB", "opA"]);
});

test("a twoStageNarrowerOf reranks at most RETRIEVE_COUNT candidates", async () => {
  const bigVectors: CatalogueVectors = new Map(
    Array.from({ length: RETRIEVE_COUNT + 20 }, (_unused, index) => [
      `op${String(index)}`,
      [1, 0, 0],
    ]),
  );
  const bigCatalog = Array.from({ length: RETRIEVE_COUNT + 20 }, (_unused, index) =>
    op(`op${String(index)}`, `summary ${String(index)}`),
  );
  const narrower = twoStageNarrowerOf(
    bigVectors,
    bigCatalog,
    QUESTION_VECTORS,
    reversingRerankTransport(),
    "http://fake",
  );
  const ranked = await narrower.rank("question text");

  expect(ranked.length).toBe(RETRIEVE_COUNT);
});

test("a twoStageNarrowerOf throws for a question with no prefetched vector", async () => {
  const narrower = twoStageNarrowerOf(
    VECTORS,
    CATALOG,
    new Map(),
    reversingRerankTransport(),
    "http://fake",
  );

  await expect(narrower.rank("unknown question")).rejects.toThrow("no prefetched vector");
});
