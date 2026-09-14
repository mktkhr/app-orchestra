import { expect, test } from "vite-plus/test";

import { rerank, type RerankCandidate } from "./client.ts";
import type { FetchLike } from "../embedding/client.ts";

/**
 * The rerank client's own tests (docs/plans/retrieving.md Task 3 Step 1).
 * `/v1/rerank` returns `results: [{index, relevance_score}]` in **input
 * order**, not sorted by score - the fake transport below deliberately
 * answers out of order, because that is the exact bug this test exists to
 * catch: a client that assumes rank order, or loses the mapping from a
 * result's `index` back to the candidate that document came from, produces
 * a plausible-looking table that is wrong.
 */

const CANDIDATES: readonly RerankCandidate[] = [
  { id: "opA", text: "text of operation A" },
  { id: "opB", text: "text of operation B" },
  { id: "opC", text: "text of operation C" },
];

/**
 * A fake transport that answers with `results` exactly as given - callers
 * below hand it results out of order and out of score order, on purpose.
 */
/** A request's JSON body, read back as text, or `""` when it carried none. */
function stringBodyOf(init: RequestInit | undefined): string {
  return typeof init?.body === "string" ? init.body : "";
}

function fakeTransport(results: readonly { index: number; relevance_score: number }[]): FetchLike {
  return () => Promise.resolve(new Response(JSON.stringify({ results })));
}

test("rerank sorts by relevance_score descending even when the response arrives out of order", async () => {
  const fetchImpl = fakeTransport([
    { index: 1, relevance_score: 0.2 },
    { index: 2, relevance_score: 0.9 },
    { index: 0, relevance_score: 0.5 },
  ]);

  const results = await rerank("does it matter", CANDIDATES, fetchImpl, "http://fake");

  expect(results.map((result) => result.id)).toEqual(["opC", "opA", "opB"]);
});

test("rerank maps a result's index back to the candidate that document came from", async () => {
  const fetchImpl = fakeTransport([
    { index: 1, relevance_score: 0.2 },
    { index: 2, relevance_score: 0.9 },
    { index: 0, relevance_score: 0.5 },
  ]);

  const results = await rerank("does it matter", CANDIDATES, fetchImpl, "http://fake");

  expect(results.find((result) => result.id === "opC")?.score).toBe(0.9);
});

test("rerank sends the model, the query and every candidate's text as documents", async () => {
  let capturedBody = "";
  const fetchImpl: FetchLike = (_url, init) => {
    capturedBody = stringBodyOf(init);

    return Promise.resolve(
      new Response(
        JSON.stringify({
          results: CANDIDATES.map((_candidate, index) => ({ index, relevance_score: index })),
        }),
      ),
    );
  };

  await rerank("在庫を見せて", CANDIDATES, fetchImpl, "http://fake");
  const body: unknown = JSON.parse(capturedBody);

  expect(body).toEqual({
    model: "bge-reranker-v2-m3-q8",
    query: "在庫を見せて",
    documents: ["text of operation A", "text of operation B", "text of operation C"],
  });
});

/** A transport that always rejects — no candidates means `rerank` must never reach it. */
const neverCalledFetch: FetchLike = () => Promise.reject(new Error("should not be called"));

test("rerank returns nothing for no candidates, without calling the transport", async () => {
  const results = await rerank("does not matter", [], neverCalledFetch, "http://fake");

  expect(results).toEqual([]);
});
