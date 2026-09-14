/**
 * Talks to llama-swap's `/v1/embeddings` (docs/plans/retrieving.md Task 1,
 * the llama-swap table above it). Every call goes through a `fetch`-shaped
 * function passed in by the caller, defaulted to the real `fetch`, so a test
 * can hand it a fake transport instead - `make check` must call no model of
 * any kind (AC-V-106, spec section 8), and this is the seam that keeps that
 * true rather than something a test has to remember.
 *
 * A model's contract - which prefix a query or a document gets - lives in
 * `configs.ts`; this module only applies it. What it adds is: batching (64
 * inputs per request, so a thousand-operation catalogue is a handful of
 * calls rather than one enormous one or a thousand tiny ones), and
 * normalising every returned vector to unit length so a dot product is a
 * cosine similarity everywhere downstream (`cache.ts`, `contract.ts`, and
 * Task 2's narrower).
 */
import type { EmbeddingConfig } from "./configs.ts";

/** The shape of `fetch` - the seam a test replaces with a fake transport. */
export type FetchLike = typeof fetch;

/** llama-swap's OpenAI-compatible base, verified reachable on 2026-09-14. */
export const DEFAULT_BASE_URL = "http://localhost:11435";

/** Inputs per `/v1/embeddings` request (docs/plans/retrieving.md Task 1 Step 3). */
export const EMBEDDING_BATCH_SIZE = 64;

/** A single embedded and normalised vector. */
export type EmbeddingVector = readonly number[];

/** One text embedded as a query (a question) or as a document (catalogue text). */
export type EmbeddingKind = "query" | "document";

function prefixFor(config: EmbeddingConfig, kind: EmbeddingKind): string {
  return kind === "query" ? config.queryPrefix : config.documentPrefix;
}

/** `vector` scaled to length 1, or returned unchanged when it is already the zero vector. */
function normalise(vector: readonly number[]): EmbeddingVector {
  const magnitude = Math.sqrt(vector.reduce((sum, value) => sum + value * value, 0));

  return magnitude === 0 ? vector : vector.map((value) => value / magnitude);
}

/** `items` split into chunks of at most `size`, in order. */
function batchesOf<T>(items: readonly T[], size: number): readonly (readonly T[])[] {
  const chunks: T[][] = [];

  for (let start = 0; start < items.length; start += size) {
    chunks.push(items.slice(start, start + size));
  }

  return chunks;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

interface IndexedEmbedding {
  readonly index: number;
  readonly embedding: readonly number[];
}

/** One `data[]` entry of an OpenAI-compatible embeddings response, read by runtime check. */
function parseEmbeddingItem(value: unknown): IndexedEmbedding {
  if (!isRecord(value)) throw new Error("embeddings response item was not a JSON object");

  const { index, embedding } = value;

  if (typeof index !== "number") throw new Error("embeddings response item had no numeric index");
  if (!Array.isArray(embedding)) throw new Error("embeddings response item had no embedding array");

  const numbers = embedding.map((component) => {
    if (typeof component !== "number") {
      throw new TypeError("embeddings response item had a non-numeric component");
    }

    return component;
  });

  return { index, embedding: numbers };
}

/**
 * `body` read as an OpenAI-compatible embeddings response, reordered by
 * `index` - llama-swap does not promise `data` arrives in request order,
 * mirroring how `docs/plans/retrieving.md` Task 3 already warns the rerank
 * endpoint does not.
 */
function parseEmbeddingsResponse(body: unknown, expectedCount: number): readonly EmbeddingVector[] {
  if (!isRecord(body)) throw new Error("embeddings response was not a JSON object");

  const { data } = body;

  if (!Array.isArray(data)) throw new Error("embeddings response had no data array");

  const items = data.map((item) => parseEmbeddingItem(item)).toSorted((a, b) => a.index - b.index);

  if (items.length !== expectedCount) {
    throw new Error(
      `embeddings response returned ${String(items.length)} vectors for ${String(expectedCount)} inputs`,
    );
  }

  return items.map((item) => normalise(item.embedding));
}

async function embedBatch(
  config: EmbeddingConfig,
  texts: readonly string[],
  fetchImpl: FetchLike,
  baseUrl: string,
): Promise<readonly EmbeddingVector[]> {
  const response = await fetchImpl(`${baseUrl}${config.endpoint}`, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ model: config.model, input: texts }),
  });
  const body: unknown = await response.json();

  return parseEmbeddingsResponse(body, texts.length);
}

/**
 * Embeds `texts` as `kind`, applying `config`'s prefix for that kind and
 * batching at `EMBEDDING_BATCH_SIZE`. One vector per input, in the same
 * order, each normalised to unit length.
 */
export async function embedMany(
  config: EmbeddingConfig,
  texts: readonly string[],
  kind: EmbeddingKind,
  fetchImpl: FetchLike = fetch,
  baseUrl: string = DEFAULT_BASE_URL,
): Promise<readonly EmbeddingVector[]> {
  const prefix = prefixFor(config, kind);
  const prefixed = texts.map((text) => `${prefix}${text}`);
  const chunks = batchesOf(prefixed, EMBEDDING_BATCH_SIZE);
  const vectors: EmbeddingVector[] = [];

  for (const chunk of chunks) {
    vectors.push(...(await embedBatch(config, chunk, fetchImpl, baseUrl)));
  }

  return vectors;
}

/**
 * A single question's vector, computed at question time - never cached
 * (spec V4, AC-V-103): a question arrives one at a time, so there is
 * nothing to reuse a cache entry for.
 */
export async function embedQuestion(
  config: EmbeddingConfig,
  text: string,
  fetchImpl: FetchLike = fetch,
  baseUrl: string = DEFAULT_BASE_URL,
): Promise<EmbeddingVector> {
  const [vector] = await embedMany(config, [text], "query", fetchImpl, baseUrl);

  return vector ?? [];
}
