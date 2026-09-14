/**
 * Talks to llama-swap's `/v1/rerank` (`bge-reranker-v2-m3-q8`, the
 * llama-swap table above docs/plans/retrieving.md's Task 1) - the second
 * half of the two-stage configuration (spec V6, `narrower.ts`).
 *
 * `/v1/rerank` returns `{results: [{index, relevance_score}]}` where
 * `index` is the position of a document in the `documents` array that was
 * sent, and the results are **not sorted** (docs/plans/retrieving.md Task 3,
 * "the bug this task exists to avoid"). A client that assumed rank order, or
 * that lost the mapping from a result's `index` back to the candidate that
 * document came from, would produce a plausible-looking table that is
 * wrong - `client.test.ts`'s fake transport answers out of order on
 * purpose, to catch exactly that.
 */
import { DEFAULT_BASE_URL, type FetchLike } from "../embedding/client.ts";

/** The one model configured behind `/v1/rerank` (docs/plans/retrieving.md's llama-swap table). */
export const RERANK_MODEL = "bge-reranker-v2-m3-q8";

/** One document offered to the reranker, keyed by the id the caller wants back. */
export interface RerankCandidate {
  readonly id: string;
  readonly text: string;
}

/** One candidate's outcome: its id and the reranker's relevance score. */
export interface RerankResult {
  readonly id: string;
  readonly score: number;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

interface IndexedScore {
  readonly index: number;
  readonly score: number;
}

/** One `results[]` entry of a `/v1/rerank` response, read by runtime check. */
function parseResultItem(value: unknown): IndexedScore {
  if (!isRecord(value)) throw new Error("rerank response item was not a JSON object");

  const { index, relevance_score: relevanceScore } = value;

  if (typeof index !== "number") throw new Error("rerank response item had no numeric index");
  if (typeof relevanceScore !== "number") {
    throw new TypeError("rerank response item had no numeric relevance_score");
  }

  return { index, score: relevanceScore };
}

/**
 * `body` read as a `/v1/rerank` response - in whatever order it arrived,
 * because the endpoint does not promise an order (see this file's header).
 * Sorting happens in `rerank` below, once every result has been mapped back
 * to the candidate its `index` names.
 */
function parseRerankResponse(body: unknown, expectedCount: number): readonly IndexedScore[] {
  if (!isRecord(body)) throw new Error("rerank response was not a JSON object");

  const { results } = body;

  if (!Array.isArray(results)) throw new Error("rerank response had no results array");

  const items = results.map((item) => parseResultItem(item));

  if (items.length !== expectedCount) {
    throw new Error(
      `rerank response returned ${String(items.length)} scores for ${String(expectedCount)} documents`,
    );
  }

  return items;
}

function toRerankResult(item: IndexedScore, candidates: readonly RerankCandidate[]): RerankResult {
  const candidate = candidates[item.index];

  if (candidate === undefined) {
    throw new Error(`rerank response referenced document index ${String(item.index)} out of range`);
  }

  return { id: candidate.id, score: item.score };
}

/**
 * Reranks `candidates` against `query` with `bge-reranker-v2-m3-q8`, sorted
 * best (highest `relevance_score`) first. `candidates`' `text` must be the
 * same text that was embedded for retrieval - `combinedTextOf` (`../
 * lexical.ts`, spec V3) - so the reranker sees what the retriever saw.
 * Returns `[]` for no candidates without calling the transport at all.
 */
export async function rerank(
  query: string,
  candidates: readonly RerankCandidate[],
  fetchImpl: FetchLike = fetch,
  baseUrl: string = DEFAULT_BASE_URL,
): Promise<readonly RerankResult[]> {
  if (candidates.length === 0) return [];

  const response = await fetchImpl(`${baseUrl}/v1/rerank`, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({
      model: RERANK_MODEL,
      query,
      documents: candidates.map((candidate) => candidate.text),
    }),
  });
  const body: unknown = await response.json();
  const items = parseRerankResponse(body, candidates.length);

  return items
    .map((item) => toRerankResult(item, candidates))
    .toSorted((a, b) => b.score - a.score);
}
