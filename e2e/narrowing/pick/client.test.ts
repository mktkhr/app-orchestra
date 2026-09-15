import { expect, test } from "vite-plus/test";

import {
  PICK_MODEL,
  PICK_SYSTEM_PROMPT,
  newPickGuardState,
  pick,
  type PickCandidate,
} from "./client.ts";
import type { FetchLike } from "../embedding/client.ts";

/**
 * The picker client's own tests: request shape, the longest-id-first
 * parsing rule, the `ambiguous` flag, and the thinking-budget guard
 * (this file's header in `client.ts`) firing once and only once per run.
 */

const CANDIDATES: readonly PickCandidate[] = [
  {
    operationId: "listInventoryItems",
    serviceDisplayName: "在庫管理",
    summary: "在庫の一覧を返す",
  },
  {
    operationId: "listInventoryLots",
    serviceDisplayName: "在庫管理",
    summary: "ロットの一覧を返す",
  },
];

/** A request's JSON body, read back as a parsed object. */
function bodyOf(init: RequestInit | undefined): unknown {
  const raw = typeof init?.body === "string" ? init.body : "{}";

  return JSON.parse(raw);
}

interface FakeChoice {
  readonly content: string;
  readonly finishReason?: string;
  readonly reasoningContent?: string;
}

function fakeTransport(choice: FakeChoice): FetchLike {
  return () =>
    Promise.resolve(
      new Response(
        JSON.stringify({
          choices: [
            {
              finish_reason: choice.finishReason ?? "stop",
              message: {
                content: choice.content,
                ...(choice.reasoningContent === undefined
                  ? {}
                  : { reasoning_content: choice.reasoningContent }),
              },
            },
          ],
        }),
      ),
    );
}

/** A stand-in reply that satisfies the thinking-budget guard, plus a `body()` reading back the last request captured. */
function capturingFetch(): { readonly fetchImpl: FetchLike; readonly body: () => unknown } {
  let captured: unknown = {};
  const fetchImpl: FetchLike = (_url, init) => {
    captured = bodyOf(init);

    return Promise.resolve(
      new Response(
        JSON.stringify({ choices: [{ finish_reason: "stop", message: { content: "" } }] }),
      ),
    );
  };

  return { fetchImpl, body: () => captured };
}

/** The `/v1/chat/completions` request body `pick` should send for `question` and `candidates`, given a formatted user message's content. */
function expectedRequestBody(
  question: string,
  userContent: string,
  systemPrompt: string = PICK_SYSTEM_PROMPT,
): unknown {
  return {
    model: PICK_MODEL,
    temperature: 0,
    max_tokens: 200,
    chat_template_kwargs: { enable_thinking: false },
    messages: [
      { role: "system", content: systemPrompt },
      { role: "user", content: `質問: ${question}\n\n候補:\n${userContent}` },
    ],
  };
}

test("pick finds the longest candidate id when one is a substring of another", async () => {
  const result = await pick(
    "在庫を見せて",
    CANDIDATES,
    newPickGuardState(),
    fakeTransport({ content: "listInventoryLots certain" }),
    "http://fake",
  );

  expect(result.operationId).toBe("listInventoryLots");
});

test("pick returns an empty operationId when no candidate id appears in the response", async () => {
  const result = await pick(
    "在庫を見せて",
    CANDIDATES,
    newPickGuardState(),
    fakeTransport({ content: "something unrelated" }),
    "http://fake",
  );

  expect(result.operationId).toBe("");
});

test("pick flags ambiguous when the response contains that word, case-insensitively", async () => {
  const result = await pick(
    "在庫を見せて",
    CANDIDATES,
    newPickGuardState(),
    fakeTransport({ content: "listInventoryItems AMBIGUOUS" }),
    "http://fake",
  );

  expect(result.ambiguous).toBe(true);
});

test("pick does not flag ambiguous when the word is absent", async () => {
  const result = await pick(
    "在庫を見せて",
    CANDIDATES,
    newPickGuardState(),
    fakeTransport({ content: "listInventoryItems certain" }),
    "http://fake",
  );

  expect(result.ambiguous).toBe(false);
});

