import { expect, test } from "vite-plus/test";

import type { EmbeddingConfig } from "./configs.ts";
import { embedMany, type FetchLike } from "./client.ts";

/**
 * The client's own tests (docs/plans/retrieving.md Task 1 Step 1). Every
 * test here uses a fake transport - `make check` must call no model of any
 * kind (AC-V-106) - so `FetchLike` is faked rather than the real `fetch`
 * ever being reached.
 */

const CONFIG: EmbeddingConfig = {
  id: "test-config",
  model: "test-model",
  endpoint: "/v1/embeddings",
  pooling: "mean",
  queryPrefix: "QUERY> ",
  documentPrefix: "DOC> ",
};

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

interface RecordedCall {
  readonly input: readonly string[];
}

/** A fake transport that records every request and answers with `vectorFor`'s vector, repeated per input. */
function fakeTransport(vectorFor: (text: string) => readonly number[]): {
  readonly fetchImpl: FetchLike;
  readonly calls: RecordedCall[];
} {
  const calls: RecordedCall[] = [];

  const fetchImpl: FetchLike = (_url, init) => {
    const body: unknown = JSON.parse(typeof init?.body === "string" ? init.body : "{}");
    const input = extractInput(body);

    calls.push({ input });

    const data = input.map((text, index) => ({ index, embedding: vectorFor(text) }));

    return Promise.resolve(new Response(JSON.stringify({ data })));
  };

  return { fetchImpl, calls };
}

test("a document gets the document prefix", async () => {
  const { fetchImpl, calls } = fakeTransport(() => [1, 0]);

  await embedMany(CONFIG, ["在庫ロット"], "document", fetchImpl, "http://fake");

  expect(calls[0]?.input).toEqual(["DOC> 在庫ロット"]);
});

test("a question gets the query prefix", async () => {
  const { fetchImpl, calls } = fakeTransport(() => [1, 0]);

  await embedMany(CONFIG, ["在庫ロットを見せて"], "query", fetchImpl, "http://fake");

  expect(calls[0]?.input).toEqual(["QUERY> 在庫ロットを見せて"]);
});

test("a long input list is split into batches of 64", async () => {
  const texts = Array.from({ length: 65 }, (_unused, index) => `text-${String(index)}`);
  const { fetchImpl, calls } = fakeTransport(() => [1, 0]);

  await embedMany(CONFIG, texts, "document", fetchImpl, "http://fake");

  expect(calls.map((call) => call.input.length)).toEqual([64, 1]);
});

test("a shorter input list is sent in a single batch", async () => {
  const { fetchImpl, calls } = fakeTransport(() => [1, 0]);

  await embedMany(CONFIG, ["one", "two", "three"], "document", fetchImpl, "http://fake");

  expect(calls.length).toBe(1);
});

/** The Euclidean length of `vector`, or 0 when there is none to measure. */
function magnitudeOf(vector: readonly number[] | undefined): number {
  return Math.sqrt((vector ?? []).reduce((sum, value) => sum + value * value, 0));
}

test("returned vectors are normalised to unit length", async () => {
  const { fetchImpl } = fakeTransport(() => [3, 4]);

  const [vector] = await embedMany(CONFIG, ["anything"], "document", fetchImpl, "http://fake");

  expect(magnitudeOf(vector)).toBeCloseTo(1);
});

test("normalisation preserves direction", async () => {
  const { fetchImpl } = fakeTransport(() => [3, 4]);

  const [vector] = await embedMany(CONFIG, ["anything"], "document", fetchImpl, "http://fake");

  expect(vector).toEqual([0.6, 0.8]);
});
