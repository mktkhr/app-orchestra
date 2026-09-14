import { expect, test } from "vite-plus/test";

import type { FixtureOperation } from "../fixture/index.ts";
import type { CatalogueVectors } from "../embedding/cache.ts";
import type { EmbeddingVector, FetchLike } from "../embedding/client.ts";
import type { UtteranceVectors } from "../embedding/utterance-cache.ts";
import { twoStageWithUtterancesNarrowerOf } from "./two-stage.ts";

/**
 * `twoStageWithUtterancesNarrowerOf`'s own tests (docs/plans/describing.md
 * Task 2): the same shape `rerank/narrower.test.ts` exercises, with an
 * operation that only wins retrieval because of its utterance, and a fake
 * rerank transport that proves the reranked order (not retrieval order) is
 * what comes back.
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
    examples: [],
  };
}

const CATALOG: readonly FixtureOperation[] = [op("opA", "summary A"), op("opB", "summary B")];

// opA's own vector points away from the question; only its utterance
// matches. Without the utterance layer opB would retrieve first.
const VECTORS: CatalogueVectors = new Map([
  ["opA", [-1, 0, 0]],
  ["opB", [0.5, 0.5, 0]],
]);

const UTTERANCES: UtteranceVectors = new Map([["opA", [[1, 0, 0]]]]);

const QUESTION_VECTORS: ReadonlyMap<string, EmbeddingVector> = new Map([
  ["question text", [1, 0, 0]],
]);

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function extractDocuments(body: unknown): readonly unknown[] {
  if (!isRecord(body)) return [];

  const { documents } = body;

  return Array.isArray(documents) ? documents : [];
}

/** A fake `/v1/rerank` transport that reverses whatever order it was sent. */
function reversingRerankTransport(): FetchLike {
  return (_url, init) => {
    const body: unknown = JSON.parse(typeof init?.body === "string" ? init.body : "{}");
    const documents = extractDocuments(body);
    const results = documents.map((_document, index) => ({ index, relevance_score: index }));

    return Promise.resolve(new Response(JSON.stringify({ results })));
  };
}

test("retrieval is scored with the utterance layer before reranking", async () => {
  const narrower = twoStageWithUtterancesNarrowerOf(
    VECTORS,
    UTTERANCES,
    CATALOG,
    QUESTION_VECTORS,
    reversingRerankTransport(),
    "http://fake",
  );
  const ranked = await narrower.rank("question text");

  // Both retrieved, opA first into retrieval thanks to its utterance, then
  // the fake reranker reverses whatever order it was handed.
  expect(ranked.map((result) => result.operationId)).toEqual(["opB", "opA"]);
});

test("throws for a question with no prefetched vector", async () => {
  const narrower = twoStageWithUtterancesNarrowerOf(
    VECTORS,
    UTTERANCES,
    CATALOG,
    new Map(),
    reversingRerankTransport(),
    "http://fake",
  );

  await expect(narrower.rank("unknown question")).rejects.toThrow("no prefetched vector");
});
