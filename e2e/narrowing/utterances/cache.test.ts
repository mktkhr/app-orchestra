import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { afterEach, beforeEach, expect, test } from "vite-plus/test";

import type { FetchLike } from "../embedding/client.ts";
import type { FixtureOperation } from "../fixture/index.ts";
import { generateUtterances } from "./cache.ts";

/**
 * The cache's own tests (docs/plans/describing.md Task 1 Steps 4-5,
 * AC-G-101). Every test uses a fake transport and a throwaway directory, so
 * the miss is tested and not only the hit: a cache that never invalidates
 * looks correct right up until an operation's text, or the prompt, changes.
 */

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

let dir: string;

beforeEach(() => {
  dir = mkdtempSync(join(tmpdir(), "utterances-cache-test-"));
});

afterEach(() => {
  rmSync(dir, { recursive: true, force: true });
});

/** A fake transport that counts calls and always answers with the same two lines. */
function fakeTransport(): { readonly fetchImpl: FetchLike; readonly callCount: () => number } {
  let calls = 0;

  const fetchImpl: FetchLike = () => {
    calls += 1;

    return Promise.resolve(
      new Response(
        JSON.stringify({
          choices: [{ message: { content: "一つ目\n二つ目" }, finish_reason: "stop" }],
        }),
      ),
    );
  };

  return { fetchImpl, callCount: () => calls };
}

test("a first call reaches the transport once per operation", async () => {
  const { fetchImpl, callCount } = fakeTransport();

  await generateUtterances(CATALOG, { dir, fetchImpl, baseUrl: "http://fake" });

  expect(callCount()).toBe(2);
});

test("a second call with the same catalogue calls the transport zero times", async () => {
  const first = fakeTransport();
  const second = fakeTransport();

  await generateUtterances(CATALOG, { dir, fetchImpl: first.fetchImpl, baseUrl: "http://fake" });
  await generateUtterances(CATALOG, { dir, fetchImpl: second.fetchImpl, baseUrl: "http://fake" });

  expect(second.callCount()).toBe(0);
});

test("the served utterances match what the transport returned", async () => {
  const { fetchImpl } = fakeTransport();

  await generateUtterances(CATALOG, { dir, fetchImpl, baseUrl: "http://fake" });
  const second = await generateUtterances(CATALOG, {
    dir,
    fetchImpl: fakeTransport().fetchImpl,
    baseUrl: "http://fake",
  });

  expect(second.get("opA")).toEqual(["一つ目", "二つ目"]);
});

test("changing one operation's text regenerates only that operation", async () => {
  const first = fakeTransport();
  const second = fakeTransport();
  const changedCatalog = [
    operation("opA", "a different summary entirely"),
    operation("opB", "summary b"),
  ];

  await generateUtterances(CATALOG, { dir, fetchImpl: first.fetchImpl, baseUrl: "http://fake" });
  await generateUtterances(changedCatalog, {
    dir,
    fetchImpl: second.fetchImpl,
    baseUrl: "http://fake",
  });

  expect(second.callCount()).toBe(1);
});

test("changing the prompt regenerates every operation", async () => {
  const first = fakeTransport();
  const second = fakeTransport();

  await generateUtterances(CATALOG, { dir, fetchImpl: first.fetchImpl, baseUrl: "http://fake" });
  await generateUtterances(CATALOG, {
    dir,
    fetchImpl: second.fetchImpl,
    baseUrl: "http://fake",
    prompt: "a different prompt entirely",
  });

  expect(second.callCount()).toBe(2);
});