test("pick sends the model, temperature 0, max_tokens 200, thinking disabled, the verbatim system prompt and a formatted user message", async () => {
  const { fetchImpl, body } = capturingFetch();

  await pick("在庫を見せて", CANDIDATES, newPickGuardState(), fetchImpl, "http://fake");

  expect(body()).toEqual(
    expectedRequestBody(
      "在庫を見せて",
      "listInventoryItems\t在庫管理\t在庫の一覧を返す\n" +
        "listInventoryLots\t在庫管理\tロットの一覧を返す",
    ),
  );
});

test("pick appends an e.g. column for a candidate whose examples are non-empty (TODO.md item 1)", async () => {
  const { fetchImpl, body } = capturingFetch();
  const candidatesWithExamples: readonly PickCandidate[] = [
    {
      operationId: "listInventoryItems",
      serviceDisplayName: "在庫管理",
      summary: "在庫の一覧を返す",
      examples: ["在庫を見せて", "在庫の一覧が見たい"],
    },
    {
      operationId: "listInventoryLots",
      serviceDisplayName: "在庫管理",
      summary: "ロットの一覧を返す",
    },
  ];

  await pick("在庫を見せて", candidatesWithExamples, newPickGuardState(), fetchImpl, "http://fake");

  expect(body()).toEqual(
    expectedRequestBody(
      "在庫を見せて",
      "listInventoryItems\t在庫管理\t在庫の一覧を返す\te.g. 在庫を見せて / 在庫の一覧が見たい\n" +
        "listInventoryLots\t在庫管理\tロットの一覧を返す",
    ),
  );
});

test("pick omits the e.g. column entirely when a candidate's examples are absent or empty", async () => {
  const { fetchImpl, body } = capturingFetch();
  const candidatesWithEmptyExamples: readonly PickCandidate[] = [
    {
      operationId: "listInventoryItems",
      serviceDisplayName: "在庫管理",
      summary: "在庫の一覧を返す",
      examples: [],
    },
    {
      operationId: "listInventoryLots",
      serviceDisplayName: "在庫管理",
      summary: "ロットの一覧を返す",
    },
  ];

  await pick(
    "在庫を見せて",
    candidatesWithEmptyExamples,
    newPickGuardState(),
    fetchImpl,
    "http://fake",
  );

  expect(body()).toEqual(
    expectedRequestBody(
      "在庫を見せて",
      "listInventoryItems\t在庫管理\t在庫の一覧を返す\n" +
        "listInventoryLots\t在庫管理\tロットの一覧を返す",
    ),
  );
});

test("pick rejects on the first response when finish_reason is not stop", async () => {
  await expect(
    pick(
      "q",
      CANDIDATES,
      newPickGuardState(),
      fakeTransport({ content: "listInventoryItems certain", finishReason: "length" }),
      "http://fake",
    ),
  ).rejects.toThrow(/thinking leaked/u);
});

test("pick rejects on the first response when reasoning_content is non-empty", async () => {
  await expect(
    pick(
      "q",
      CANDIDATES,
      newPickGuardState(),
      fakeTransport({ content: "listInventoryItems certain", reasoningContent: "let me think..." }),
      "http://fake",
    ),
  ).rejects.toThrow(/thinking leaked/u);
});

test("pick accepts a first response with finish_reason stop and no reasoning_content", async () => {
  const result = await pick(
    "q",
    CANDIDATES,
    newPickGuardState(),
    fakeTransport({ content: "listInventoryItems certain" }),
    "http://fake",
  );

  expect(result.operationId).toBe("listInventoryItems");
});

test("pick checks the thinking-budget guard only on the first response of a run, not on later ones", async () => {
  const guardState = newPickGuardState();

  await pick(
    "q1",
    CANDIDATES,
    guardState,
    fakeTransport({ content: "listInventoryItems certain" }),
    "http://fake",
  );
  const second = await pick(
    "q2",
    CANDIDATES,
    guardState,
    fakeTransport({ content: "listInventoryLots certain", finishReason: "length" }),
    "http://fake",
  );

  expect(second.operationId).toBe("listInventoryLots");
});

test("pick's guard is per guardState, not global — a fresh state re-checks the first response", async () => {
  const result = pick(
    "q",
    CANDIDATES,
    newPickGuardState(),
    fakeTransport({ content: "listInventoryItems certain", finishReason: "length" }),
    "http://fake",
  );

  await expect(result).rejects.toThrow(/thinking leaked/u);
});
