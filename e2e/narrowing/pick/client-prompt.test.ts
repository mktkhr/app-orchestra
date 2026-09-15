import { readFileSync } from "node:fs";
import path from "node:path";

import { expect, test } from "vite-plus/test";

import {
  PICK_MODEL,
  PICK_SYSTEM_PROMPT,
  PICK_SYSTEM_PROMPT_WITH_EXAMPLES,
  newPickGuardState,
  pick,
  type PickCandidate,
} from "./client.ts";
import type { FetchLike } from "../embedding/client.ts";

const thisDir = import.meta.dirname;

/**
 * AC-S-103's TypeScript half (docs/specs/staging.md): the Go picker's own
 * `SystemPrompt` (`services/platform/internal/adapter/planner/pick/prompt.go`)
 * must be byte-identical to `PICK_SYSTEM_PROMPT` above - checked here by
 * reading the Go source directly, never by hand-copying its text into this
 * file. `prompt_test.go` is this test's own mirror, reading this file.
 */
test("the Go picker's SystemPrompt is byte-identical to PICK_SYSTEM_PROMPT", () => {
  const goSource = readFileSync(
    path.join(thisDir, "../../../services/platform/internal/adapter/planner/pick/prompt.go"),
    "utf8",
  );

  expect(goSource).toContain(PICK_SYSTEM_PROMPT);
});

/**
 * `PICK_SYSTEM_PROMPT_WITH_EXAMPLES`'s own tests (TODO.md item 1), split
 * out of `client.test.ts` only for eslint's `max-lines`: the two prompts
 * must never drift into one edited-in-place constant again - an earlier
 * version of this change did exactly that, and it silently moved every
 * pre-existing pick row's measured numbers (`client.ts`'s own comment on
 * `PICK_SYSTEM_PROMPT` records what that cost:
 * `pick:e5-large-q8+reranker` fell from 83%/51% to 77%/47% correct/flagged
 * on the one added sentence alone).
 */

const CANDIDATES: readonly PickCandidate[] = [
  {
    operationId: "listInventoryItems",
    serviceDisplayName: "在庫管理",
    summary: "在庫の一覧を返す",
  },
];

function bodyOf(init: RequestInit | undefined): unknown {
  const raw = typeof init?.body === "string" ? init.body : "{}";

  return JSON.parse(raw);
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

test("PICK_SYSTEM_PROMPT_WITH_EXAMPLES is PICK_SYSTEM_PROMPT plus exactly one added sentence, never an edit to PICK_SYSTEM_PROMPT itself", () => {
  expect(PICK_SYSTEM_PROMPT_WITH_EXAMPLES).toBe(
    `${PICK_SYSTEM_PROMPT}\n候補に e.g. 列がある場合、それはそのAPIに対して人がよく尋ねる質問の例。`,
  );
});

test("pick sends PICK_SYSTEM_PROMPT, unedited, when no systemPrompt is given", async () => {
  const { fetchImpl, body } = capturingFetch();

  await pick("在庫を見せて", CANDIDATES, newPickGuardState(), fetchImpl, "http://fake");

  expect(body()).toEqual({
    model: PICK_MODEL,
    temperature: 0,
    max_tokens: 200,
    chat_template_kwargs: { enable_thinking: false },
    messages: [
      { role: "system", content: PICK_SYSTEM_PROMPT },
      {
        role: "user",
        content: "質問: 在庫を見せて\n\n候補:\nlistInventoryItems\t在庫管理\t在庫の一覧を返す",
      },
    ],
  });
});

test("pick sends the given systemPrompt when one is passed explicitly - only the two examples-shown rows opt into PICK_SYSTEM_PROMPT_WITH_EXAMPLES", async () => {
  const { fetchImpl, body } = capturingFetch();

  await pick(
    "在庫を見せて",
    CANDIDATES,
    newPickGuardState(),
    fetchImpl,
    "http://fake",
    PICK_SYSTEM_PROMPT_WITH_EXAMPLES,
  );

  expect(body()).toEqual({
    model: PICK_MODEL,
    temperature: 0,
    max_tokens: 200,
    chat_template_kwargs: { enable_thinking: false },
    messages: [
      { role: "system", content: PICK_SYSTEM_PROMPT_WITH_EXAMPLES },
      {
        role: "user",
        content: "質問: 在庫を見せて\n\n候補:\nlistInventoryItems\t在庫管理\t在庫の一覧を返す",
      },
    ],
  });
});
