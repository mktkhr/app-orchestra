import { expect, test } from "vite-plus/test";

import type { FixtureOperation } from "../fixture/index.ts";
import type { CatalogueVectors } from "../embedding/cache.ts";
import type { EmbeddingVector, FetchLike } from "../embedding/client.ts";
import type { UtteranceVectors } from "../embedding/utterance-cache.ts";
import { twoStageWithWrittenRerankerNarrowerOf } from "./reranker-written.ts";

/**
 * `twoStageWithWrittenRerankerNarrowerOf`'s own tests (TODO.md item 1): the
 * same shape `two-stage.test.ts` exercises for the plain reranker, plus the
 * one thing this narrower changes - the reranker's document text carries
 * the operation's written examples, not `combinedTextOf` alone.
 */

function op(operationId: string, summary: string, examples: readonly string[]): FixtureOperation {
  return {
    operationId,
    service: "test",
    serviceDisplayName: "",
    summary,
    description: "",
    displayName: "",
    isSetting: false,
    examples,
  };
}

const CATALOG: readonly FixtureOperation[] = [
  op("opA", "summary A", ["written example one", "written example two"]),
  op("opB", "summary B", []),
];

// opA's own vector points away from the question; only its utterance
// matches, the same fixture shape `two-stage.test.ts` uses.
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

/** A fake `/v1/rerank` transport that captures every document it was sent and reverses the order it hands back. */
function capturingRerankTransport(sink: unknown[][]): FetchLike {
  return (_url, init) => {
    const body: unknown = JSON.parse(typeof init?.body === "string" ? init.body : "{}");
    const documents = extractDocuments(body);

    sink.push([...documents]);

    const results = documents.map((_document, index) => ({ index, relevance_score: index }));

    return Promise.resolve(new Response(JSON.stringify({ results })));
  };
}

test("retrieval is scored with the written layer, exactly as the plain reranker row does it", async () => {
  const sentDocuments: unknown[][] = [];
  const narrower = twoStageWithWrittenRerankerNarrowerOf(
    VECTORS,
    UTTERANCES,
    CATALOG,
    QUESTION_VECTORS,
    capturingRerankTransport(sentDocuments),
    "http://fake",
  );
  const ranked = await narrower.rank("question text");

  // Both retrieved, opA first into retrieval thanks to its utterance, then
  // the fake reranker reverses whatever order it was handed.
  expect(ranked.map((result) => result.operationId)).toEqual(["opB", "opA"]);
});

test("the reranked documents carry each operation's written examples, one per line, after the rest of the combined text - or nothing, for an operation with none", async () => {
  const sentDocuments: unknown[][] = [];
  const narrower = twoStageWithWrittenRerankerNarrowerOf(
    VECTORS,
    UTTERANCES,
    CATALOG,
    QUESTION_VECTORS,
    capturingRerankTransport(sentDocuments),
    "http://fake",
  );

  await narrower.rank("question text");

  // Retrieval order (opA first: its utterance vector matches the question
  // exactly, opB's own vector only partly does) — the same order
  // `two-stage.test.ts` relies on for its own fake transport.
  expect(sentDocuments[0]).toEqual([
    "summary A\n\n\n\nwritten example one\nwritten example two",
    "summary B\n\n\n",
  ]);
});

test("throws for a question with no prefetched vector", async () => {
  const narrower = twoStageWithWrittenRerankerNarrowerOf(
    VECTORS,
    UTTERANCES,
    CATALOG,
    new Map(),
    capturingRerankTransport([]),
    "http://fake",
  );

  await expect(narrower.rank("unknown question")).rejects.toThrow("no prefetched vector");
});
