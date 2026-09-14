import { expect, test } from "vite-plus/test";

import type { FetchLike } from "../embedding/client.ts";
import {
  UTTERANCE_MAX_TOKENS,
  UTTERANCE_MODEL,
  UTTERANCE_PROMPT,
  generateUtterancesFor,
} from "./generate.ts";

/**
 * The generator's own tests (docs/plans/describing.md Task 1 Step 2). Every
 * test here uses a fake transport - `make check` must call no model of any
 * kind - so `FetchLike` is faked rather than the real `fetch` ever being
 * reached.
 */

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

interface RecordedCall {
  readonly body: Record<string, unknown>;
}

/** A fake transport that always answers with `content` as the chat message and records every request body. */
function fakeTransport(content: string): {
  readonly fetchImpl: FetchLike;
  readonly calls: RecordedCall[];
} {
  const calls: RecordedCall[] = [];

  const fetchImpl: FetchLike = (_url, init) => {
    const body: unknown = JSON.parse(typeof init?.body === "string" ? init.body : "{}");

    calls.push({ body: isRecord(body) ? body : {} });

    return Promise.resolve(
      new Response(JSON.stringify({ choices: [{ message: { content }, finish_reason: "stop" }] })),
    );
  };

  return { fetchImpl, calls };
}

test("five lines become five trimmed strings", async () => {
  const { fetchImpl } = fakeTransport("  一つ目  \n二つ目\n三つ目\n四つ目\n五つ目");

  const utterances = await generateUtterancesFor(
    "何かの操作",
    UTTERANCE_PROMPT,
    fetchImpl,
    "http://fake",
  );

  expect(utterances).toEqual(["一つ目", "二つ目", "三つ目", "四つ目", "五つ目"]);
});

test("blank lines are dropped", async () => {
  const { fetchImpl } = fakeTransport("一つ目\n\n二つ目\n\n\n三つ目");

  const utterances = await generateUtterancesFor(
    "何かの操作",
    UTTERANCE_PROMPT,
    fetchImpl,
    "http://fake",
  );

  expect(utterances).toEqual(["一つ目", "二つ目", "三つ目"]);
});

test("fewer than five lines are kept as-is, not padded", async () => {
  const { fetchImpl } = fakeTransport("一つ目\n二つ目");

  const utterances = await generateUtterancesFor(
    "何かの操作",
    UTTERANCE_PROMPT,
    fetchImpl,
    "http://fake",
  );

  expect(utterances).toEqual(["一つ目", "二つ目"]);
});

test("the request carries temperature 0", async () => {
  const { fetchImpl, calls } = fakeTransport("一つ目");

  await generateUtterancesFor("何かの操作", UTTERANCE_PROMPT, fetchImpl, "http://fake");

  expect(calls[0]?.body["temperature"]).toBe(0);
});

test("the request turns thinking off", async () => {
  const { fetchImpl, calls } = fakeTransport("一つ目");

  await generateUtterancesFor("何かの操作", UTTERANCE_PROMPT, fetchImpl, "http://fake");

  expect(calls[0]?.body["chat_template_kwargs"]).toEqual({ enable_thinking: false });
});

test("the request names the configured model", async () => {
  const { fetchImpl, calls } = fakeTransport("一つ目");

  await generateUtterancesFor("何かの操作", UTTERANCE_PROMPT, fetchImpl, "http://fake");

  expect(calls[0]?.body["model"]).toBe(UTTERANCE_MODEL);
});

test("the request caps max_tokens", async () => {
  const { fetchImpl, calls } = fakeTransport("一つ目");

  await generateUtterancesFor("何かの操作", UTTERANCE_PROMPT, fetchImpl, "http://fake");

  expect(calls[0]?.body["max_tokens"]).toBe(UTTERANCE_MAX_TOKENS);
});

/** The first chat message's `content` a recorded call's body carried, or `undefined` when it did not have one. */
function firstMessageContent(call: RecordedCall | undefined): unknown {
  const messages = call?.body["messages"];
  const first: unknown = Array.isArray(messages) ? messages[0] : undefined;

  return isRecord(first) ? first["content"] : undefined;
}

test("the prompt and the operation's text both reach the request, under a 操作: label", async () => {
  const { fetchImpl, calls } = fakeTransport("一つ目");

  await generateUtterancesFor("在庫ロットの一覧", "テストプロンプト", fetchImpl, "http://fake");

  expect(firstMessageContent(calls[0])).toBe("テストプロンプト\n\n操作:\n在庫ロットの一覧");
});
